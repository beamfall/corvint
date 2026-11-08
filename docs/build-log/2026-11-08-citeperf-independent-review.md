# Citation runner environment equivalence

Independent review found that same-file executable identity did not establish
equivalence with `dogfoodflow.Exec`: the child forces `LC_ALL=C` and removes an
inherited stable reuse store, optionally substituting the context-owned store.
This repairs the invocation-equivalence class under `DCW-V0-8760` (V1-4851),
without changing the public CEM interface or verifier rules.

## Admission decision

Keep inline execution only for a successfully resolved same-file executable,
an absolute repository root, the exact cite/unknown-mark argv forms emitted by
the citation loops, and equality of the complete effective process and child
environments at each call. This includes the context-owned stable store and
future changes to the child environment contract. Unknown options, actions,
roots or environment differences fall back to the existing subprocess runner.
Do not temporarily set global environment variables: concurrent commands must
not observe a transient locale or reuse store. The admitted commands use
invocation-local parsing and a fresh CEM session, without changing cwd, os.Args,
global flags or introducing a citation-result cache. Other command forms keep
the subprocess isolation boundary. Rollback restores external execution for all
citation rows; frozen outputs and existing assertions remain intact.

## Evidence and limits

The existing multi-hunk byte-parity regression now includes `mark`, still removes
the selected executable to prove inline execution, and retains every failure
and private-probe assertion. A hostile-environment regression changes locale and
sets an inherited reuse store after runner construction, compares map/probe and
stdout/stderr/status bytes against Exec, and checks caller environment preservation.
An unknown-option regression proves external dispatch by the selected executable's
output. A separate environment regression refuses a context-introduced store even
when the inherited locale and environment would otherwise qualify.

Whole `internal/dogfoodflow` comparison before this repair: lane `2ecb68c3c`
failed five tests in 38.355 s; base `6339f980e` failed the same five in 38.414 s.
The matching signatures are aggregate quiet-proof/cleanup HOLD, regenerated
requirements merge-resolution-unbound, regenerated commits tree-differs, and
regenerated hunk citation-plan-map-mismatch. These are inherited failures on
this host, not justification to change assertions or regeneration behavior.
Final package results and raw-log digests are retained in the lane handback.

For V1-4851/V1-4852 measurement, the harness now writes a separate text summary
with per-mode/arm N, median, minimum, maximum, and median percentage reduction.
Failed or unequal arms suppress a savings claim. The raw JSON shape is unchanged.
Synthetic arithmetic checks establish calculation correctness only; they are
not acceptance timings. Same-host measurements and remaining qualification
limits belong in the handback alongside their run count and spread.

Independent review and ticket completion remain orchestrator-owned. No push,
PR, ticket mutation, seal or whole-repository gate is part of this repair.
