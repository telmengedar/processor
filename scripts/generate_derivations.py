#!/usr/bin/env python3
"""Fill coverage gaps in internal/eval/derivations.json by generating derived
queries for corpus rows the sidecar does not yet pin.

Background
----------
internal/eval/derivations.json pins a set of alternate search queries per
corpus row. LoadDerivations (internal/eval/derivations.go) validates that
every *sidecar* row resolves to a corpus row, but not the reverse, so a
corpus row the sidecar never mentions sweeps on its raw input alone, on
every arm, and is identical between arms by construction. See DiVoid #11327.

This script closes that gap for whichever rows are currently unpinned,
without touching rows that are already pinned (the pinned rows are the
comparability baseline for prior measurements and must not change).

Blindness, enforced structurally
---------------------------------
The model must never see a corpus row's `required` node ids, their content,
or the `why` commentary explaining what the row is "really" asking -- doing
so contaminates the derived arm's evaluation of itself (DiVoid #11235 S7 R2a,
#11292). This script enforces that by construction rather than by care:

  * `project_corpus_blind` reads corpus.json and immediately discards every
    field except `id` and `input`, returning a list of `BlindRow`.
  * Every function on the model-facing call path (`call_model`,
    `generate_for_row`) takes a `BlindRow` (or its two string fields) as
    its only row-shaped input. Nothing else in corpus.json ever reaches
    them, because nothing else survives the projection.
  * `main` is the only function that reads the full corpus.json structure,
    and it does so only to build the id list and the blind projection --
    it never passes the full row dict onward.

The two few-shot examples embedded in the prompt are themselves rows that
are NOT being regenerated (r01, r06) and are shown as (input, queries)
pairs only -- their `required`/`subject`/`why` fields are not included --
so they demonstrate the desired query *shape* without leaking any answer
key for the rows this script actually generates. They are harmless only as
long as r01/r06 stay untouched: were either ever regenerated, its own
pinned queries would be sitting in its prompt history verbatim. `--only`
refuses to overwrite an already-pinned row without `--force` precisely to
keep that from happening silently.

Usage
-----
    python scripts/generate_derivations.py \\
        --corpus internal/eval/corpus.json \\
        --derivations internal/eval/derivations.json

    # Dry run: print what would be generated without writing the sidecar.
    python scripts/generate_derivations.py --dry-run

    # Regenerate specific unpinned rows (fails if any of them is already pinned).
    python scripts/generate_derivations.py --model ai/qwen3-coder --only r12,r13

    # Overwrite an already-pinned row deliberately -- this replaces the measured
    # baseline that row was pinned against and invalidates any prior sweep taken
    # with it, so it requires the explicit --force.
    python scripts/generate_derivations.py --only r02 --force

Requires only the Python standard library. Talks to an OpenAI-compatible
chat/completions endpoint (Docker Model Runner on this network, by default
http://gangolf:12434/engines/v1/chat/completions).
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

DEFAULT_ENDPOINT = "http://gangolf:12434/engines/v1/chat/completions"
DEFAULT_MODEL = "ai/qwen3-coder"
QUERIES_PER_ROW = 5
MAX_FILL_ATTEMPTS = 4
REQUEST_TIMEOUT_SECONDS = 180

# Few-shot exemplars: format only. Drawn from rows this script never
# regenerates (r01, r06 are already pinned and out of scope), and stripped
# down to (input, queries) -- no subject/required/why -- so they teach the
# query *shape* without exposing any row's answer key.
FEW_SHOT = [
    {
        "input": "Where is a Go test supposed to state what it is checking, and why not in a comment above it?",
        "queries": [
            "How does a Go test carry what it pins?",
            "Why are comments not the place to state a test's intent?",
            "What does the comment contract say about test files?",
            "What names a test's intent so it survives a refactor?",
            "Go test name intent comment contract",
        ],
    },
    {
        "input": "What happens to a request that is still being worked on when the service is told to stop?",
        "queries": [
            "How does the server drain requests in flight during shutdown?",
            "Is a run cancelled or allowed to finish when the process is told to stop?",
            "What bounds the graceful shutdown period?",
            "What happens to write-back while the process is shutting down?",
            "graceful shutdown in-flight request drain timeout",
        ],
    },
]

SYSTEM_PROMPT = f"""You generate alternate search queries for a semantic retrieval system.

