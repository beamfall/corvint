# OpenCode lifecycle receipt version binding

Issue: https://github.com/beamfall/corvint/issues/314

The OpenCode 2 package and compatibility row identified adapter 0.3.0 while the runtime sent
0.1.0. The transport fixture also hardcoded 0.1.0, so its assertion preserved the mismatch.
AHI-010 requires exact package identity and AHI-020 requires a new version for shipped changes.
This repair uses 0.3.1 in the package, declaration, matrix and runtime. The fixture now reflects
its actual adapter-version argument; the transport regression compares argv and receipt identity
to the package and matrix instead of a stale literal.

The regression failed before the runtime repair (actual 0.1.0, expected 0.3.0) and passed after it.
Terminal verification selects TestHostAdapterJavaScriptHosts and the package-version gate;
results and the independent review remain in the local dogfood evidence for the frozen target.
Repository-wide make gate is not selected under the scoped-issue policy.

Self-use: query and affected receipts were retained locally. Non-Go path impact is unavailable;
the fixture Go change and adapter suite determine focused verification. Initial dogfood-change
reported git-diff-failed at the unchanged base, absent intent/outcome inputs and missing private
prechange receipts; these were not success evidence. Authority-start query was captured before
implementation and then copied to the worktree-private receipt path.

This repair does not qualify an OpenCode host tuple or admit an execution authority. FULL support
is a separate owner-requested workstream. Rollback restores the earlier runtime and test behavior
and requires another package version if shipped.
