# Restore foreign-caller comment diagnostics — verifier repair 1

The host verifier at `f31de9d9427684ceb70b8fcd1e7993a718cd6fec` failed the
existing `TestForeignCallerCommentDiagnostic`: an undeclared comment mention
returned only the generic refusal, omitting the offending path and line.
TSP-V0-7302 (V1-3913) already requires that diagnostic. The lane's
AZU-V0-004 / AZU-V0-8901 (V1-4687) completeness split retained the diagnostic
for a missing script unit but dropped it when the original completeness guard
refused an otherwise present script.

Restore the existing mismatch diagnostic immediately before that original
guard's refusal return. Retain conservative uncertainty and the constructed
caller fallback. Newly detected constructed-only uncertainty follows the same
path as before this repair. No new diagnostic format, schema, error code,
exit status, golden, assertion or requirement is introduced. The accepted
specification and its existing named acceptance test remain unchanged.
Rollback removes only the restored diagnostic call; it would reintroduce the
reported failure.

Acceptance: the complete shell package, including the unchanged regression,
passed through `script/heavy.sh go test -count=1 -timeout 30m
./internal/liveverify/affected/shell` (0.853 seconds). The required affected
plan at base `8fc0cf37642e7012f4c26671c9ed97a8706d6e09` selected this package;
it retained UNKNOWN, 184 selected units and 86 unknowns, with stale-binary
advice degradation. No broader coverage is inferred. Shell log SHA-256:
`63eaff2590faf420690835aac94ecbd4c12eae658df35fe7f93daf24c8a7509d`.

Evidence stays under
`/home/russell/lanes/azu-new-evidence/verifier-repair-1-comment/`;
final commands, binding results and limits are recorded in the external lane
handback. Existing pre-change capture is reused. Applicable self-development
routes used are affected selection and final CEM inspection/binding; a new
pre-change capture, model-backed review, broad gates, publication, Tasks writes
and sealing are excluded by the repair instructions. Independent review and
host verification remain orchestrator-owned. This repair does not upgrade the
lane's historical qualification or unresolved work.
