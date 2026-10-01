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
README. Positional impact of the JavaScript source is an explicit unsupported non-Go profile,
retained in prechange-impact.json. The initial make dogfood-change at the unchanged base retained
NOT_PRODUCED git-diff-failed/map-unavailable because no delivered diff existed yet. Range affected
selection is retained with non-Go scope limits; focused adapter, qualification and documentation
checks plus the exact native campaign are the selected verification. Repository-wide make gate is
NOT_RUN under the owner's scoped-issue preference. Final evidence binding, campaign and review are
recorded before publication; qualification remains local evidence rather than execution authority.

Rollback restores adapter 0.7.6 source behavior and its matching package/matrix version under a
new package version, invalidating changed-tuple qualification. The owner's installed experimental
local copy and original configuration backup remain available independently of this source change.
