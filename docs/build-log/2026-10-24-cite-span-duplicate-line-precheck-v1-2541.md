# Duplicated-line cite refusals name a recovery, and finish-lane prechecks the plan

Date: 2026-10-24. Lane: claude/cloud-v1-2541 (base 025120e1). Ticket: V1-2541 (P2). Related:
V1-1724 (dogfood prepare should list citable sources up front). Requirements `DCW-V0-334` and
`DCW-V0-335` (`docs/specs/daily-change-evidence-workflow-v0.md`) are accepted under the owner's
standing auto-accept (2026-10-11); `DCW-V0-335` amends `DCW-V0-115`.

## Problem

A dogfood finish refused the plan row `internal/tasks/journal/resume.go 317:317` with
`cite-span-not-stable`: the line's text appears twice in the changed file. Two finish attempts
were spent before the CEM existed, because the refusal surfaced only at the bind pass.

## Decision: stability by text stays

A span pinned by commit, blob and line number is unambiguous in the BASE. But the frozen `cem/1.0`
drift classifier (`verify.ClassifySpan`, restated in the `protocol/cem-1.0` packet) relocates every
cited span into the target by its text, and the verifier rejects an `ambiguous` item as
`evidence-drift`. Admitting the citation at `cem cite` would only move the refusal to `cem check`
and to the portable consumer. Changing the classifier would be a wire change for both consumers and
is out of this ticket's scope. So the requirement stays, and the refusal names the way out:

- each changed-file copy of the text (`at changed-file lines 5:5, 12:12`, at most 8 named);
- the narrowest wider BASE span containing the cited lines that is unique and unedited
  (`recovery: cite the wider unique span --lines 11:12`), searched at most 16 lines on each side.
  It is named only when it classifies stable or relocated, so it cites as stated.

## Decision: precheck in the prepare pass

finish-lane's prepare pass already holds the map the bind will re-prepare. It now cites every
non-unknown plan row against that map into a removed probe output, the same probe `DCW-V0-114`
uses, and stops at `prepare` with every refused row and its `cem cite` detail. Only plan-row codes
stop it; shape, uniformity and binding refusals stay the bind's. A separate plan-check verb, and
listing citable paths before a plan exists (the rest of V1-1724), were not built.

## Evidence

`TestCiteDuplicatedLineNamesCopiesAndWiderUniqueSpanV12541`,
`TestOccurrenceLinesBoundsNamedCopiesV12541` and
`TestFinishLaneDCWV0335PrepareRefusesUnstableCiteSpanBeforeBind` pass. The existing DCW-V0-068
finish-lane test's fake now admits probe cites, so its sidecar removal still happens at bind.
