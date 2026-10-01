# OpenCode context without visible prompt JSON

Owner request: retain Corvint context while removing the repository-data envelope from visible
OpenCode requests, and merge the fix for the next build. Native ticket: V1-0590. Base:
`0b1244244066391da26f58b8e0cc8ebd737d9e8b`.

AHI-032 now delivers the framed task receipt through the model-request context hook. User input
stays unchanged, the bounded session cache is replaced for each distinct prompt before admission
can fail busy, and compaction clears it. Prompt-generation checks reject older responses. The
package and compatibility adapter version advance from 0.7.6 to 0.7.7 under AHI-020.

The original live OpenCode 2.0.21/adapter 0.7.6 loopback probe reproduced visible JSON. A local
experimental override demonstrated unchanged native prompt text, no automatic envelope in provider
user messages, and receipt-linked system context. The repository regressions failed against the
original adapter and passed after the fix. The qualification collector now compares the hidden
frame at the context-hook and provider boundaries, retains unchanged-prompt evidence, and measures
hidden frame bytes. Timing probes inspect the registered context callback for admitted idle fixture
prompts; a separate real-host request proves transport delivery. Scripted provider usage is not
billed token, task-solving quality or cost evidence.

Corvint context was used before edits; its packet retained the governing spec, adapter tests and
README. Positional JavaScript impact returned READY with one path result and four ranked results omitted,
retained in prechange-impact.json; that does not establish complete dependency coverage. The initial make dogfood-change at the unchanged base retained
NOT_PRODUCED git-diff-failed/map-unavailable because no delivered diff existed yet. Range affected
selection is retained with non-Go scope limits; focused adapter, qualification and documentation
checks plus the exact native campaign are the selected verification. Repository-wide make gate is
NOT_RUN under the owner's scoped-issue preference. Independent review caught an unbound unchanged-prompt observation: a later idle sample could mask
mutation of the first provider-bound draft. Observer and probe session/message identities now join
that exact draft, with a negative regression for mixed evidence. Independent review passed the prompt-evidence repair. Exact native qualification on adapter 0.7.7,
OpenCode 2.0.21, Core build 163 and darwin/arm64 passed all 26 checks and 14 conformance groups.
The hidden frame was 2,458 bytes; fixture critical recall was 1/1; query p95 was 291.395 ms and
lifecycle p95 was 143.003 ms. Interruption observed passive absence before supervisor cleanup.
The earlier timing campaign lost a descendant snapshot outside the sandbox; its failed receipt
remains retained and the suspected tooling friction is V1-0599. No check was bypassed. AHI-020
version assertions have explicit requirement names for independent OCM linkage. Final binding and
review are recorded before publication; qualification remains local evidence rather than execution authority.

Rollback restores adapter 0.7.6 source behavior and its matching package/matrix version under a
new package version, invalidating changed-tuple qualification. The owner's installed experimental
local copy and original configuration backup remain available independently of this source change.
