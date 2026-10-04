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

The private preparation directory `/private/tmp/corvint-501-preparation-20261004/` is not durable
repository evidence, so its verdicts are restated here:

- `INDEPENDENT-GATE-A.md` (four-file plan): PASS with no HIGH, two MED precision findings and one
  LOW implementation note; `REVISION.md` resolves the two MED findings.
- `SOURCE-REVIEW.md` (frozen four-file source): PASS with no HIGH, MED or LOW findings.
- `integration-prep/INDEPENDENT-GATE-A.md` (later full integration direction): PASS with no HIGH and
  one MED precision item to resolve before writer code; preparation only.

The delivered code and spec address them as follows:

- Material bindings: explicit expectedRevision, supersedes, requestId and queueId comparisons in
  `ValidateOperatorNoteMaterial`, with per-binding re-hashed refusal cases. This is pure supporting
  input for ON-V0-003; ON-V0-006 integrated preservation stays NOT_RUN.
- Historical reads: `ResolveOperatorNote` takes no latest ticket revisions. A note proposed after
  unrelated edits advance the ticket and acceptance revisions still validates, and a prior note
  newer than the audited ticket refuses (ON-V0-008).
- Layering: `ticket` does not import `mutation`; `go list -deps` confirms it (ON-V0-009).
- Whole-post preservation from the integration Gate A remains spec text (ON-V0-005, ON-V0-006),
  because no native material adapter exists yet.

A fresh independent review of this delivery returned FAIL with no HIGH findings, one MED and four
LOW findings and NITs. Corrections made before the change evidence was bound:

- MED: the historical-resolution test built an unused later record. It now proposes and validates a
  note after unrelated ticket and acceptance edits, and checks that a prior note postdating the
  audited ticket refuses for either revision. Reverting the postdate comparison makes it fail.
- LOW: an ARCHIVED ticket refused as UNAUTHORIZED. It now refuses as BLOCKED, matching the existing
  writer's archived rule; shadow and import ownership remain UNAUTHORIZED, and tests assert both.
- LOW: the closed-event and duplicate-key claims lacked direct tests. Unknown event keys, CLEAR with
  text, wrong-typed reference members and duplicate keys now refuse in `TestIssue501_NoteCodec`.
- LOW: the spec lacked non-goals and unresolved decisions. Both sections were added, including the
  revision-capacity outcome mapping (wire LIMIT_EXCEEDED in the pure helper; whether the writer
  reports CAPACITY_EXHAUSTED is a writer-slice decision).
- LOW: this entry relied on non-durable evidence and did not name the delegation source. The
  verdicts above, the hashes below and the delegation text are now quoted here.

## Delegation source

The delegation is recorded in the private session file `OWNER-DECISIONS-2026-10-04.md`, item 2:
"#501 operator-note intent: ADOPTED, with limits", based on NOTE501-001..010 in `PLAN.md` and
`REVISION.md`, with "The first delivery is the pure four-file foundation plus spec intent" and
writer, CLI, claim delivery and durable qualification later. It is an agent decision under owner
delegation. `docs/SPEC-DRIVEN-DEVELOPMENT.md` defines accepted intent as explicitly approved by the
repository owner, so direct owner confirmation of this acceptance remains open before promotion.

## Evidence

Frozen source SHA256 values from the source review, and the delivered state:

| File | Frozen SHA256 | Delivered |
|---|---|---|
| `internal/tasks/ticket/operator_note.go` | `0bec120b70612a7a6cf0c10c787c71a391f903866dd91b581a19691052c71617` | identical |
| `internal/tasks/ticket/operator_note_test.go` | `b526e0d43d4da0397a1a5471cc860c438fa4d7d07a234bcc4191b8b12f165707` | extended with the refusal cases above |
| `internal/tasks/mutation/operator_note.go` | `34371dcadaab63d12126151eb1a55597e98fbc267f1054ff295da0baf8488e91` | ARCHIVED refusal changed to BLOCKED |
| `internal/tasks/mutation/operator_note_test.go` | `32dca2572d6a5deb7dd27a124eebc2dc7c4d8773cde396b5267ebd85cb01f01e` | historical and outcome assertions extended |

On darwin/arm64 with Go 1.27.1, `go test -count=1` of `./internal/tasks/ticket`,
`./internal/tasks/mutation` and `./internal/specindex`, and `go vet` of the two task packages,
passed. Other platforms are NOT_RUN. The codec-level maximum escaped SET event measured 17003 bytes,
within the 65536-byte event bound. The 1670-byte MUTATE descriptor encoding and worst-case escaped
ticket record sizes are unmeasured.

Reused, read only, from the preparation directory: `PLAN.md`, `REVISION.md`, both
`INDEPENDENT-GATE-A.md` reviews, `SOURCE-REVIEW.md`, `source-frozen/` and the `integration-prep/`
intent seed. The preparation controller scripts were not run. Native ticket V1-0698 stays open: this
delivery does not complete the capability, and no Tasks completion write is made.
