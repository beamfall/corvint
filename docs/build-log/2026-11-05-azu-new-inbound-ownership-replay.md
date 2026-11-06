# Inbound caller closure, ownership completion and frozen replay

Scope: V1-4687 / AZU-V0-004, V1-4692 / AZU-V0-009 and the remaining V1-4685 /
AZU-V0-002 work. Base is `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`. The initial AZU spec
commit is inherited. The available driver prerequisite was integrated from
`claude/azu-driver-claimant`; the requested `claude/azu-driver` ref and matching exported handbacks
were absent. Its implementation record, original diff and audit baseline supplied the evidence.
Sibling site/small diffs and handbacks were read; their worktrees were not changed.

## Decisions and limits

Remaining data gets verification owners through the existing test-selection contract. This only
adds tests and keeps all existing reader/executor/import/embed selections and frontier bearing.
The audit baseline is reduced to empty, rather than ratcheting another accepted residual. Exact
parser control declarations count as ownership in the audit; a declaration's existing selection
uncertainty stays unchanged. This distinction preserves frozen assertions for named declarations.
There is no wildcard ownership of arbitrary new `.corvint` data. The existing replay's expected
unowned residual becomes empty, with an additional assertion that both paths have selected owners.
No existing assertion or golden was removed or weakened.

Constructed foreign references widen the completeness guard through quoted directory
concatenation, path joins and formatted paths. The tracked-file gate uses the same matcher even
outside the plugin's walk. Caller declarations conservatively include executable-source references.
The literal `git rev-parse --show-toplevel` root query of a wrapper resolves its known root-relative
shell target; a query with `-C` elsewhere does not. Unresolved computed commands remain uncertain.

A located, declared end-to-end runtime site selects its tests and dependents before its unknown
closes, including a TypeScript code change. Declared-data-only changes do not create runtime code
selections. A changed shell declaration needs actual bounded base/head bytes. The union of old
and new scripts, callers and mentions seeds traversal; removed/unindexed callers and absent
historical evidence retain the changed-declaration unknown. Bare selectors keep the old contract.
An empty optional driver no longer changes the identity of a graph without driver inputs: this
repairs an observed frozen application-flow digest regression without changing its golden.

The replay uses exact historical trees in external private clones, supplied binary digests and a
fixed initial list of eight merges plus later local first-parent merges. It retains each plan,
unknown entries, selected/test counts, time and named before/after deltas. Full-suite and mutation
receipts are external evidence; their revision/inventory/changed-line bindings and inclusion checks
are not proof that an external runner actually ran or that its mutation population is adequate.
The same checker rejects deliberate outcome-test and mutant-killer selection drops. Missing or
invalid historical receipts never become an acceptance claim.

## Verification and failures

Detailed commands and raw outputs are outside this repository under
`/home/russell/lanes/azu-new-evidence/`. The final lane handback carries the command/result index.
The first whole affected-package run failed on control declaration audit ownership, a missing
literal-glob witness check, current ownership replacing old replay residuals, the preserved
computed wrapper frontier, and runtime selection on declared data. Those findings drove the
bounded repairs above. Its failed output remains retained; it is not counted as passing evidence.
The first CLI package run also failed a frozen flow digest and many host qualification checks.

The mandated scratch directory is beneath the outer `/home/russell/lanes` Git worktree. Tests
expecting a non-repository directory discover that parent instead. Executable qualification also
correctly refuses its mode `drwxrwxr-x` (group-writable without sticky). No host directory mode or
product safety check was changed. Git discovery ceilings are measured separately, without claiming
that they fix the unsafe-parent qualification refusal. The baseline archive build initially could
not obtain VCS status; rebuilding that exact archive with `-buildvcs=false` succeeded.

## Self-use and remaining qualification

Actual context, query, impact and affected outputs were retained before implementation. The
installed binary identified itself as stale; this is recorded uncertainty, not current-source
qualification. The required starting dogfood-change ran with the prescribed identity and base;
its missing citation/intent/verification inputs and absent bound agent receipts remain visible.
Binding/check evidence is produced at the terminal committed target. Learning, host adapters,
external interoperability and unrelated feature families are not applicable to this slice.

The orchestrator owns independent review, aggregate sibling integration, historical full-suite /
mutation receipts and native ticket completion. No Tasks write, push, PR, model-backed reviewer or
seal is part of this lane. AZU-V0-009 is not accepted while any historical plan is unknown or its
safety cell is unobserved. An empty ownership audit alone does not establish zero selection unknowns.

The final caller-declaration fixed point contains 35 scripts, within the existing 64-script and
64 KiB limits. Literal and directory-construction completeness is unchanged by classifying proven
non-executing source references as mentions. In particular, the citation checker only reruns
itself, the use-case merge driver runs Git, the spec acceptance audit executes the requirements
generator, and the named Go citation classifiers read or print paths. Treating those references
as executions had manufactured reverse edges into most Go commands; the retained failed runs show
that widening. Actual script invocations and script-copying tests remain callers.

The repository Go/shell/CEM inbound probes pass after that classification (`inbound-terminal-probe.log`:
whole selected probe, 8.639 s test runtime). The completeness matcher now scans each candidate once
per callee directory; the tracked-file gate reads each candidate once as well. This is a bound on
repeated work, not a cross-host latency claim. A zero-byte audit baseline was refused by CEM as
`metadata-only-change-unsupported`; one LF represents the same empty list and supplies a textual
hunk. The unsafe parent qualification and outer-worktree discovery failures remain external limits;
a Git ceiling did not change the product's hermetic discovery behavior.
