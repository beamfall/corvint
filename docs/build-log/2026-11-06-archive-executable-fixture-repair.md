# V1-4733: verifier repair 1 uses guarded executable fixture writes

Repair starts at `3df77835502045c6518793f7765799fb4eeffc3d`, against unchanged base
`9db7a94169821d9b28701eaa88866aee2fec1083`.

The host verifier's `test-exec-writes-check` rejected two direct executable writes in
`TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup`. Under `ASD-V0-929`, executable
test fixtures must use `execfile.WriteFile` to exclude concurrent forks while their write
descriptors are open. The hangup regression for `GAG-V0-9962` now uses that existing helper
for both the copied wrapper and the Go shim. Contents, permissions and all assertions are
unchanged. This repair changes no production behavior or frozen contract. Rollback is to
revert this repair; the executable-write checker would reject the original writes again.

## Verification

Commands used `TMPDIR=/home/russell/lanes/archpar.tmp` (0700), with no identity exports
during tests or gates and no heavy semaphore overrides.

- `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`: exit 0;
  touched package `conformance/release-artifact-v0` selected by `DIRECT_SOURCE_CHANGE`.
  The installed binary reports `DEGRADED self-binary-stale`; retained plan scope UNKNOWN,
  100 selected and 79 unknown. This is not current-tree selection qualification.
- `script/heavy.sh --admit-only make -s test-exec-writes-check`: PASS, 0.4 s admitted run,
  no queue. Only the failing checker was rerun, not the enclosing doc-gates suite.
- `HEAVY_WHOLE_REASON='V1-4733 verifier repair 1: affected DIRECT_SOURCE_CHANGE selects conformance/release-artifact-v0; existing whole touched-package tests required' script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/release-artifact-v0`:
  PASS, 13.591 s package time, 14.2 s admitted run, no queue. All existing touched-package
  tests ran without a filter, including the group hangup regression. No assertion changed.
- `test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.3"`: PASS.

Line citation repinning/checking and the final CEM rebind/check follow these edits. Exact
commands, results and final head belong in the external handback. The existing pre-change
capture is reused. Independent review belongs to the host orchestrator. Whole-repository
checks, doc-gates, broad selections, Windows/Darwin qualification and cold archive reruns
are NOT_RUN under the repair boundary. Tasks writes/completion, publication and seal are
NOT_PRODUCED by instruction. No new performance or external-user outcome claim is made.

## Retained evidence

Raw evidence remains outside the repository in
`/home/russell/lanes/archpar-evidence/verifier-repair-1/`.

| File | SHA-256 |
| --- | --- |
| `affected.json` | `e57413763eb331fed0e750214aa62452afbd30c696ee738edc2541656a8bc1ed` |
| `package-test.log` | `d5dd4dfac6e8fa4575a566c7bbe8a270a972ec61599f1a241c488b4a49ebf5f2` |
| `test-exec-writes-check.log` | `98c6b5957ab91e566b71ed020376d8aec80cc4e5dff29f67aafb187fb83b49ea` |

## Friction

1. Installed selection binary is stale: reproduce with the affected command above;
   it reports binary revision `d436dff76de1` and 54 differing Go paths. The degraded
   plan was retained and only the touched package was tested.
