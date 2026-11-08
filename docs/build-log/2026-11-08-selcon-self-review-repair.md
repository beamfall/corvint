# V1-5094: self-review repair — validate exclusions and FULL coverage

Requirements: AFP-V0-9500 and AFP-V0-9501. Base:
`9761ea1f459c0d3b3c963953e24a76eaeda16370`.

The review found two unsound certifications: malformed dirty paths could bypass
validation after conservative fallback and then become NOTHING through nested-module
coverage; two FULL labels could pass parity even when an unreadable command declaration
left the local manifest empty. Both repairs preserve existing wire shapes, errors,
exit codes, goldens and test assertions.

For AFP-V0-9500, validation now precedes every possible module downgrade, regardless
of conservative verdict. The closed input grammar is `fs.ValidPath` slash paths without
backslashes, the existing supported scopes, and the existing recognized nested-module
certificate grammar. Unknown certificate forms retain fallback. The regression
`TestQualifiedSelectionModuleCoverageFailsClosed_AFPV09500` checks the reported
`nested//x.go`, a parent element, an unknown future certificate, and a valid positive
control through both direct and indexed callers. This repairs the bypass class rather
than enumerating malformed path spellings.

For AFP-V0-9501, FULL has no scope-only success branch. The replay independently lists
Go packages from the checked-out root using `go list -mod=readonly ./...` plus explicit
hidden root packages discovered by the existing bounded source walk. Nested modules
are outside this root universe. A failed listing or bounded hidden-source walk cannot
certify coverage. The allowlist is a nonempty observed concrete package set matched
exactly by explicit `go:<package>` manifest units; neither a FULL label, an unknown
unit form nor a command-shaped wildcard is package evidence. Other adapters and nested
module suites keep their declared profile differences.

Design decision: independently enumerate packages instead of interpreting declared
shell commands as equivalent full runs. This keeps unknown commands unproven and
prevents an unreadable declaration from supplying its own coverage proof. Hidden root
packages must be included because the existing runner explicitly executes them.
`TestSelectionContractFullCoverage_AFPV09501` covers complete, empty, truncated,
unknown-form, wildcard and unobserved-universe cases, hidden-package omissions,
nested-module exclusion, and a failed Go listing. Existing parity assertions remain.

The lane's affected receipt selected both touched packages. Whole-package tests ran
through `script/heavy.sh` with no test filter: `internal/gateselect` passed in 2.737s;
`tools/selection-baseline` passed in 2.000s after hidden-universe coverage was added.
The initial baseline package run also passed before that additional coverage; its
result does not stand in for the final run. Raw evidence remains outside the repository
in `/home/russell/lanes/selcon-evidence/`. Documentation checks and final CEM/check
results are retained in the lane handback.

| Evidence file | SHA-256 |
| --- | --- |
| `self-review-test-gateselect.log` | `a7a54144165f5f82656c4ad465826a7f3323d96681cc6afd35f6b6c2ed3dfe27` |
| `self-review-test-baseline-final.log` | `9908ee5b73abe87b6ee0f69685fe32ea80209a8f553346a5badb7d8568707694` |
| `self-review-affected.json` | `db95b8f94cb798f4e4e73a1978c1c2f3fbff874ee7fc8b998485789fd3ff47be` |

 No performance or corpus-parity claim follows
from these fixtures.

Self-use: reused the existing pre-change capture and ran affected; final CEM rebind/check
is required once the final edits are committed. Corpus replay, whole-repository gates,
independent review and separate CI control-plane integration remain with the host and
orchestrator under the owner's explicit split. Prior failed cmd/corvint evidence remains
visible. No seal, push, PR or Tasks write is authorized in this round.

Rollback reverts the two repairs and their new regressions together; the parity checker
then cannot certify FULL coverage and the nested-module downgrade must remain unqualified.
