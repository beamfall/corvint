<!-- SPDX-License-Identifier: Apache-2.0 -->
# Tasks store lane review corrections

Base: `6c33df33bc5670131482f5263b410cb980580657`; incoming round-0 head:
`a6eb8ed06e9a403fffc7b56f95954a825dfef0ad`. Human-owned scope is the
pk-tstore FIX ROUND 1 charter and its six independent-review findings. This is
an additive repair; no history rewrite, publication, native ticket writes or
local independent review is authorized. The orchestrator owns independent
review, integration and ticket completion.

## Decisions and acceptance

1. **V1-0775 / ATR-V0-7203:** restore `observation.Files` reuse after a settled
   successful preparation audit. Reuse supplies only metadata; the final content
   binding still runs under the writer lock. The paired experiment compiles the
   identical opt-in `TestATRV07203_PairedHoldSample` worker against BASE and the
   candidate, alternates BASE/HEAD for five pairs at both 13,500 and 40,000
   receipts, and records one-minute/five-minute/fifteen-minute load before and
   after each claim and complete sample. The remeasurement continues on the same fixtures
   with the next tickets (preliminary repetitions 5..9; final repetitions 10..14), without reset or journal rewrite.
   Nominal history sizes are 13,500/40,000; actual pre-operation sequences are
   n+1+4*rep for claim and n+4+4*rep for complete (one policy update plus
   claim/submit/gate/complete per earlier sample). Both measured operations explicitly
   discard the derived writer checkpoint and process audit cache, exercising
   the complete route. Setup, submit and gate run are unmeasured. Each version
   owns a separate equivalently generated 880-ticket, segmented fixture. The
   requested 1.5x criterion is applied to the per-verb median hold ratio at 40k.
   The conservative narrowing binds all current intent/state and evidence, the
   head receipt, the requested replay projection and all live request projections
   physically read by the audit. It removes historical receipt and sealed request
   body rereads from the lock; their audit and capacity metadata stay unchanged.
   This retains more than just the target ticket because capacity/dependencies
   and gate decisions can depend on other current records/evidence. Recovery and
   routes lacking a settled physical observation retain the full pass. Mapped
   policy, request-projection and ticket tests must still refuse both boundaries.
   Raw results and source/binary bindings are retained under
   `.agent-evidence/pk-tstore-fix1/`. Host load above 16 makes the entire run
   **NOT_QUALIFIED**; paired numbers are diagnostic, not a quiet-host promotion.
2. **V1-4573 superseded by `claude/v1fix-a` / DWC-V0-929:** drop the duplicate
   report-publication implementation, regression, ignore pattern, LCP-V0-7216
   clause and associated round-0 repair-only evidence. `internal/dogfoodflow/`
   must have an empty diff from BASE. The known cross-mount EXDEV remains an
   allowed final-check blocker; the orchestrator runs the host check with the
   sealed lane's fix. It is not bypassed or claimed passing here.
3. **V1-4588 / ATR-V0-7202:** tolerate identity-read failures for two ticks,
   retiring only after three consecutive errors. A readable same identity resets
   that counter; a successful empty or changed identity retires immediately.
   Protocol-directory loss/replacement still retires immediately. PID 1 is a
   valid spawning owner when readable. `TestATRV07202_OwnerIdentityReadFailures`
   exercises recovery, reset, threshold, absence and replacement;
   `TestATRV07202_ReadablePID1Owner` uses the initialization seam. The existing
   actual owner/directory-loss process regression remains required.
4. **V1-3942 / TCL-V0-7206:** help and intent name the closed wire code first:
   `MALFORMED (detail COMMIT_AMBIGUOUS)` and
   `MISSING_EVIDENCE (detail COMMIT_NOT_FOUND)`. `CommitPrefixAmbiguous` lives in
   `store/review_packet.go` and counts commit objects only. Enumeration/type-read
   failure abstains with an error rather than inventing ambiguity; enumeration
   is bounded at 1 MiB. `TestTCLV07206_CommitPrefixCountsOnlyCommits` constructs a
   real one-commit/one-blob prefix collision and verifies unique commit
   resolution. The native CLI completion test now checks both wire codes as well
   as detail text.
