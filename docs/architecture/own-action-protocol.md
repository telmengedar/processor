# The harness's own action protocol

Standards applied: DiVoid #114 §0 and §4, bound to Go by #10861.

## Why this exists

The loop acted for the first time on 2026-09-06: given a file tool it called it unprompted and wrote
a real file. Against the ollama endpoint that replaced that host it stopped acting, and the reason is
the whole argument for this change.

The model emits a complete, well-formed action — valid JSON, correct name, correct arguments, clean
stop — inside `content`, while `tool_calls` comes back empty with `finish_reason: "stop"`.

The endpoint is not at fault, and the corrected diagnosis is the one that matters. The model's own
chat template names **two** tags: `<tools>` wraps the function *definitions* presented to it, and
`<tool_call>` is what it must emit to *call* one. The server parses `<tool_call>`, which is right.
The model emitted `<tools>` — it echoed the wrapper used to present the tools instead of the one
specified for calling them, one token apart in the same prompt.

So the structured field depends on the model correctly reproducing a syntax it was told about
in-band, and it demonstrably may not. When it fails, it fails **silently**: an action the model
genuinely declared arrives as an empty `tool_calls` and a `stop`, which the harness can only read as
a refusal to act. A correct action, discarded, with no trace.

That is not a fault a different model fixes, because the confusion is created by the protocol's own
shape — a definitions wrapper and a call wrapper that differ by one token. Nor is it one a different
server fixes: `tool_calls` is filled by whatever parser the server wraps around the model, so a
harness that depends on the field is portable only across servers whose templates agree. Both are
the opposite of what this project needs, since the vision is running on whatever hardware and models
a person actually has.

So the harness defines its own protocol, states it in the system text, and parses it out of the
response content. Any model that emits text can drive the loop.

Provider-native `tool_calls` is dropped outright rather than kept as a fallback. Keeping it would
mean two parsers for one concern, two shapes of "the model asked for something" to test, and a
retained dependency on a field whose failure mode is silence. No concrete case was found where the
fallback would recover a run the content parser loses: a server that fills `tool_calls` populates it
*instead of* leaving the text in content, which is a different protocol, not a degraded one.

This also closes a defect already on the books: the wire decoder read `ToolCalls[0]` only, while
endpoints emit several per response, dropping the rest without trace.

## The envelope

An action is the line `<processor-action>`, one JSON object, then the line `</processor-action>`.
The object has two members, `name` and `arguments`.

**One tag, used only to invoke.** There is no second wrapper anywhere in the prompt for the model to
confuse it with, and no separate syntax presenting the actions — they are described in the same
prose that describes everything else. The failure this replaces was a model picking the wrong one of
two adjacent tags; a protocol with one tag cannot have that failure by construction rather than by
instruction.

Adopting what the model volunteered — `<tools>` — was considered and rejected. What it volunteered
**was the mistake**, a good imitation of the nearest visible pattern, and building on it would encode
the error.

The spelling was probed against the endpoint rather than argued: the same one-action protocol was
instructed four times with four different tags. All four were reproduced correctly, which is the
useful negative result — with only one tag in the prompt the model does not need a distinctive one,
so *singularity* is doing the work, not novelty. Two observations decided the rest. `<tools>` was the
only spelling whose JSON came back malformed, with raw newlines inside a string where the other three
escaped them — one sample at temperature 0, so suggestive rather than conclusive, but pointing the
same way as the original failure. And `<tool_call>` survived untouched only because this harness
sends no `tools` array, which is what leaves the server's tool parsing switched off; its safety is
conditional on a server-side branch, where `<processor-action>` is safe under any request shape.

The *inner* shape is deliberately not ours: `{"name": ..., "arguments": {...}}` is the shape
function-calling training has already drilled into these models, and the probe confirms they emit it
cleanly. Our delimiter, the model's argument shape.

## Several actions in one response

All of them are parsed, and the loop **carries out all of them in one round**, in the order written,
then makes one model call carrying every result.

Taking the first and dropping the rest is the defect this change closes. Executing one per round
would keep every action but spend a model call on each, and `MaxModelCalls` is 3 — a response
declaring three writes could not complete. Rounds are the expensive resource here; actions are not.

If one action in a round fails, the ones after it still run. Stopping early would leave an action
neither carried out nor explained, which is the same silent drop in a smaller costume. Each action
gets its own record entry with its own outcome, and the model sees all of them together.

Every record entry carries the `round` that declared it. Without it, `toolCalls` read positionally
attributes the second action of round 1 to model call 2 — a fabricated history, and precisely the
misattribution the `tool` field was added to prevent one field earlier.

## Prose and an action in the same response

Both are legitimate and both are kept. The answer is the text outside the action blocks; the action
is carried out. The blocks are removed from the prose rather than left in it, because a `write_file`
block carries the file's entire content and would otherwise land in `Record.Answer` and in the
answer shown to a reader.

An accompanying sentence does not end the turn: when a response both speaks and acts, the actions
win and the loop continues.

## Content that cannot be read

**Nothing unread is ever dropped without trace.** The failure this replaces was silent — a correct
action discarded with no record — and replacing one silent failure with another would be worse than
not shipping.

Unread content becomes a record entry of its own under the tool name `unparsed`, carrying the reason
and the text. It travels the same path a failed action does: onto the record, into the trace, and
back to the model as a result it can correct. A response carrying only unread content therefore
costs a round rather than ending the turn, bounded by the existing call cap.

The recorded description states the reason, the **whole** byte count, and a bounded excerpt. The
excerpt is bounded because a truncated `write_file` can be tens of kilobytes and would otherwise be
copied into the run record and back into the next prompt; the size is always stated in full so the
excerpt can never be mistaken for the whole of it.

After a block that does not parse, parsing stops and the entire remaining text is carried into that
one problem. Once the stream is unreadable the parser cannot know where the block ended, so nothing
after it can be honestly attributed to a block or to prose — recording the tail whole is the only
choice that loses nothing.

Terminal reasons, in order: a response with actions ends as the first action's kind; otherwise a
response the endpoint truncated ends as `Truncated`, an unreadable block ends as the new `Malformed`,
an action naming something never offered ends as `Unrecognised`, and a response with neither actions
nor problems ends on the endpoint's own finish reason — a plain answer stays a first-class outcome.

`Truncated` outranks `Malformed` when the endpoint said it ran out of room, because the endpoint's
own account of why the text ends is better information than the parser's inference from the damage.

The reason for a multi-action response is taken from the **first** action rather than a new
"several actions" value. `Actions` and `toolCalls` carry the full truth; `stopReason` is a display
field two scripts already read, and the first action is the response's own leading intent, not an
arbitrary pick.

## Where the protocol lives

`internal/action` owns the delimiters, the action names, the parser, and the system text that states
them. The system text and the parser must agree exactly, so they are the same file: a protocol
described in one package and parsed in another drifts silently, and the drift is only visible as a
model that has stopped acting.

`internal/openaicompat` maps parsed calls onto the loop's actions and owns the argument shapes; the
loop owns the record. The parser knows nothing about either, so a second transport can reuse it
unchanged — which is the portability this whole change is for.

## Replaying prior rounds

Prior rounds go back to the model as plain `assistant` and `user` messages: the assistant turn
carries the action blocks exactly as the protocol states them, the user turn carries the results.

The `tool` role is not used. It is part of the provider tool protocol being dropped, it requires a
`tool_call_id` that strict servers validate, and replaying an action in its own protocol shows the
model a correct example of the format at every round.

Actions of one round are replayed as **one** assistant turn carrying every block. Splitting them
into one turn each would teach the opposite of what the protocol permits.
