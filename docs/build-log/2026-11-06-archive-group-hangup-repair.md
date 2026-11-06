# V1-4733: group hangup retires loose builds before wrapper cleanup

Independent review finding P2; requirement `GAG-V0-9962`.
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Repair starts at `228219aac41e4affbc81b820e856505ad51add8b`.

Loose builds have isolated process groups. The producer previously subscribed only to
SIGINT and SIGTERM, so group SIGHUP could terminate it without canceling those builds.
Adding SIGHUP cancellation alone also leaves the foreground `go run` launcher vulnerable:
its early exit lets the wrapper remove scratch while the producer is still joining workers.

Decision: subscribe the producer to SIGHUP and have the wrapper launch `go run` with SIGHUP
ignored. The producer's explicit subscription overrides that inherited ignore; the launcher
survives hangup and waits for producer exit, so the wrapper's pending hangup trap runs only
after the existing worker join and contained-command cleanup. Other signal policies, public
JSON/wire contracts, field names, error codes, exit codes, diagnostics, goldens and existing
assertions are unchanged. Rollback is to revert this repair as a unit; reverting just one
half restores a failure demonstrated by the regression.

`TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup` copies the real wrapper into a
clean fixture repository and substitutes compilation with a re-executed helper. The helper
launcher waits for the real `main`, CLI preflight, worker pool, `buildOnce` and process-group
cleanup. A build helper records both its PID and its child PID over a Unix socket, proves the
child is outside the wrapper group, and holds retirement after acknowledging cancellation.
The test sends SIGHUP to the wrapper group, checks that the wrapper and scratch remain while
the worker is held, then releases it and requires exit 129, absence of both recorded PIDs,
and scratch removal. The package retains its accepted unbounded-reader floor: existing tests
read Git metadata and repository-wide evidence; a wrapper-only read declaration would be false.
This exercises production cancellation and ownership without another cold cross-target build.

## Verification

Commands used `TMPDIR=/home/russell/lanes/archpar.tmp` (0700), the configured heavy semaphore,
no lock overrides and no identity exports during tests or checks. Go commands named one package.

- `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`: exit 0;
  `conformance/release-artifact-v0` selected by `DIRECT_SOURCE_CHANGE`, additive fallback
  `NON_GO_OR_UNINDEXED_PATH`. Retained scope UNKNOWN: 100 selected, 556 excluded, 79 unknown
  (78 LANGUAGE_FRONTIER, one UNOWNED_DIRTY_PATH). Installed binary reports `self-binary-stale`;
  this is its degraded selection, not current-tree selection qualification.
- Focused new regression initially PASS (0.097 s). With only the producer SIGHUP subscription
  removed it FAILS waiting for cancellation (10.140 s). With only the launcher's HUP ignore
  removed it FAILS because the wrapper exits before worker retirement (0.137 s). Both repaired
  inputs were restored before the final package run; original failure logs are retained.
- `HEAVY_WHOLE_REASON='V1-4733 affected DIRECT_SOURCE_CHANGE selects conformance/release-artifact-v0; owner requires existing whole touched-package tests' script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/release-artifact-v0`:
  PASS, 17.536 s package runtime, 17.9 s admitted run, no queue. All existing touched-package
  tests ran without a `-run` filter; no assertion was changed.
- `script/heavy.sh --admit-only make go-archive-gate-test`: PASS, 2.1 s, no queue;
  existing private setup, cleanup, failure, interruption, witness and closed-set checks.
- `script/check-line-citations.sh --repin`: no unchanged content moved; legacy basename skips
  retained. `make -s spec-requirements-check line-citations-check`: PASS; specindex whole
  0.853 s, focused lrfrepo 0.070 s, both through heavy.sh. Generated indexes unchanged.
- `GOTOOLCHAIN=local go env GOVERSION`: `go1.27.3`.

The existing pre-change capture is reused. One final CEM finish cycle will bind this complete
repair, rebind post-commit and run dogfood-check, stopping without seal. Exact final head and
check results belong in the external handback. The orchestrator owns independent review;
no model-backed reviewer or task-write command ran here. Repository-wide gates, doc-gates,
selection baseline, broad selections, Windows/Darwin execution, live cold archive rerun and
unrelated frozen evaluations are NOT_RUN under the repair boundary. Prior performance evidence
keeps its original revision; this repair makes no new speed claim. Publication, native Tasks
completion and seal are NOT_PRODUCED by instruction. Optional guarding-test runner receipts
and external-user outcome validation remain NOT_PRODUCED / NOT_OBSERVED.

## Retained evidence

Raw logs stay outside the repository under `/home/russell/lanes/archpar-evidence/hangup-review/`.

| File | SHA-256 |
| --- | --- |
| `affected.json` | `732a3f4dc29e7cd5f1c30c3d2522df87e535957649145c52d4cc465bd15928c2` |
| `regression-main-before.log` | `d75134f8edaf76fd4f87662b97534d9e8bd6601953f7d361547c21c364f5b608` |
| `regression-wrapper-before.log` | `fbfbcb6cfcca8eb07f290d9383c10f2923aa169091b1117b1ed20daa74ca4f13` |
| `package-test.log` | `0fd98ef10ad3e3e1e164ec38c18a7d96051d066cd731d7554b52a7e497517517` |
| `wrapper-test.log` | `63ccc99d5deae26f0cd894de13d26112808815825933f46e5cd14313becf0f69` |
| `repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `static-check.log` | `9cbd4f4cc2e54fde426a6157ef623264f763146b5d6576db4c47bd552c2562eb` |

## Friction

1. Installed `corvint affected --base` emits `DEGRADED self-binary-stale` (binary built at
   `d436dff76de1`, 54 differing Go paths). Reproduce with the affected command above.
   Retained the degraded plan and tested only the required touched package; no wider selection
   or binary install was performed.