5. **V1-4025 / TCD-V0-7212 release note:** an interrupted legacy multi-target
   command retried after upgrade may apply twice because the new derived request
   ID differs from the legacy ID. Recover each target singly with its reported
   concrete requestId and original issuedAt. The spec and original release-note
   line both retain this caveat; the hashing implementation is unchanged.
6. **Evidence accuracy:** archive-reason `NOT_OBSERVED` is in `cli/cli.go`,
   `showReader.item`; `CommitPrefixAmbiguous` is in `store/review_packet.go`.
   V1-3903's criterion was satisfied by refutation: bounded valid receipts expose
   complete posts, so paging was unnecessary. V1-4204 remains **PARTIAL**:
   13.5k versus 40k total hold within 10% is **NOT MET / unqualified**; B2 p50
   494.696462 ms versus 50 ms is **NOT MET**. These fix-round paired measurements
   do not replace that qualification. Darwin APFS `TestATRV07203` is **NOT_RUN**
   here; the orchestrator runs it on Mac.

## Verification, limits and rollback

The start `make dogfood-change BASE=...` ran before edits; it retained missing
citation/intents/outcome inputs and `prechange-impact NOT_OBSERVED
agent-receipt-absent` rather than retroactively inventing pre-change evidence.
The affected route is used before Go tests. The installed binary reports stale
Core sources; its omissions stay visible. No ranking, learning, provider or CEM
wire algorithm changes require unrelated frozen evaluations. Optional external
services are not applicable; independent review and Darwin qualification are
explicitly deferred to the orchestrator.

`/var/tmp/pk-tstore-fix` cannot be created under the managed filesystem policy
(`mkdir: Read-only file system`); tests use `/tmp/pk-tstore-fix` and the required
shared heavy lock directory. Each Go invocation uses `script/heavy.sh` and one
package. Focused runtime tests, staged-spec generation and traceability/line
citation checks are retained with their results below. Exhaustive gate and
Windows vet are NOT_RUN: the owner requires focused checks and forbids pushes.

Rollback reverts these code/spec amendments together, retaining existing
completion-manifest readers. Reverting the content binding restores the mapped
write window and cannot be described as equivalent integrity. Do not rewrite
journal evidence or manually close tickets. The duplicate dogfood publication
fix belongs to v1fix-a and must not be reintroduced in this lane.

## Focused results

WSL2/Linux, Go go1.27.3, 32 logical CPUs. The final selected store run passes in
31.371 s (the earlier 24.671 s run exercised the preliminary selection), supervisor in 10.954 s, CLI in 2.572 s, and dogfoodflow's existing
`TestCheck*` tests in 0.208 s. Store tests include the new metadata-reuse and
commit/blob collision regressions, both sweep/lock mmap boundaries for policy,
request projection and ticket, CAL-V0-116 route parity and checkpoint decode
outside both locks. Opt-in scale/load tests skip unless explicitly enabled; no
new 200-trial spawn qualification or Darwin result is inferred from this run.

A narrow Go overlay restores round-0 `lease_write.go` and `review_packet.go`
while retaining the new regressions. Both tests fail for the expected reason:
`commit + blob: true <nil>` and `audited files reread during preparation`. This
negative control exits 1; it is failure evidence, not a passing verification
command. Current code passes both in the selected store run.

Staged `make spec-requirements` and `make traceability-tests-check
line-citations-check` pass. The existing 256-obligation warnings and 180 skipped
non-root line citations remain visible. There is no `.taskman` diff.

Corvint's final affected plan selects 140 units with scope UNKNOWN, including
LANGUAGE_FRONTIER and UNOWNED_DIRTY_PATH. Its whole-package advice for TestMain
changes is explicitly narrowed by the owner's fixed test selectors; it is not
claimed equivalent coverage. The context packet leaves four named requirement
anchors unresolved as ticket-shaped IDs and records 1,828 exclusions. Owning
clauses were found through REQUIREMENTS.tsv and read directly. These packets
are discovery, not authority. Windows CPU/power telemetry is NOT_OBSERVED; no
native Windows command or host-setting change was performed.

## Retained failed narrowing

