# V1-4733: SIGTERM joins archive workers before wrapper cleanup

Self-review finding P2; requirement `GAG-V0-9962`, preserving `GAG-V0-004` cleanup and signal
exit codes. Repair starts at `0cf45417265a0b35975dd9cbe213872efacccce4`, against unchanged base
`9db7a94169821d9b28701eaa88866aee2fec1083`.

The launcher previously ignored only SIGHUP. Group SIGTERM could kill `go run` while the
producer was still joining isolated loose builds, allowing the wrapper's EXIT cleanup to
remove live workers' scratch. The requested SIGTERM regression reproduces that ordering:
`wrapper exited before worker retirement: exit status 143` (0.170 s package time).

## Decision and compatibility

Keep the launcher in an isolated process group, owned by an awaited Bash job inside a
foreground subshell. That shell relays HUP, INT and TERM cancellation as SIGHUP to the
launcher group and waits for producer retirement before returning the original 129, 130 or
143 signal status. The launcher preserves its inherited SIGHUP ignore, while the producer's
existing explicit subscription cancels and joins its workers. The wrapper's original traps,
EXIT cleanup and ordinary delegated exit status remain unchanged. Signal delivery between
job creation and PID assignment is retained and relayed once the PID is recorded; repeated
signals are ignored while the handler waits. This uses Bash's existing job control without
another executable dependency or a detached job. INT forwarding preserves the existing
cancellation route when the launcher is moved out of the wrapper group.

Rejected measured alternative: inheriting a SIGTERM ignore does not protect a Go executable.
The pinned runtime installs its own SIGTERM handler at startup, unlike its treatment of
inherited SIGHUP/SIGINT ignores (`/usr/local/go/src/runtime/signal_unix.go`,
`sigInstallGoHandler`). The regression still failed (39.723 s whole-package run), and that
ignore also prevented the existing shell fixture from installing its TERM retirement trap.
That shell check was interrupted (exit 130); the candidate was removed before final checks.
Neither the test assertions nor fixtures were weakened to accommodate it.

`TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup` now runs SIGHUP and SIGTERM cases.
Both retain the original cancellation acknowledgment, blocked retirement, live scratch check,
recorded build/child PID absence after retirement, final scratch removal, and signal-specific
exit assertion (129 and 143). The helper still executes the real producer and worker cleanup.
The existing executable-write helper remains in place (`ASD-V0-929`). The owning spec and its
traceability row name both signals. No public JSON/wire shapes, field names, error codes,
diagnostics, goldens or core-freeze outputs change. Rollback: revert this repair as a unit;
the retained negative run demonstrates the cleanup race that would return.

## Verification

All commands used `TMPDIR=/home/russell/lanes/archpar.tmp` (0700), the configured heavy semaphore,
no lock overrides and no identity exports during tests or checks. Go tests named one package.

- `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`: exit 0;
  `conformance/release-artifact-v0` selected by `DIRECT_SOURCE_CHANGE`. Scope UNKNOWN:
  100 selected, 556 excluded, 79 unknown (78 LANGUAGE_FRONTIER, one UNOWNED_DIRTY_PATH).
  Installed binary reports `self-binary-stale` (54 differing Go paths); retained its degraded
  selection, without claiming current-tree selection qualification.
- Negative control: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup/SIGTERM ./conformance/release-artifact-v0`:
  FAIL before the wrapper change, as reported above.
- Repaired two-signal regression through heavy.sh: PASS, 0.389 s package time.
- `HEAVY_WHOLE_REASON='V1-4733 self-review repair: affected DIRECT_SOURCE_CHANGE selects conformance/release-artifact-v0; existing whole touched-package tests required' script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/release-artifact-v0`:
  PASS, 14.155 s package time, 14.4 s admitted run, no queue. All existing touched-package tests
  ran without a `-run` filter after the final code edits.
- `script/heavy.sh --admit-only make -s go-archive-gate-test test-exec-writes-check`:
  PASS, 2.5 s admitted run, no queue; existing setup, failure, interruption, cleanup, witness,
  closed-set and executable fixture checks remain satisfied.
- `script/check-line-citations.sh --repin`: no unchanged citation content moved; no second pass
  required. `make -s spec-requirements-check line-citations-check`: PASS, internal/specindex
  whole package 1.287 s and focused internal/lrfrepo 0.074 s through heavy.sh. Generated indexes
  unchanged; 180 legacy basename citation skips retained.
- Pinned local toolchain check: `go1.27.3`; `bash -n script/go-archive-gate` and diff checks PASS.

Pre-change capture is reused. One final CEM finish cycle will bind, commit, rebind post-commit
and run dogfood-check, stopping without seal; exact final revision and report limits are in the
external handback. Independent review belongs to the orchestrator. Repository-wide tests/vet,
Windows vet, doc-gates, selection baseline, corvint test, gate-affected, broad selections, cold
archive rerun, Darwin execution and unrelated frozen evaluations are NOT_RUN by the repair
boundary. No new performance claim. Remote publication, native Tasks writes/completion and seal
are NOT_PRODUCED by instruction; no new independent model-backed review was run here.

## Retained evidence

Raw evidence stays outside the repository in
`/home/russell/lanes/archpar-evidence/self-review-repair/`.

| File | SHA-256 |
| --- | --- |
| `affected.json` | `d9bf2c7125cb08c29eae308fada0909e1214a466006aaf73bdb4bb3ca8a66e6c` |
| `regression-before.log` | `c177fd15014a790192028f20e637da48de5104273f5bb191b219b9c261af8609` |
| `package-test.log` | `0fe6ec1c1367febe73918c85afc54c04d0d65d35f68531572b3271eefab30ba2` |
| `regression-after.log` | `802ef25e113cfcfed1fa9454be8853c527e638d3a76b3e648bd642ed3a7759f3` |
| `package-test-final.log` | `4796e24d98289bf77780eac6ce74a1a9986cf8c6ff4f553b7d415f9cf9e9c0eb` |
| `wrapper-check-final.log` | `57ad3c45ccf58f73e3b2f06e7a58c43eeb311465b9627dd2d7259b7b7cebaac5` |
| `repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `static-check.log` | `d3fdabc2308d21fcc44955cf893d6b50aa8530e8b109eda2c1d11493d25f8aaa` |

## Friction

1. Installed `corvint affected --base` reports `DEGRADED self-binary-stale`, built from
   `d436dff76de1` with 54 differing Go source paths. Reproduce with the affected command above.
   Retained the degraded plan; no install, broad selection or authority claim was substituted.
2. Inheriting TERM ignore is ineffective for Go and prevents shell fixtures from overriding
   TERM. Reproduce by replacing the relay with `(trap '' HUP TERM; exec go run ...)` and running
   the SIGTERM regression and `make go-archive-gate-test` through heavy.sh. The former fails
   early retirement; the latter waits for an ignored TERM. Final relay passes both unchanged.
