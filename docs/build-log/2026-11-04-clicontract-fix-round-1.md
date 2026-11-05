# clicontract fix round 1: preserve native CLI contracts

The owner's final compatibility decision governs this repair: shared CLI presentation must
not change any existing wire contract. ACX-V0-7707 is narrowed accordingly. The rejected
universal envelope, stdout-to-stderr move, exit-code normalization and pre-dispatch refusal
guards are withdrawn. Existing JSON refusals already provide their command-owned envelopes;
Core's frozen codes/error shape and Tasks' stdout result/exit code remain authoritative.
Plain diagnostics pass through immediately, without truncation, extra buffering or duplicated
stderr fields. The console's native plain-text failure writer is restored too.

QAT-V0-7708 and its two-rendering exception are withdrawn as out of scope. QAT-V0-001/004,
their exact original byte predicate, replay checks and existing hostile tests are restored.
The Tasks result decoder and the three compiled fixture readers are also restored to their
base contracts. No goldens or existing dogfood/Core freeze tests are changed.

V1-3736 is PARTIAL: the remaining native refusal compatibility, opt-in lint and offline help
work does not satisfy its universal JSON stderr/exit-scheme acceptance. The other six tickets
retain their prior engineering dispositions: V1-2685 (CAP-V0-7700), V1-3781 (CAP-V0-7701),
V1-3738 (CAP-V0-7702/TCL-V0-7703), V1-3976 (TCL-V0-7704), V1-3938 (LPCV-V0-7705),
and V1-3277 (CAP-V0-7706). No task-store completion write is authorized.

Evidence lives in `/home/russell/lanes/clicontract.evidence/fix-round-1/`; the final external
handback records exact commands, results, immutable head and remaining limits. Initial
`make dogfood-change` ran before edits (exit 2: missing citation/intent/outcome inputs).
Its retained pre-change query and impact receipts are bound to the same original base.
`corvint affected --base d436dff76de1b8e75c427069ae7f80003ae99bc2` ran before Go testing:
281 selected units, 91 unknowns, scope UNKNOWN. Focused package checks include the original
dogfood tests, Core frozen outputs, GPK-V0-002 replay, provider usage smoke and native Tasks
consumers. Every Go test runs through the required heavy wrapper, one package per command.
The spec requirements index is regenerated after staging the owning specs.

Routes used: pre-change context/change evidence and affected planning. Frozen CLI replay is
applicable; retrieval/ranking/learning evaluations, live provider execution, remote services,
mutation experiments and external installation are not applicable to this compatibility repair.
Independent review belongs to the orchestrator. Coverage/discrimination, cross-platform and
external outcome evidence remain unproduced or unobserved; passing structural checks cannot
supply them. No exhaustive gate, publication, seal or Tasks write is authorized. Rollback is
a revert of this repair; it would reintroduce the reviewed compatibility regressions and must
not be used to claim the withdrawn acceptance behavior delivered.
