# Structured read decisions

Implementation contract for task24460, based on DiVoid design24484's locked contract after gate24485 (2026-10-23).

A config may opt into `responseMode:"structuredRead"`; omission retains native behavior. Unknown values, wrong types, null, and combination with any non-null note config are rejected. Existing `/runs` responses contain only decoded answer text, never the transport envelope.

Each existing judgement call returns exactly one JSON object, with exactly four required, case-sensitive keys:

| action | text | query | id |
| --- | --- | --- | --- |
| `answer` | nonblank actual reply | null | null |
| `recall` | empty string | nonblank phrase in the existing `IsPhrase`/`MaxPhraseBytes` domain | null |
| `read` | empty string | null | positive int64 |

Duplicate or unknown keys, malformed JSON, trailing values, invalid UTF-8, wrong types and branch-inconsistent fields are rejected. Existing silence and nonreply classification applies to decoded answer text. An answer promising to read remains an answer; only an explicit retrieval action requests dispatch.

The strict decoder runs before dispatch. Invalid decisions or native calls accompanying JSON use existing model-failure handling (502, or existing reserved-call failure accounting). There is no prose recovery, validation retry, or extra model phase. Structured mode bypasses native, textual, reference and echo tool recovery.

Read and recall reuse existing dispatch, filters, working-memory absorption, receipt recording, yield accounting, and call/time limits. Subsequent judgement sees retrieved facts. Allowed actions reflect each call's capabilities: answer is always available, recall is withdrawn when its existing yield account closes, and reserved final calls permit only answer. Native tools are absent on the wire. `offered` records empty native arrays per call; optional `structuredAllowedActions` records the JSON action vocabulary per call. Retrieval rounds use source `structured`. Native records omit the additional field.

The typed system builder accepts response mode. `systemtext.Compose` remains available; `ComposeForMode` preserves native output and supplies structured mechanics only in this mode. Caller identity passes verbatim; translating caller-authored interface instructions belongs to the caller. The loop supplies the structured recall clause. Adapters add no hidden system instructions: Ollama uses `format`; OpenAI-compatible endpoints use strict `response_format.json_schema`. Default requests omit those fields.

The OpenAI-compatible request shape follows the [Chat Completions API reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create). HTTP fixture tests verify this adapter's wire shape; they do not establish live provider support.

Incremental structured framework and schema overhead is charged within the existing 80,000-byte working-memory ceiling. Identity, conversation and existing charges remain. No budget increase or new query cap is introduced.

The finite gemma4:latest gate24485/raw24486 passed missing-pointer selection, supplied-fact answering without lookup, and simulated read-result completion. Those results do not establish actual backend dispatch or broader model quality. Fake-port tests cover read→absorb→decoded answer and supplied-answer controls; private branch integration remains a separate acceptance gate. Note decisions/quality, file actions and supplied-memory interpretation are outside this change. Gangolf's broader active-memory task17771 remains open.