You will be given ONE user request. Your job is to produce {QUERIES_PER_ROW} additional \
queries that a semantic (embedding-based) search engine could use to surface the specific \
documentation that answers that request. The request itself is already one query the search \
runs; your job is to add angles that request does not cover.

Think about what actually helps a semantic search here: the request is often phrased the way \
a confused or informal user would phrase it, while the documentation that answers it is \
phrased the way an author states a ruling, a mechanism, or a design decision. A good derived \
query bridges that gap -- it surfaces the underlying mechanism, the specific technical terms, \
the named concept, or the class of problem the request is really an instance of, using \
vocabulary closer to how documentation states things.

Rules:
- Do not restate or lightly reword the input request. Each query must approach the underlying \
information need from a genuinely different angle than the input and from each other.
- Do not answer the request. You are generating queries, not answers.
- Do not invent specifics (names, numbers, node ids) that are not implied by the request itself.
- Output exactly {QUERIES_PER_ROW} lines:
  - The first {QUERIES_PER_ROW - 1} lines are distinct, standalone questions (each ending in \
"?") that name a mechanism, concept, or specific terminology likely to appear in the answer.
  - The last line is a dense, keyword-style query (no question mark) combining the most \
salient technical terms an embedding search would key on.
- Output ONLY those {QUERIES_PER_ROW} lines. No numbering, no bullets, no quotes, no preamble, \
no commentary, no blank lines between them.

Examples of the desired shape (unrelated to the request you will be given):
"""


@dataclass(frozen=True)
class BlindRow:
    """A corpus row projected to exactly what the model is allowed to see."""

    id: str
    input: str


def project_corpus_blind(corpus_path: Path) -> list[BlindRow]:
    """Read corpus.json and discard every field except id/input before anything else touches it."""
    data = json.loads(corpus_path.read_text(encoding="utf-8"))
    rows = []
    for entry in data:
        # Only `id` and `input` are read here. `entry` (with `subject`,
        # `required`, `stratum`, etc.) is discarded at the end of this loop
        # iteration and never returned or passed to any caller.
        rows.append(BlindRow(id=entry["id"], input=entry["input"]))
    return rows


def load_sidecar(derivations_path: Path) -> list[dict]:
    if not derivations_path.exists():
        return []
    return json.loads(derivations_path.read_text(encoding="utf-8"))


def pinned_ids(entries: list[dict]) -> set[str]:
    return {entry["row"] for entry in entries}


def build_few_shot_messages() -> list[dict]:
    messages = []
    for example in FEW_SHOT:
        messages.append({"role": "user", "content": example["input"]})
        messages.append({"role": "assistant", "content": "\n".join(example["queries"])})
    return messages


def call_model(
    endpoint: str,
    model: str,
    row_input: str,
    exclude: list[str],
    endpoint_timeout: int,
    temperature: float,
) -> str:
    """Call the chat completion endpoint for one row. Takes only the row's raw input text."""
    user_content = row_input
    if exclude:
        excluded_block = "\n".join(f"- {q}" for q in exclude)
        user_content = (
            f"{row_input}\n\n"
            f"(Already have these, from an earlier attempt -- do not repeat them or anything "
            f"close in meaning; produce different angles instead:\n{excluded_block})"
        )

    messages = [{"role": "system", "content": SYSTEM_PROMPT}]
    messages.extend(build_few_shot_messages())
    messages.append({"role": "user", "content": user_content})

    payload = json.dumps(
        {
            "model": model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": 500,
        }
    ).encode("utf-8")

    request = urllib.request.Request(
        endpoint,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=endpoint_timeout) as response:
        body = json.loads(response.read().decode("utf-8"))
    return body["choices"][0]["message"]["content"]


LINE_PREFIX_RE = re.compile(r"^\s*(?:[-*•]|\d+[.)])\s*")
THINK_BLOCK_RE = re.compile(r"<think>.*?</think>", re.DOTALL | re.IGNORECASE)


def parse_lines(raw_text: str) -> list[str]:
    """Strip reasoning artifacts and list decoration, returning one query per line."""
    text = THINK_BLOCK_RE.sub("", raw_text)
    lines = []
    for line in text.splitlines():
        line = LINE_PREFIX_RE.sub("", line.strip())
        line = line.strip('"“”\'')
        if line:
            lines.append(line)
    return lines


