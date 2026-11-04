# 2026-11-02 lane htg-u25: Codex review fixes

The lane's Codex review (merge-base `c605450399873cca71193213c5aa4d3f7e43624b`..`628b390d5`) returned
one P1 and three P2 findings, verdict FIX. Each finding was checked against the code first; all four
held and all four are fixed. The lane entry is `2026-11-01-htg-u25`.

## Dispositions

| Finding | Disposition | Change |
| --- | --- | --- |
| 1 (P1) `script/batch-precheck.sh` declared inputs omit files the node tests read | fixed | `UNATTRIBUTED_INPUTS` adds `cmd/corvint/pi_adapter.go`, `docs/specs/pi-workflow-v0.md` and `conformance/`; `inputs_cover` keeps the FAIL when coverage is uncertain |
| 2 (P2) git-quoted paths bypass the prefix comparison | fixed | the diff and untracked listings are read NUL-delimited (`-z`), so a quoted name compares as its path |
| 3 (P2) `script/host-adapters-test.sh` cancellation does not reach its children | fixed | children run as supervised background jobs in their own process group; the trap forwards the signal, escalates to SIGKILL after `HOST_ADAPTERS_GRACE` (10 s), then exits and removes the scratch directory |
| 4 (P2) `strings.Fields` splits checkout paths with spaces | fixed | `goListDirs` splits `go list` output on line ends only |

## Finding 1: verification and design

At `628b390d5`, `integrations/pi/capabilities.test.mjs` reads `../../cmd/corvint/pi_adapter.go` and
`../../docs/specs/pi-workflow-v0.md`, and `integrations/opencode/evidence.test.mjs` reads
`../../conformance/opencode-native-v1/results/`. The `host-adapter-node-test` recipe runs both files.
So a change confined to `cmd/corvint/pi_adapter.go` could be reported as unattributed. The finding held.

The declared list alone would drift again, so `inputs_cover` scans the files at run time:
- It scans the test files the Makefile recipe names, plus the tracked non-test `.js`/`.mjs` under their directories.
- It resolves every parent-relative string literal (`..`, `../x`) against the file's directory.
- It reports uncertain coverage (introduced FAIL) when:
  - a literal resolves to the root, outside the repository, or outside the declared inputs;
  - a literal names a module outside the scanned directories;
  - the recipe names no tracked test file.

Fork decision: the scan is limited to the recipe's test directories. `integrations/pi-protected/build.mjs`
resolves `../..` to the root, and the step never runs it; scanning all of `integrations/` would make
every failure uncertain.

Retained limit (spec failure mode): a path that a test computes without a parent-relative literal, or
reads relative to the working directory, is invisible to the scan. The WARN still withholds the receipt.

On the real tree, the scan passes with the five inputs. It refuses the old two-input list, and it refuses
lists missing either `docs/specs/pi-workflow-v0.md` or `conformance/`.

## Finding 3: verification and design

The old script ran `go build` and `node --test` as foreground commands. Bash defers a trap until the
foreground command returns, and the trap only called `exit`. Run against the old script, the new test
hangs until its 60 s watchdog kills the script (rc 137).

The new script makes these changes:
- `set -m` puts each child in its own process group.
- `supervise` backgrounds the child and waits on it, so the trap runs at once.
- `cancel` forwards the signal to the group. If the leader exits while group members remain, it waits
  for a SIGKILL escalation, which the grace period bounds. Then it exits 130 or 143.
- The EXIT trap removes the scratch directory last.

New maintained test: `script/host-adapters-test_test.sh` (target `host-adapters-test-test`).
- It uses a stub runner whose grandchild honours TERM, then one whose grandchild ignores TERM.
- Each case must exit 143, leave no live grandchild and leave an empty scratch parent.
- The target is outside the gate because it is a process-timing check, like `public-release-check-test` (ASD-V0-942).

## Requirement IDs

- ASD-V0-110: the text and its trace row are updated for the five inputs, the NUL-delimited diff and the coverage scan.
- No new ID was added. V1-4147 (finding 3) has no requirement ID, and finding 4 is test-only.

## Tests run

Tests ran on WSL with `GOTOOLCHAIN=local GOCACHE=/tmp/corvint-go-build-cache`.
`corvint affected --base c605450399873cca71193213c5aa4d3f7e43624b` was scope=UNKNOWN (218 selected,
77 unknown: 74 LANGUAGE_FRONTIER, 2 UNSELECTED_BINARY_CONSUMER, 1 UNOWNED_DIRTY_PATH). Only focused
tests ran.

- `make batch-precheck-test`: PASS. Section 6 now also covers:
  - a `cmd/corvint/pi_adapter.go` change;
  - an untracked and a committed `integrations/pi/régression.test.mjs`;
  - a recipe test that reads an undeclared `internal/tool.go`.
- `make host-adapters-test-test`: PASS (2 s). It FAILs against the old script (exit 137 from the watchdog).
- `./internal/companionrelease` TestTasksExportCoversCorvintTasksImportClosure and TestGoListDirsKeepsDirectoriesWithSpaces: PASS (heavy.sh).
- `make -s --keep-going unbounded-readers-check shell-test-registration-check test-commands-check`: PASS.
- `script/check-bash32.sh`: PASS.
- shellcheck on the changed regions: no new warning.

## Friction

1. **A PreToolUse hook rejects `go test` that is not wrapped in `script/heavy.sh`, even for a one-package focused run.**
   The fix prompt says to run focused tests with `GOTOOLCHAIN=local GOCACHE=...` but does not mention heavy.sh.
2. **`pkill -f "sleep 300"` over ssh matched and killed the ssh command's own shell.**
   Its command line contained the pattern. The cleanup of a manual regression run lost its output.
3. **Registering one new shell test touches three places.**
   They are a Makefile target, the gate or out-of-gate comment, and the unbounded-readers rules (`docs/SHELL-TEST-REGISTRATION.md`).
4. **`corvint affected` stays scope=UNKNOWN for this lane (218 selected).**
   The `Makefile` edit widens it further, so it cannot narrow the fix round's tests.
5. **WSL `/tmp` is 82% full.** heavy.sh warns `/tmp free=2G floor=3G`.

## Landing review fix (2026-11-04): relative overrides

The landing review found that `script/host-adapters-test.sh` changed directory before using its
overrides, so `CORVINT_TEST_REAL_BINARY=./corvint` (and a relative `CORVINT_TEST_NATIVE_FIXTURE`)
reached the suite relative to `integrations/`, and a relative `HOST_ADAPTERS_NODE` passed the
availability check but failed at launch (`env: './runner': No such file or directory`, exit 127).
The script now resolves both binary overrides, and `HOST_ADAPTERS_NODE` when it contains a slash,
against the caller's working directory before any `cd`; a bare node name stays a `PATH` lookup.
`script/host-adapters-test_test.sh` adds `relative-binaries` and `relative-node`, run from a caller
directory with relative overrides; with the previous script `relative-binaries` fails exit 9 naming
`./fixture` from `integrations/`, and a relative runner fails exit 127. No requirement ID (V1-4147
has none).
