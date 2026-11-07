<!-- SPDX-License-Identifier: Apache-2.0 -->
# sbgate verifier repair 1

The host verifier rejected head `001656470a5fe9441b5aae246b9e5b228351e9bd`
at `test-read-scopes-check`: `tools/selection-baseline/inputs_test.go` named
`cmd/corvint/main.go`, `tools/gate-affected-select/main.go` and
`.corvint/test-read-scopes.json` outside its declared reads. The package's
existing parent-directory locator makes those literals root-relative to the
static checker (AFP-V0-021/023).

Inspection of every occurrence confirms these paths are written or compared
inside temporary fixture repositories rooted at `t.TempDir()`. Apply the existing
ASD-V0-895 `//corvint:fixture-path` annotation to all nine occurrences. This keeps
the real read scope narrow and preserves ASD-V0-944 regression assertions,
fixture bytes and production behavior. No checker exception, wire contract,
golden or test assertion changes. Rollback is removal of these annotations;
the original static failure would return.

Verification against base `c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514`:

- `corvint affected --base` selected `tools/selection-baseline`; retained plan
  scope is UNKNOWN with two LANGUAGE_FRONTIER entries. Wider selection is
  deferred to the host verifier under the repair charter.
- `TMPDIR=/home/russell/lanes/sbgate.tmp make -s test-read-scopes-check`: PASS.
- With Git identity variables unset, `script/heavy.sh` admitted
  `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/selection-baseline`:
  PASS, 1.196 seconds, entire existing and added package tests, no filter.
- Toolchain: `go1.27.3`.

Raw evidence stays in `/home/russell/lanes/sbgate-evidence/verifier-repair-1/`.
`package-tests.log` records the complete package result; `test-read-scopes.log`
is empty because the successful check is silent. The original host failure is
retained in `/home/russell/lanes/sbgate.verify.log`.

Pre-change context is reused; no new pre-change capture. Existing missing impact
and guarding-test evidence remain NOT_PRODUCED. Independent review, broader
doc-gates, baseline replay and ticket completion remain orchestrator-owned.
V1-4724 remains implemented to the lane charter; V1-4329 remains PARTIAL for its
previously recorded external full-closeout qualification blocker. No seal,
publication or Tasks writes are authorized in this round.
