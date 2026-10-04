# Issue 501 operator-note pure foundation (V1-0698)

## Decision

Issue 501 asks for one current, superseding operator note per Corvint Tasks ticket. The
NOTE501-001..010 preparation intent, with its review precision corrections, is adopted with limits by
owner-delegated decision 2026-10-04 (agent decided). This is an agent decision taken under the
owner's delegation, not a direct owner statement. Its limits: the first delivery is the pure
four-file foundation plus spec intent; the writer, CLI, claim delivery and durable qualification come
later, and issues 502 and 504 may then build on it.

The intent is catalogued as `docs/specs/corvint-tasks-operator-notes-v0.md`, with `ON-V0-001..010`
mapping one-to-one to NOTE501-001..010. The `ON-V0` prefix was absent at base
`4b10a02144faed4a77b36eba4b0d1cbc315706d3`. The prepared additive agent-lease amendment for
claim-time delivery is not included; it belongs to the deferred claim-delivery slice.

## Delivered

- `internal/tasks/ticket/operator_note.go`: closed `operatorNote` reference and
  `taskman-operator-note/0` event codecs, request decoding and historical resolution.
- `internal/tasks/mutation/operator_note.go`: pure NOTE_SET/NOTE_CLEAR proposal and material
  rebinding over a caller-supplied trusted context.
- Their focused tests `TestIssue501_NoteCodec`, `TestIssue501_NoteTransition`,
  `TestIssue501_NoteCASAndBounds` and `TestIssue501_NoteMaterialBindings`.

Nothing calls this code. The record codec, native writer, material validation, receipt audit and
redo, show/history reads, the `ticket note` CLI, claim delivery, the Core reader and native
qualification are deferred and remain NOT_RUN.

## Review findings carried

The prepared plan passed independent Gate A with two medium findings and one low finding; the frozen
source then passed independent source review with no findings. Both are retained in the private
preparation directory below. The delivered code and spec address them as follows:

- Material bindings: explicit expectedRevision, supersedes, requestId and queueId comparisons in
  `ValidateOperatorNoteMaterial`, with per-binding re-hashed refusal cases (ON-V0-003, ON-V0-006).
- Historical reads: `ResolveOperatorNote` takes no latest ticket revisions, and a later record's
  revisions do not change a resolved historical event (ON-V0-008).
- Layering: `ticket` does not import `mutation`; `go list -deps` confirms it (ON-V0-009).
- Whole-post preservation from the integration Gate A remains spec text (ON-V0-005, ON-V0-006),
  because no native material adapter exists yet.

## Evidence

The four files were copied byte-for-byte from the reviewed frozen source; their SHA256 values match
the source review. On darwin/arm64 with Go 1.27.1, `go test -count=1` of `./internal/tasks/ticket`
and `./internal/tasks/mutation` and `go vet` of both passed. Other platforms are NOT_RUN. The
65536-byte event bound, the 1670-byte MUTATE descriptor and worst-case escaped record sizes are
unmeasured.

Reused, read only, from `/private/tmp/corvint-501-preparation-20261004/`: `PLAN.md`, `REVISION.md`,
`INDEPENDENT-GATE-A.md`, `SOURCE-REVIEW.md`, `source-frozen/` and the `integration-prep/` intent
seed. The preparation controller scripts were not run. Native ticket V1-0698 stays open: this
delivery does not complete the capability, and no Tasks completion write is made.
