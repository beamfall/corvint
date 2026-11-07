# Bugfix verifier repair: installed dependency scan boundary

The owner requested only the doc-gates failure at
`ee764cd8bdafa8e54aabded7998b3569e9542bea`, against lane base
`cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`. The original GOC-V0-008 scan
recursed into the installed protected-Pi SDK and reported its third-party
`python-data-analysis` example. The isolated documentation gate reproduced the
host failure with exit 2 while the locked dependencies remained installed.

GOC-V0-008 governs maintained Corvint adapters and tools. The repair prunes only
directories whose basename is exactly `node_modules`, at every depth of the
integration target enumeration. It retains all maintained integration files
outside those installed dependency trees. This boundary avoids deleting the
prerequisites that the prior Pi repair needed. The interpreter matcher, existing
assertions, exemptions, diagnostic text, exit codes and frozen artifacts remain
unchanged. No runtime or wire contract changes.

The shell test now creates top-level and nested installed dependency fixtures,
adjacent owned `.mjs` and `.json` files, and `node_modules-extra/runner.mjs`.
It requires the exact owned target set and requires interpreter detection in
every selected fixture. Existing positive and negative scan assertions all run
through the same test entry point. The isolated gate passes after repair.

| Observation | Raw log SHA-256 |
| --- | --- |
| Original isolated gate, exit 2 | `7a438da34eec92021f052b7fb97a00b7391936eb73685dbd1096590efc530284` |
| Repaired isolated gate, exit 0 | `2f998c3ae8b2c63b411da71e13d59b993dc5e93c12aec665d41d2925ce905af1` |

Raw evidence stays under `/home/russell/lanes/bugfix-evidence/verifier1-*`.
Affected advice selected the shell test directly; scope remains UNKNOWN with
88 unknowns and the installed binary's self-binary-stale warning. No Go package
is changed in this repair. The existing pre-change capture is reused with its
previously recorded base-tree mismatch, not recaptured or relabelled. Ranking,
learning, native task completion and broader ticket qualification are outside
this repair. The orchestrator owns independent review. Final gate and binding
results are retained in `/home/russell/lanes/bugfix.handback.md`.

Rollback reverts the target enumeration repair and its regression fixtures,
then rebinds the CEM; installed SDK examples again trigger the original gate.
No dependency lock, committed golden or existing test assertion changes.
