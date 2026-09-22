# Decision 0105 — SRG-V0, GAG-V0 and OACS-V0 stay proposed, each on one named unmet clause

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

The owner ruled on 2026-09-07 that gate tooling is in scope for invariant 8. Three owning specs
were still `proposed`: `spec-requirement-index-generator-v0.md` (SRG-V0),
`go-archive-gate-v0.md` (GAG-V0), and `optional-artifact-check-scripts-v0.md` (OACS-V0). Each was
reviewed clause by clause against its script at `3c41cad3`, and its tests or gate member were run.
The rule applied: accept a spec only when every clause's evidence passes, and otherwise leave it
`proposed` with the failing clause named. A `NOT_RUN` cell that the spec already discloses is not by
itself a failure; a clause the script demonstrably violates is.

The owner calls:

1. **SRG-V0 stays proposed on `SRG-V0-005`.** `make spec-requirements-check` and
   `make spec-requirements-test` exit 0, and generating from cwd `/` reproduces the committed index.
   `SRG-V0-001`'s index-enumeration defect is repaired. `SRG-V0-005` requires titles of at most 80
   characters, but `script/gen-spec-requirements.sh:108@ec359190` truncates undecoded bytes. A scratch fixture
   title containing `é` came out at 75 characters, ending in an invalid UTF-8 byte.
2. **GAG-V0 stays proposed on `GAG-V0-006`.** `make go-archive-gate-test` exits 0, and
   `make go-archive-gate` exits 0 at `3c41cad3` (`targets=5 verdict=PASS`, 5m10s). `GAG-V0-007`'s
   stale-witness defect is repaired. The negative paths were run once with `go` stubbed on `PATH`, as
   an uncommitted scratch check. Missing, extra-regular-file, and symlinked-name outputs failed. So
   did a failing build and a wrong-revision witness. A read-only cache directory and `TERM` both
   cleaned up. An extra top-level directory or symlink exited 0, because the count uses `-type f`.
   `GAG-V0-006` requires exactly seven top-level entries as a closed set, so the clause is not met.
   The review also corrected the spec's claim that an uncommitted change passes this gate. The
   delegated `archive` command refuses any non-clean worktree, untracked files included, so such a
   change fails the gate.
3. **OACS-V0 stays proposed on `OACS-V0-002`.** Its three stub-driven tests
   (`make analyzer-python-offline-build-test`, `analyzer-python-ratchets-test`,
   `release-artifact-reproducibility-test`) exit 0. Case 4 pins that the Core dependency refusal
   never fires. That is the named gap, already filed in `bugs.md`. The cells those tests do not
   cover stay `NOT_RUN`.
4. **EPG-V0 is not on this branch.** There is no `eol-policy-gate-v0.md` in `docs/specs`, so
   `script/check-eol-policy.sh` is still owned only by decision 0061.

Narrowing a clause to fit current behavior was rejected in each case. The two new gaps are filed in
`docs/agent-memory/bugs.md`, each with its fix and the evidence cell to flip. A spec is accepted by a
later decision once its fix lands.

Rollback: revert this decision's commit. That restores the prior digest, evidence, and backlog
text; no script or status changes.

## Accepted amendment 2026-09-12 (same day): SRG-V0 and GAG-V0 are accepted

Both named clauses are repaired, so the rule above now accepts both specs. OACS-V0 is not decided
here. `OACS-V0-002` was repaired separately at `ccdd39da`, but its re-review against this rule has
not been done. EPG-V0 is still absent from `docs/specs`.

1. **SRG-V0 is accepted.** At `42f41ca8` the generator decodes UTF-8 around the 80-character cut,
   and `script/gen-spec-requirements_test.sh` case 4 requires a 90-character `é` title to keep
   exactly 80 characters. Before the repair the case failed at 40 characters (80 bytes).
   Regeneration lengthened ten committed rows to their full 80 characters. The committed index is
   valid UTF-8, and no title exceeds 80 characters. A title that is not valid UTF-8 is still cut by
   byte, as the spec discloses.
2. **GAG-V0 is accepted.** At `3b01a0b0` the gate counts every top-level entry and also counts
   regular files. `script/go-archive-gate_test.sh` case 3 requires an extra top-level directory and
   an extra symlink each to fail; both exited zero before the repair. The real gate's negative cases
   stay `NOT_RUN`, as disclosed.