def dedupe_against(raw_input: str, candidates: list[str], already: list[str]) -> list[str]:
    """Drop blanks, exact duplicates of the row's own input, and repeats already collected."""
    seen = {q.strip().casefold() for q in already}
    seen.add(raw_input.strip().casefold())
    kept = []
    for candidate in candidates:
        key = candidate.strip().casefold()
        if not key or key in seen:
            continue
        seen.add(key)
        kept.append(candidate)
    return kept


QUESTION_STARTERS = {
    "how", "why", "what", "is", "are", "was", "were", "does", "do", "did",
    "can", "could", "should", "would", "will", "where", "who", "whom",
    "which", "when", "must", "shall", "may", "might",
}


def looks_like_a_question(line: str) -> bool:
    """Ends in '?', or opens with a common interrogative/auxiliary word.

    A trailing '?' alone under-detects: a line does not stop being a prose question just
    because the model forgot the mark. A line reading "How does the ranking system prevent
    item cycling in top positions" is unmistakably phrased as a question and not a dense
    keyword line, even though it satisfies "does not end in '?'" -- exactly the gap that let
    r15's fifth line through unnoticed before this check existed.
    """
    stripped = line.strip()
    if stripped.endswith("?"):
        return True
    first_word = stripped.split(" ", 1)[0].strip(",.:;\"'").lower()
    return first_word in QUESTION_STARTERS


def shape_errors(queries: list[str]) -> list[str]:
    """Report ways `queries` misses the 4-question + 1-keyword-line shape the prompt asks for.

    Judges only whether the model followed the requested shape, not whether a query is a *good*
    derived question -- that judgment is off-limits here (tuning against a measurement is the
    contamination this unit exists to avoid). Kept separate from dedupe_against because it
    judges the assembled set's shape, not any one candidate in isolation.
    """
    if len(queries) != QUERIES_PER_ROW:
        return [f"expected {QUERIES_PER_ROW} queries, got {len(queries)}"]

    errors = []
    for i, query in enumerate(queries[:-1], start=1):
        if not looks_like_a_question(query):
            errors.append(f"line {i} should be phrased as a question: {query!r}")
    last = queries[-1]
    if looks_like_a_question(last):
        errors.append(f"line {QUERIES_PER_ROW} should be a dense keyword line, not phrased as a question: {last!r}")
    return errors


def generate_for_row(
    endpoint: str,
    model: str,
    row: BlindRow,
    endpoint_timeout: int,
    temperature: float,
    log,
) -> list[str]:
    """Generate QUERIES_PER_ROW distinct, valid queries for one row. Sees only row.input."""
    collected: list[str] = []
    for attempt in range(1, MAX_FILL_ATTEMPTS + 1):
        raw_text = call_model(endpoint, model, row.input, collected, endpoint_timeout, temperature)
        candidates = parse_lines(raw_text)
        fresh = dedupe_against(row.input, candidates, collected)
        collected.extend(fresh)
        log(f"    attempt {attempt}: model returned {len(candidates)} line(s), "
            f"{len(fresh)} new/valid, {len(collected)}/{QUERIES_PER_ROW} collected")
        if len(collected) >= QUERIES_PER_ROW:
            final = collected[:QUERIES_PER_ROW]
            for problem in shape_errors(final):
                log(f"    WARNING: row {row.id} shape: {problem}")
            return final
    if not collected:
        raise RuntimeError(
            f"row {row.id}: the model never produced a usable query after "
            f"{MAX_FILL_ATTEMPTS} attempts"
        )
    log(f"    WARNING: row {row.id} only reached {len(collected)}/{QUERIES_PER_ROW} "
        f"distinct queries after {MAX_FILL_ATTEMPTS} attempts; pinning what was collected")
    for problem in shape_errors(collected):
        log(f"    WARNING: row {row.id} shape: {problem}")
    return collected


def corpus_order_key(corpus_ids: list[str]):
    index = {row_id: i for i, row_id in enumerate(corpus_ids)}
    return lambda entry: index.get(entry["row"], len(index))


