# Dispatch command-reader lifecycle integration — #464

Status: experimental implementation; terminal qualification and native V1-0654
completion remain open. This entry records a preparation checkpoint, not a gate
receipt or acceptance of the proposed Process-group Owner contract.

The original cancellation repair now uses the creation-owned Owner API already in
the integration base. One reader slot and a fixed retirement bound prevent a late
join or cancellation retry from granting another cleanup allowance. A pre-spawn
UNKNOWN marker survives process restart; only matching identity/bytes are cleared
after RELEASED/result handling or a start failure with no owned child. HOLD and
marker failure stop the invocation, normal Close writes and later Tick/Run effects.
Status reports reader containment separately from dispatcher process liveness.
The CAL-V0-053 amendment and traceability table describe the exact behavior.

The integration base is `4b10a02144faed4a77b36eba4b0d1cbc315706d3`.
Source construction starts at `ccee056f8b486dc0c5d49b0644eebe4051e22758` with
ten frozen source/test paths. Original enrollment key and successor plan are
preserved; the five checks are unchanged. No existing sealed archive is removed.

## Evidence retained at this checkpoint

Repair1 actual macOS full dispatch, selected reader/existing CLI, focused race and
vet checks passed. Linux arm64 test binaries built successfully. These are source
implementation evidence; they are not final-commit enrolled observations.

The initial dispatch run failed two existing witnesses:
`TestCALV0056_CancelledHealingStopsNextWrite` (release and reap) and
`TestCALV0064_CheckedSaveFailureDoesNotGrantOrEscapeThroughClose`.
Repair1 added context guards before each healing write and moved the save-failure
fixture's permission change to its actual checked-save callback. The test preserves
its ledger/backoff/unpark/Close assertions. Direct-reader fixtures now use private
state directories. Raw failures remain retained after passing repair checks.

Repair1 Linux interruption job r08 remains HOLD/exit1: the ready fixture received
SIGTERM but its controller did not join, and its Docker client remained. Separate
recovery checked exact creation-owned container ID/name/image/nonce, stopped it,
copied evidence, removed it and proved absence; subsequent process observation
found no old client/group members. This was containment, not a passing interruption
proof. A reached-frame comparison diagnosed Python PollSelector swallowing builtin
InterruptedError; a custom BaseException propagated and joined its captured child.
The reviewed private harness repair changes only those two signal handlers.

The repaired Linux arm64 interruption proof (`l01`) then passed with exit 0 and owned
cleanup proved. The Linux runtime job (`l02`) ran the full dispatch package: 88 of 88 cells
passed, including all 18 required, with no fail or skip. It exited 1 only because all
five CLI witnesses failed during fixture ticket creation, before any reader behavior ran:
the CGO-disabled test binary under uid 501 had neither `$USER` nor
`CORVINT_TASKS_ACTOR`, so `ticket create` refused with no local operator identity. The
CLI rerun used the same image, the same checksum-bound `cli.test` binary, the same ten
payload files (matching the source byte for byte), `--network none --cap-drop ALL` and
uid 501. Its only change exported `CORVINT_TASKS_ACTOR=gh464-linux-qualification`. All
five top-level witnesses passed (seven cells including subtests), with no fail or skip,
and the container was removed. The fixture-identity failure stays retained as raw
evidence. It is a harness-environment defect, not a product change.

Darwin, Linux and FreeBSD each passed `go vet` on amd64 and arm64 for the dispatch and
CLI packages, and FreeBSD/amd64 test binaries compile. On FreeBSD the dispatch package
reports no test files, so no FreeBSD runtime behavior is claimed. Linux amd64 runtime
remains unclaimed.

Final metadata review, original five keyed checks on the final clean binding,
semantic CEM citations, scoped OCM/report inspection, strict clean dogfood check,
seal, independent implementation review, publication/integration and native
completion are NOT_PRODUCED. The full repository gate is NOT_RUN under the scoped
work policy. Earlier unproduced context and evidence remain visible in the retained
packets; a later query cannot repair chronology.

Local retained evidence: `/private/tmp/corvint-464-code-prep-20261004/evidence/`,
`repair-06/evidence/` and `repair-06/linux-repair2/` under that same packet. The
terminal metadata proposal retains hashes of the exact reviewed inputs. Private
paths identify operator evidence and are not portable or stable release claims.

Rollback keeps the original source commits, frozen enrollment and failure evidence.
Quiesce owned readers before reverting; retain unresolved markers. Downgrading or
removing a marker does not establish containment. No automatic recovery, power-loss
durability or escaped-session containment is claimed.
