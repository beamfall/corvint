# 2026-10-05: V1-0808 corvint-tasks attach-evidence

## Intent

Ticket V1-0808 (agent-filed, implemented on orchestrator instruction 2026-10-05): attach sha256
evidence digests and a short reason to an OPEN ticket without changing its record, acceptance
revision, status or gate state. The governing contract is the new proposed profile
`docs/specs/corvint-tasks-evidence-attachments-v0.md`, requirement `TEA-V0-001`. Owner acceptance
of its drafted defaults is pending.

## Change

- **Operation:** `ATTACH_EVIDENCE` with closed payload `{evidence, reason}`; CLI verb
  `ticket attach-evidence` on the generic mutation surface, with a help note.
- **Record:** optional member `attachedEvidence` (append-ordered entries
  `{acceptanceRevision, actor, evidence, reason, recordedAt}`), omitted when empty, so legacy
  records keep their bytes. Tasks and Core readers validate it and stay closed otherwise; the
  shared optional-key fixture now carries it.
- **Apply:** one step case on the existing Apply/finalize path. Finalize bumps revision only; no
  acceptance-relevant field changes, so acceptanceRevision and the attempt-liveness check are
  untouched. No derived event, receipt kind, lock or recovery path was added; receipt audit and redo
  replay it like any inline mutation.
- **Indirect writers:** ADOPT_FILE lists the member as protected; IMPORT_APPLY refuses any
  difference (an imported ticket can never carry one).
- **Policy:** OWNER default; OPERATOR only through an explicit row (ExplicitGrantOperations).
- **Docs:** `docs/TASKS-EXTERNAL-AGENTS.md` documents the verb and payload.

## Decisions

Verb, required `--expected-revision`, revision +1 with acceptanceRevision unchanged, OPEN and
NATIVE only, DUPLICATE_ID per acceptance revision, request-id replay, OWNER/explicit-OPERATOR role
and the 32/16/512 bounds are recorded with their reasons in the spec's Resolved decisions. Where the
ticket left a fork open (HELD tickets, OPERATOR default), the fail-closed option was chosen and is
listed as an owner question. "Record unchanged" is interpreted as unchanged except the attachment
member and the four finalizer fields, because a receipt-backed write must chain
previousRecordSha256.

## Limits

- Digests are unverified claims; nothing reads the member for gates, criteria or completion.
- Archive export/import has no carrier for the member; the closed importer key set refuses it.
- No remove/edit verb.

## Evidence

- Focused tests: `go test -count=1 -timeout 30m` over `./internal/tasks/...` and
  `./internal/taskman/...`, plus `./internal/specindex/`; `go vet` over the same packages; gofmt
  clean.
- Negative controls: disabling the OPEN guard, the IMPORT_APPLY guard or the ADOPT_FILE protected
  entry each makes the matching `TestTEAV0001_*` test fail.
- `make gate` and `go test ./...`: NOT_RUN (lane rule; `corvint affected` lists them as mandatory).