Evidence at `3b01a0b0`, each exit 0: `make spec-requirements-check`, `make spec-requirements-test`,
`make go-archive-gate-test`, and generating from cwd `/` byte-identical to the committed index.
`make go-archive-gate` printed `SUMMARY revision=3b01a0b006f31d4dc25023b571d8e2611930f266
targets=5 verdict=PASS` in 6m00s.

Rollback: revert this amendment's commit. That returns both specs to `proposed`; the two repairs
stay in.

## Accepted amendment 2026-09-12 (same day, second): OACS-V0 is accepted

`OACS-V0-002` was repaired at `ccdd39da`, so the rule above was reapplied to OACS-V0 clause by
clause. The three scripts are unchanged since `ccdd39da`; each was read against its clause, and its
cited test was run at `ce4204e4`. EPG-V0 is still absent from `docs/specs` and is not decided here.

1. **`OACS-V0-001` holds.** The script takes `${1:?}` and `test -d`, and refuses via
   `test ! -e ROOT/vendor`. It allocates three `mktemp` paths under `TMPDIR`, runs `cd ROOT`, then
   `go build` and `go list -deps` under the six pinned variables with `set -e`. A scratch stub run
   also refused a non-directory root and an empty argument, and failed with the list's exit 7.
2. **`OACS-V0-002` holds.** An exact `grep -qx` line now exits 1 with the message. The
   `cache/download` filter exempts only `/github.com/corvint-context/corvint/@v/`.
3. **`OACS-V0-003` holds.** The trap is installed after all three allocations and removes all
   three. Test cases 2-3 and the scratch failing-list run left no allocation behind.
4. **`OACS-V0-004` holds.** The script checks `${1:?}`/`${2:?}` and `test -d`, then `-f` with
   `! -L` on each final entry, and `wc -c` with `-le` at 65,536, 4,096, and 6,291,456 bytes. A
   scratch run refused a directory binary, a FIFO binary, a non-directory root, and empty
   arguments.
5. **`OACS-V0-005` holds.** The script reads nothing but the three sizes, and case 5 admits an
   empty binary.
6. **`OACS-V0-006` holds.** `ROOT` comes from the script directory. The lexical `case` refusal
   prints the message and exits 2 before `mkdir -p`, then runs `GOTOOLCHAIN=local exec go run` with
   the exact arguments. The test was run with `GOTOOLCHAIN` unset, so the pin comes from the
   wrapper. A scratch output below a regular file failed at `mkdir` without invoking `go`.
7. **`OACS-V0-007` holds.** `exec` preserves output and status (cases 3-4), and the argument list
   never names `archive`. One real run printed the delegated `SUMMARY` line with
   `smoke-not-run=4 ... verdict=PASS pending-evidence=W10-performance-GPK-V0-016-017`. Its
   `report.json` keeps `pendingEvidence` and eight `NOT_RUN` values and never mentions `archive`.
8. **`OACS-V0-008` holds.** The only callers are the three opt-in targets, at `Makefile@3c41cad3` as reviewed above (lines may have moved since).
   None is in `gate`, `gate-affected`, or `.github/workflows`.

Evidence at `ce4204e4`, each exit 0: `make analyzer-python-offline-build-test`,
`make analyzer-python-ratchets-test`, and `make release-artifact-reproducibility-test`, all under
`env -u GOTOOLCHAIN`; `go test ./conformance/release-artifact-v0 -run
'^(TestOutputInsideRepositoryIsRefused|TestSummaryCountsNotRunSmokeSeparately)$'`. One-time real
runs, not retained as receipts: `script/check-release-artifact-reproducibility.sh` with a scratchpad
output (`targets=5 byte-identical=5 of 5`, `verdict=PASS`, 1m50s); the offline build over the
worktree with `GOTOOLCHAIN=local` (7s, no temporary path left); and the ratchet over a
`-trimpath -ldflags='-s -w'` build (2,781,266 bytes binary, 32,997 and 874 bytes source). The spec's
`NOT_RUN` cells for the committed tests stay as disclosed.

Gate membership, stronger toolchain enforcement, path containment, and failure cleanup were left
open by the spec's own promotion section. Acceptance does not decide them, and the spec text now
says so instead of tying its intent status to them. Delivery stays `experimental`.

Rollback: revert this amendment's commit. That returns OACS-V0 to `proposed`; no script changes.
