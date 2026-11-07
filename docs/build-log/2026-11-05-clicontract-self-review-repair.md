# clicontract self-review repair

Owner intent: fix only the four self-review findings against
`d436dff76de1b8e75c427069ae7f80003ae99bc2`. Preserve native wire bytes,
statuses, goldens and native compatibility assertions. Independent review,
integration and ticket completion remain with the orchestrator; no seal or
publication is authorized.

## Decisions and delivered repair

- V1-3736, ACP-011 / ACX-V0-7707: analyzer help calls the help answerer without
  the unknown-verb guard. Unsupported argv and malformed help remain unhandled
  with untouched streams, so native dispatch emits its original protocol frame.
  Process coverage pins exact stdout, empty stderr and exit 0 for Go and the
  four isolated native analyzers, including unknown verbs, malformed help and
  extra version arguments. Supported generated help remains covered.
- V1-2685, CAP-V0-7700: generated-help conformance applies to the retained
  requirements. The repair request explicitly directs this scope correction;
  no generated-help assertion body or existing native compatibility assertion
  is weakened or deleted. Separate process controls assert the excluded native
  analyzers' exact rejection frames and the four shims' exact help bytes,
  statuses and stream ownership. This supersedes the earlier spec instruction
  to retain a knowingly contradictory universal conformance claim. The owning
  requirement now records the split; universal help remains withdrawn.
- V1-2685, CAP-V0-7700: normalize only the four shims' exact `help --tree`
  request to their existing `help --json` path. Conformance checks both forms
  against the unchanged tree snapshots. Native plain help stays byte-identical.
- V1-3736, QAT-V0-001/004: remove the withdrawn alternative-rendering instructions
  from DOGFOOD.md. Only the original exact envelope plus one LF qualifies;
  checker behavior and frozen contracts are unchanged.

Rollback reverts this repair commit, leaving the original native entrypoints,
protocol implementations and committed goldens untouched.

## Evidence and limits

Raw evidence is outside the repository at
`/home/russell/lanes/clicontract-evidence/self-review-repair/`.
Whole-package runs through script/heavy.sh pass: internal/cliapp (0.004s)
and conformance/cli-errors-v0 (53.785s). The latter includes the original
native refusal suite, retained generated-help requirements, both shim catalog
forms and the added exact native process controls. No -run filter was used.

| Evidence | SHA-256 |
|---|---|
| cliapp.log | 15a8a0667d87f05baa894e1241e050d59535fe5b15efa52169645051a9eaa3fb |
| cli-errors.log | 20ccf57411e9d165d6f199e9b36f215172dbca3491a8838b61e11b8dda53f717 |
| affected.json | c6b3a74ed9a7c8d76d35e5a27aaf39c867708ec586adda946bc6d382b653f668 |
| context.json | 27b303e85cb0a79dc253d56d2aa51509bf33a5004761735371cc79cb1d1ac8f0 |

Affected scope remains UNKNOWN (257 selected, 90 unknown); installed-binary
staleness is retained. The context result omitted six candidates and names
missing critical sources; original task-named evidence was inspected directly.
The start dogfood pass collected query/impact receipts but refused missing
citation, intent and outcome inputs as expected. Final binding/check evidence
is retained in the handback. Earlier Core/environment failures remain failures;
scoped success does not upgrade the lane's aggregate outcome. Independent
review and host-only reruns are NOT_RUN here. Ranking/learning and external
provider qualification routes are inapplicable to this help/compatibility repair.
