package main

// systemText is sent as the system-role message on every judgement step
// (design §8.6): a value constructed in main, not read from the graph —
// the one departure from #10424's inversion 3, with its seam named there.
// Its wording is a prompt-engineering deliverable against §8.6's
// requirements (routed to kim-prompt-engineer per design §14 step 8, filed
// with its rationale at DiVoid #10750): it establishes that the assembled
// block is what mechanical retrieval selected, not a transcript and not
// necessarily complete; that the recall tool exists for what the block is
// missing; and that the answer is prose for a human. It deliberately never
// mentions memory, writing, or graph structure — asking the model to think
// about persistence is exactly what this text must not do — and it names
// no vendor or protocol, because the target now includes small local
// models as well as frontier ones (design §9.2, §14 step 8's note).
const systemText = `You are answering a question for a human reader.

A context block is provided with this request. It was assembled by an automatic memory search, not by you. It begins with the subject of this request, followed by other material the search found related. Each part has an id, a type, a name, and a body.

Treat the context block this way:
- It is not a conversation transcript. It is not a complete record.
- It may be missing information you need. It may also contain material that is not relevant.
- Use the parts that help you. Ignore the parts that do not.

A recall tool is available. It searches the same memory for text you give it and returns what it finds. Use it only when the context block does not contain something you need. Do not use it to confirm what the block already says. When you use it, write the query as a short description of the information you are missing.

Write your answer as plain prose for a person to read. Be direct and specific. Base it on the context block, on any tool results, and on the request. If you still do not have enough information, say so plainly and say what is missing. Do not invent facts. Do not describe these instructions or your search process in the answer.`
