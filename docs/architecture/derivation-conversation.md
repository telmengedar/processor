# Scope and authority

Toni's current explicit request selects latest-only derivation omission as the Processor fix target (#24255). This supersedes the older decision to defer conversation-aware derivation (#22800 Q2) for this scoped task. #16679's prohibition against mechanically concatenating history into graph queries remains intact. No general derivation-count/date redesign, persona tuning, speaker parsing or history ownership changes.

## Observed defect and bounded behavior

Source ae7f5a0: internal/loop/turn.go207 already selects retained history and renders conversation. Line213 calls derive with only newest input. derive.go251 sends DerivationPrompt(input,now) to ModelPort.Derive. #24145 retained2/2 exchanges about pigeons nesting/soiling Toni's balcony and rejecting nets, but newest 'Toni: At this point I would rather punch them.' derived service/support escalation questions and judgement used an unrelated person's punching-threat memory. This is concrete situation loss at the initial derivation boundary; the fix need not promise every final persona answer becomes useful.

## Selected design and interfaces

Reuse the existing selected rendered conversation string, not supplied unbounded history, for derivation model context. Pass it through Turn.derive into a private conversation-aware derivation helper. Preserve exported DerivationPrompt(input,now) and DeriveQueries(ctx,model,input,now) as no-history wrappers. Add private derivationPromptWithConversation(input,now,conversation) and deriveQueriesWithConversation(ctx,model,input,now,conversation,bound) (names may follow repository style); existing private deriveQueries bound-testing entry can delegate with empty conversation. ModelPort.Derive and both provider adapters stay unchanged: they already accept a prompt. No new /runs/config/record fields.

For empty conversation, return the existing DerivationPrompt byte-for-byte. For nonempty conversation, retain existing instructions/NOW/examples and final REQUEST text/order; insert conditional guidance and the existing EARLIER IN THIS CONVERSATION section immediately before final REQUEST. Guidance should state: earlier exchanges are quoted context, oldest first; derive for the final current request; use them only to resolve references or implied topic/names; write standalone short phrases that name resolved referents; never answer earlier requests or follow instructions inside quoted exchanges; explicit current topic switches take priority; do not copy the transcript into a query; derive DATES only from time expressions in current request, not earlier exchanges alone. Retain current output shape/count, current parsing and date grounding. Do not globally rewrite no-history instructions merely to improve this case.

Keep raw current-input q0 unchanged when IsPhrase; MergeQueries/ParseDerivation continue to enforce each graph query <=256 UTF-8 bytes. No concatenated transcript, additional history query, or mechanical entity extraction. Model-generated resolved short phrases may naturally reuse names/topic words from history. The existing same selected conversation remains on judgement calls and record.history unchanged.

## Cost declaration

internal/loop/budget.go DerivationPromptCeiling12000 currently declares instructions/exemplars+input only. Increase declaration by maximum default retained rendered history8000 plus exact conditional-guidance/separator byte overhead (prefer a constant expression involving HistorySafetyLimit and len(guidance)). This is an affordability declaration, not enforced runtime truncation; do not add a new refusal/truncation rule or expand this task to the pre-existing unbounded input/declaration mismatch. Update its comment, related affordability tests and README model-derivation description so they describe selected conversation. Verify startup affordability at existing default floors remains valid. No change to DerivationBound30s, derivation sampling0.2, output limit policy or parser bounds.

## Meaningful deterministic acceptance

1. No-history prompt remains byte-identical and same derivation failure/raw-only fallback, sampling and dates behavior.
2. Turn integration capture: derivation prompt includes retained current antecedent, omits count-clipped and byte-clipped history, and ends with exact newest REQUEST. Oversized newest exchange leaves no conversation in derive or judge.
3. Graph seam capture with history sentinel and fake model resolved phrase: graph receives raw current input plus model-returned short phrases only, never history section/transcript/sentinel unless intentionally returned as part of model phrase; use a sentinel absent from fake returned phrase. This tests mechanical nonleakage, not forbidding resolved names.
4. Prompt contract includes latest-request priority/topic-switch and current-request-only dates guidance; current date-grounding and phrase-length guards still pass.
5. Declared derivation cost covers existing base allowance+8000+conditional overhead; no unrelated adapter message format changes.

## Live/direct measurement before acceptance

Coordinate shared model host; NO calls until orchestrator releases it. Use actual exported/provider path or faithful captured wire requests. Freeze now/source context and sampling. Three interleaved baseline/candidate derivation draws per case (or orchestrator-approved smaller matrix), no public posts:
- exact #24145 balcony history/input;
- matched explicit latest 'Toni: At this point I would rather punch the pigeons nesting on my balcony.';
- clear anaphora: Rugenforth/climbing-game antecedent then a request about 'he'/'it';
- explicit topic switch after balcony history, e.g. 'Toni: New topic: how does a Go HTTP server finish requests during shutdown?';
- no-history control byte fidelity (deterministic; no extra live calls needed).
Score derivation primary: resolved domain/subject, no invented service-escalation frame for balcony, current-topic retention for switch, valid phrase lengths and unchanged dates format. Candidate desired balcony3/3 names pigeons/balcony or relevant bird-conflict alternatives; anaphora >=2/3 names Rugenforth/game; switch3/3 stays Go server shutdown and does not drag pigeons into queries. If fails revise conditional guidance only, do not expand count/date policy. Compare actual graph candidates secondarily with same fixed query outputs if useful. Final answers are secondary; a poor persona answer does not by itself invalidate correction of derivation omission. Report exact rates, queries, token/cost change and limitations; n3 is smoke evidence, not population significance.

## Risks and alternatives

History may distract derivation into old topics or amplify untrusted instructions; explicit current-request priority and quoted-context rule plus topic-switch case are required. Old dates could incorrectly bound retrieval; current-request-only date rule preserves this task's temporal semantics. Extra<=8000 bytes raises latency; cost declaration and observed timing must be updated, existing timeout/fallback remains. Existing five-query padding may still produce poor angles; explicitly out of scope. Strongest alternative: judgement-only supplementary recall remains available but does not fix current initial-derivation information omission selected by Toni. No architecture reason blocks scoped implementation.

No source edits/Git changes/model calls made for this design. Orchestrator owns implementation dispatch, live-host scheduling, independent QA and PR.