def select_targets(
    blind_rows: list[BlindRow],
    corpus_ids: list[str],
    already_pinned: set[str],
    only: str | None,
    force: bool,
) -> tuple[list[BlindRow], str | None]:
    """Resolve which rows to generate. Returns (targets, error); error is None on success.

    Without --only, targets are every unpinned row -- this path can never touch a pinned row.
    With --only, a name that is already pinned is refused unless force is set, since silently
    honoring it would overwrite the measured baseline that row was pinned against (F2).
    """
    if not only:
        return [row for row in blind_rows if row.id not in already_pinned], None

    target_ids = {rid.strip() for rid in only.split(",") if rid.strip()}
    unknown = target_ids - set(corpus_ids)
    if unknown:
        return [], f"--only names rows not in the corpus: {sorted(unknown)}"

    already = sorted(target_ids & already_pinned)
    if already and not force:
        return [], (
            f"--only names rows already pinned: {already} (pass --force to overwrite; this "
            f"replaces the measured baseline those rows were pinned against, invalidating any "
            f"prior sweep taken with them)"
        )

    return [row for row in blind_rows if row.id in target_ids], None


SOURCE_BLIND_GENERATED = "blind-generated"


def merge_sidecar(
    sidecar_entries: list[dict],
    generated: dict[str, list[str]],
    corpus_ids: list[str],
) -> list[dict]:
    """Replace/add `generated` rows into `sidecar_entries`, returned in corpus order.

    Every row this script produces is stamped `source: blind-generated` at the point of
    generation, since that is the only place the fact is known -- see internal/eval/derivations.go's
    SourceBlindGenerated for the reader.
    """
    kept = [entry for entry in sidecar_entries if entry["row"] not in generated]
    new = [
        {"row": row_id, "queries": queries, "source": SOURCE_BLIND_GENERATED}
        for row_id, queries in generated.items()
    ]
    merged = kept + new
    merged.sort(key=corpus_order_key(corpus_ids))
    return merged


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--corpus", default="internal/eval/corpus.json", type=Path)
    parser.add_argument("--derivations", default="internal/eval/derivations.json", type=Path)
    parser.add_argument("--endpoint", default=DEFAULT_ENDPOINT)
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--temperature", type=float, default=0.5)
    parser.add_argument("--timeout", type=int, default=REQUEST_TIMEOUT_SECONDS)
    parser.add_argument("--only", help="comma-separated row ids to generate, default: all unpinned")
    parser.add_argument("--force", action="store_true",
                         help="allow --only to overwrite rows that are already pinned")
    parser.add_argument("--dry-run", action="store_true", help="print generated queries, do not write the sidecar")
    args = parser.parse_args()

    def log(message: str) -> None:
        print(message, file=sys.stderr)

    corpus_data = json.loads(args.corpus.read_text(encoding="utf-8"))
    corpus_ids = [entry["id"] for entry in corpus_data]
    blind_rows = project_corpus_blind(args.corpus)

    sidecar_entries = load_sidecar(args.derivations)
    already_pinned = pinned_ids(sidecar_entries)

    targets, error = select_targets(blind_rows, corpus_ids, already_pinned, args.only, args.force)
    if error:
        log(f"error: {error}")
        return 2

    if not targets:
        log("nothing to generate: every requested row is already pinned")
        return 0

    log(f"model={args.model} endpoint={args.endpoint} rows-to-generate={[r.id for r in targets]}")

    generated: dict[str, list[str]] = {}
    for row in targets:
        log(f"  generating {row.id} ...")
        started = time.monotonic()
        try:
            queries = generate_for_row(args.endpoint, args.model, row, args.timeout, args.temperature, log)
        except (urllib.error.URLError, TimeoutError, RuntimeError, KeyError, json.JSONDecodeError) as exc:
            log(f"error: row {row.id}: {exc}")
            return 1
        elapsed = time.monotonic() - started
        generated[row.id] = queries
        log(f"    {row.id} done in {elapsed:.1f}s:")
        for q in queries:
            log(f"      - {q}")

    if args.dry_run:
        log("--dry-run: sidecar not written")
        print(json.dumps({rid: qs for rid, qs in generated.items()}, indent=2))
        return 0

    merged = merge_sidecar(sidecar_entries, generated, corpus_ids)

    # newline="\n" pins LF regardless of platform: LoadDerivations hashes the
    # working-tree bytes, not the git blob, so a platform-dependent newline here
    # makes the recorded hash unreproducible from a fresh checkout (F1, DiVoid PR review).
    args.derivations.write_text(json.dumps(merged, indent=2) + "\n", encoding="utf-8", newline="\n")
    log(f"wrote {args.derivations} ({len(merged)} rows pinned)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
