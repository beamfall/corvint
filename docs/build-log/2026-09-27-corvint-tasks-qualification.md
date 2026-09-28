## 2026-09-27 CAL-V0-019..020: lease qualification and the execution cutover (S7)

S7 of `docs/specs/corvint-tasks-agent-leases-v0.md` qualifies the `external-agent` lease verbs and
adds the record that opens claims on a non-fixture queue.

CAL-V0-019, the qualification suite (`internal/tasks/store`, `TestCALV0019_*`):
- Eight claims of eight tickets over one path, sent at once, admit one; the rest refuse
  `RESOURCE_COLLISION`. Two claims of one ticket also admit one.
- `claim`, `renew`, `reap`, `widen`, `release`, `submit`, `gate run` and `complete` over four
  attempts, sent at once, each commit or refuse, and the store audits `CONSISTENT`.
- A released, reaped or wrongly numbered generation is refused `FENCED` by every verb, and the
  attempt file keeps its bytes.
- For every lease verb and every artifact it publishes, an injected failure just before that
  artifact leaves the head unchanged. Before the receipt, the retry commits afresh; from the receipt
  on, the next writer redoes it and the retry replays.
- A writer process killed just before each artifact of a `claim` and of a `gate run` leaves
  staging slots behind. The retry, in a new process, clears them, completes the transaction and
  audits `CONSISTENT`.

Two defects the suite found, both fixed:
- A killed writer's staging slots were refused `MALFORMED` `unassigned stage slot` by every later
  command, so one crash stopped the store. Every writer now removes them under the writer lock
  before it redoes or models anything. Slots beside a `staging/active.json` descriptor are active
  staging, which is still refused `UNSUPPORTED`. Barrier and reconcile writes do not recover.
- `gate run` modelled its attempt before taking the lock, so a pending receipt or orphan slots
  refused it. It now settles the store under the lock first.

CAL-V0-020, the execution cutover. `corvint-tasks cutover --execution --decision REF
--qualification FILE` reads FILE as the `go test -json` output of the suite and commits one
`QUALIFICATION` receipt under an `OWNER` binding. The receipt posts the run as an evidence blob and
sets the queue's `executionCutover` to the decision and the run's digest. The spec named
`--qualification <digest>`. A file was taken instead, because the store has to see the run to
check that every suite test passed and none failed. The model refuses:
- a non-`OWNER` binding;
- a fixture queue;
- a queue whose writer is not yet `NATIVE` (`CUTOVER_MISSING`);
- a queue already cut over;
- a run with a failing test (`GATE_FAILED`), a run missing a suite test (`MISSING_EVIDENCE`), and a
  run that is not `go test -json` (`MALFORMED`).
The claim checks now test `executionCutover` itself rather than the fixture flag alone.
`init` still refuses a queue that names an execution cutover.

Rehearsal: a disposable non-fixture store imported all 2,894 Beamfall export items. A claim
before cutover refused `CUTOVER_MISSING`; authority switch receipt 416 and execution qualification
receipt 417 succeeded, and the repeated execution request replayed. A five-minute lease expired
and was reaped; renewal of that generation refused `FENCED`. Generation 2 claimed and submitted
the ticket. After the handoff, renewal, the required gate, fast-forward integration, completion,
and receipt audit all succeeded. The ticket is `COMPLETED`; the audit is `CONSISTENT`.
The required gate used a `printf ok` stand-in, so this proves the store lifecycle and receipt
checks, not Beamfall's real build gates or production cutover. Raw local evidence is retained in
`scratchpad/r7/{run.log,transcript.txt,resume-transcript.txt}` beside the S7 handoff.

Rollback: revert the change. The reverted `validateInput` refuses a non-fixture queue that names an
execution cutover `MALFORMED`, so a store that recorded one refuses every write until the change is
reapplied; the `QUALIFICATION` receipt kind predates this change. A killed writer again stops the
store until its slots are removed by hand.

Independent review found that the initial verifier admitted a truncated qualification stream
without a final package pass. The repair requires the store package's start, each named test's
run/pass pair, and its final pass; any later failure still rejects the run. Null/array events,
unknown actions, duplicate or case-folded event keys and invalid field types refuse `MALFORMED`.
The qualification remains owner-supplied evidence, not authenticated execution attestation.

Corvint self-use: `query` and `affected` ran against base `448b2c23`; their original outputs
are retained beside the handoff as `s7-query.json` and `s7-affected.json`. The query retained
mixed-worktree state and withheld test candidates. The affected plan retained language-frontier
and unowned-document unknowns. Initial `dogfood-change` refused dirty-worktree coordination and
binding; its report retained missing pre-change agent receipts. Final CEM/OCM binding is performed
after commit in the handoff's plain clone. Repository-wide `make gate` is `NOT_RUN` under the
owner's scoped-work instruction; focused task tests and document checks are the selected checks.
Retrieval/ranking evaluations and optional providers are not applicable to this task-store change.

Final parser compatibility was checked against the preserved real Go 1.27 qualification output:
the repaired CLI initialized a fresh non-fixture store and admitted the execution cutover.
Go 1.27 `OutputType` and build-event `ImportPath` fields have explicit regression coverage.
The focused task package gate passed at `3c5116b9`; the line-citation gate found an S7-shifted
reservation-check anchor, which was read and repinned before the final gate.