The first narrowing selected live request projections by membership in
`PhysicalObservation.Files`. Contrary to the name/header, `segmentView.serve`
also puts logical sealed reads in that map to charge derived checkpoints. That
selection still rehashed historical request bodies. Its complete paired series
(`narrow.jsonl`) is retained as a failed evaluation, not final qualification.
The final implementation observes the bounded native request namespace under
the preparation guard, outside both locks, without rereading request bodies.
It still binds every native live projection and the requested sealed replay
projection. `TestATRV07203_SealedHistoryIsNotDecisionContent` builds a real
segmented store, demonstrates the misleading observation membership, excludes
unrelated historical bodies, and keeps/verifies current inputs and a sealed
replay. The final series uses the next five tickets on the same fixtures.

## Paired hold measurements (WSL2/Linux)

All figures are **NOT_QUALIFIED**. Each phase has five alternating BASE/HEAD
pairs at each nominal history size; each operation reports exactly one writer
lock. Values below are medians in milliseconds, with ratio HEAD/BASE. Full
phase sampled load1 26.04..78.42; preliminary narrowing 34.26..66.95; final
16.10..45.15. All exceed 16; no qualified performance or within-10% claim is
made. The final 40k ratios remain above 1.5 even after narrowing.

| Phase | History | Operation | BASE ms | HEAD ms | Ratio |
| --- | ---: | --- | ---: | ---: | ---: |
| Full content pass | 13,500 | claim | 74.950 | 829.249 | 11.064x |
| Full content pass | 13,500 | complete | 160.805 | 1020.034 | 6.343x |
| Full content pass | 40,000 | claim | 79.794 | 2532.222 | 31.734x |
| Full content pass | 40,000 | complete | 244.825 | 3345.315 | 13.664x |
| Failed observation-membership narrowing | 13,500 | claim | 83.255 | 361.149 | 4.338x |
| Failed observation-membership narrowing | 13,500 | complete | 206.219 | 579.212 | 2.809x |
| Failed observation-membership narrowing | 40,000 | claim | 78.259 | 843.430 | 10.777x |
| Failed observation-membership narrowing | 40,000 | complete | 150.719 | 942.678 | 6.255x |
| Final native-namespace narrowing | 13,500 | claim | 44.261 | 58.638 | 1.325x |
| Final native-namespace narrowing | 13,500 | complete | 53.184 | 79.534 | 1.495x |
| Final native-namespace narrowing | 40,000 | claim | 35.108 | 68.761 | 1.959x |
| Final native-namespace narrowing | 40,000 | complete | 63.275 | 113.780 | 1.798x |

The full 40k ratios (31.734x claim / 13.664x complete) triggered the requested
narrowing. The first narrowing was rejected for retaining logical sealed
requests; the final implementation is the bounded native-namespace selection.
Exact sample order, all loads, raw holds and paired ratios are in the committed
JSONL/summary files. The first driver was interrupted only during admission
before the first 40k sample; all setups had passed and no measured call was
interrupted or sample dropped. Its explicit continuation uses the same
binaries/fixtures. Subsequent phases continue with repetitions 5..9 and
10..14; no fixture or journal reset occurs.

Commands: `script/heavy.sh go test -c -o BINARY ./internal/tasks/store`
(with `go -C` for the exact BASE source archive), then the retained foreground
`paired.py`, admission-only `resume-full.py`, `remeasure.py` and
`remeasure-final.py` drivers. Every worker invocation goes through
`script/heavy.sh --admit-only BINARY -test.run=^TestATRV07203_PairedHoldSample$
-test.timeout=30m -test.v`, with its per-size/version fixture and sample
environment set by the driver. `summarize.py full narrow final` asserts all
120 operation samples and exact alternating order, then emits this table.

The BASE archive has 6,431 matching original Go/module files, zero changed
ones, and only the identical shared worker as an added Go file. Its omitted
unrelated paths are listed explicitly. Binary/source SHA-256 and Git blob
bindings distinguish full, failed preliminary and final implementations.

Final selected store PASS 31.371 s; CLI PASS 2.535 s; unchanged supervisor
PASS 10.954 s and restored dogfoodflow PASS 0.208 s are reused under their
source bindings. The failed 520-receipt sealing fixture is retained; it was
below the 1,024-receipt trigger and is corrected to 1,100 receipts. The final
sealed-history regression passes. Final spec generation and staged
traceability/line-citation checks pass. Post-commit CEM checking remains the
required terminal step; a report-write-only EXDEV is handed to the orchestrator
under the charter exception, without reintroducing v1fix-a's duplicate.
