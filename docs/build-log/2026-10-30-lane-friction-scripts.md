# Lane script and gate friction — V1-3228, V1-3229, V1-3230, V1-3233, V1-3232, V1-3120, V1-3238, V1-3227, V1-3222

One lane (`claude/lane-friction-scripts-w4`, base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`,
WSL lane host, 32 CPUs) for the open script and gate friction tickets. Requirements:
DCG-V0-048, UCV0-020, GAG-V0-009, ASD-V0-070 to ASD-V0-075.

## Decisions

- V1-3228 (DCG-V0-048): `--repin` now stages what it rewrote and reruns itself (at most five
  passes). Staging is safe because `--repin` already rewrites only a document whose worktree copy
  equals its index copy, so the `git add` adds nothing but the rewrite. Printing the `git add`
  command, the other admitted fix, still left the two-step dance the ticket measured.
- V1-3229 (UCV0-020): the runner stays a pure reader (no Git, no subprocess); it only prints the
  recovery to stderr. A receipt that left its ledger pin is restored from HEAD first, since the
  repin script refuses such a receipt by design (UCV0-019).
- V1-3230 (ASD-V0-072): refuse, never unshallow, because unshallowing fetches from the network.
  `corvint affected` refuses only with `--base`; the dogfood wrappers carry the check in the
  sourced `dogfood-build-corvint.sh` because every fixture already copies that file, while
  `batch-precheck --base`, `batch-review-packet` and the new `script/lane-setup.sh` share
  `script/require-full-history.sh`.
- V1-3233 (ASD-V0-070): a named refusal with its own status (3) for batch-precheck and
  batch-regen, and the same `home-unset` refusal (status 2, their refusal status) for the dogfood
  wrappers, which the same `env -i` probe showed abort the same way. heavy.sh simply has no
  default config file without HOME.
- V1-3238 (ASD-V0-073, GAG-V0-009): the precheck hashes the manifest's legal files itself (a
  second, cheap reader of the same pins), and the archive gate now appends the loose gate's FAIL
  lines instead of replacing them with its summary.
- V1-3227 (ASD-V0-071): one script serves the range check (a batch-precheck step, HEAD in its
  command so a ledger pass never covers a reworded commit) and the commit-msg hook. The installed
  hook runs the committing worktree's copy, so removing one lane checkout cannot break the others.
- V1-3120 (ASD-V0-074): small packages are exempt below a measured cost threshold, limited to
  `tools/` and `conformance/`, whose cost follows their size; `cmd/` and `internal/` do not.
- V1-3232 (ASD-V0-075): split the serial parts of the critical path rather than drop a check.
- V1-3222: the repository side already existed (`HEAVY_CONFIG`, default
  `~/.config/corvint/heavy.env`); the gap was the host configuration and its documentation.

## V1-3120 cost measurement

Whole-package `go test -count=1` through `script/heavy.sh` on the WSL host, 2026-10-30:

| package | tests | `*_test.go` bytes | wall |
|---|---|---|---|
| tools/test-read-scopes-check | 6 | 21241 | 15.8 s |
| tools/test-commands-check | 7 | 17727 | 0.6 s |
| tools/gate-ledger | 26 | 44860 | 7.0 s |
| tools/test-shards | 4 | 4010 | 5.8 s |
| tools/capabilities-check | 21 | 31360 | 4.7 s |
| conformance/use-cases-v0 | 7 | 16817 | 0.3 s |

The threshold is 49152 bytes (48 KiB), just above the largest measured package.

## V1-3232 profile and before/after

Both measured on this WSL host; the ticket's Mac numbers (72 s and 92 s) were not remeasured.

`make batch-precheck` with `CORVINT_GATE_LEDGER=off`, before (clean worktree of the base):
88.3 s wall, 184 s summed. Critical path: `batch-drift` 78 s, then `host-adapter-node-test` 9 s
alone. Inside `batch-drift`, `go test -v` showed `internal/companionrelease` at 54.6 s:
`TestCorvintBundledBinariesBuildTwiceIdentically` 41.6 s and
`TestCurrentTasksSourceArchiveBuildsOffline` 13.1 s, one after the other in one package; every
other drift test took at most 5.3 s. `make doc-gates` ran as one 39 s step whose 14 targets and
`go run` (16 s in total when run one by one, warm) ran in sequence.

After (commit `80fce8564`, same flags): 68.6 s wall (-22%). The two companion-release tests run as
their own steps (55 s and 37 s, beside the 36 s union of the rest), the doc-gates targets run as
15 parallel steps (the longest 13 s), and six steps run at a time. That run included the 13 s
rerun of one failing step (fixed before this entry), so a passing run is no slower.

`make lane-scripts-test`: before 37.5 s for 28 groups; after 33.9 s for 30 groups (two new
groups). Groups each take 0.1 to 5.7 s; the largest single cost was the load-wait group, whose
5-second poll outlived its 1-second bound (now capped). The rest is many short fixtures; running
groups in parallel would need the suite split into independent files, left for a later change.

## V1-3222 host cutover (WSL)

Before: `~/lanes/heavy.sh` took `mkdir` slots in `~/bench/rt/.slots` (4), lanes' `script/heavy.sh`
took `flock` slots in `/tmp/corvint-heavy-1000` (8), and `~/bench/rt.sh`, which the Mac's offloads
run, took `flock` slots in `~/bench/rt/.locks` (4). Now `~/.config/corvint/heavy.env` sets
`HEAVY_LOCK_DIR=/home/russell/bench/rt/.locks`, `HEAVY_SLOTS=4` and
`HEAVY_SELECTION_LOG=/home/russell/lanes/heavy-selection.log`, and `~/lanes/heavy.sh` delegates to
the checkout's `script/heavy.sh` (backup `~/lanes/heavy.sh.bak-20261030-v1-3222`), so all three
share one semaphore. `script/heavy.sh --print-lock-dir` and `~/lanes/heavy.sh --print-lock-dir`
both print `/home/russell/bench/rt/.locks`, and the next run reported `slot 1/4`. Running
commands kept the slots they held. `~/corvint` (Oct 22) predates `HEAVY_CONFIG`, so the wrapper
refuses outside a current checkout rather than fall back to it.

## Independent review and renumber

A Codex review of `c5d5f0829..2f9f10e55` (run by the orchestrator) returned FAIL with eight
findings, all fixed in one commit set: the `ai-trailers` step is never downgraded to base-present
under `--base` (its base rerun checks the same pinned range); `--repin` stages with literal
pathspecs and exits non-zero on a failed `git add` or a fifth pass that still rewrote; `--install`
replaces only a byte-identical managed hook; every Git read in the trailer range check is checked;
the commit-msg form checks `#` lines above the `-v` scissors line; `lane-setup.sh` resolves relative
paths once and exits 1 when the hook install fails; and `corvint affected` refuses a shallow clone
before resolving an abbreviated base. The lane's ASD-V0-060..065 collided with other lanes and
became ASD-V0-070..075.
