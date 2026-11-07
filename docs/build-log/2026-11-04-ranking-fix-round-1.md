# Ranking fix round 1: shell conflict-marker coverage

Owner request: final lane review finding 1, V1-1323 / DCI-V0-7902.
Base: `39c03198086681fb24f509fbd00b34696d3268c4`.

The conflict-marker gate omitted `.zsh`, `.ksh` and `.bats`, although the affected
shell plugin owns them. Align its explicit shell suffixes with `shell.Suffixes`;
keep the gate independent of Go builds and preserve staged-byte authority,
diagnostics, exit codes and exact fixture exemptions. The owning requirement
now enumerates that existing shell coverage explicitly. Rollback removes the
three added pathspecs, restoring the reported blind spot.

`script/check-conflict-markers_test.sh` passed before adding the regression;
with the new cases and the old gate it failed on a staged opening marker in
`hooks/pre-commit.zsh`. After alignment the complete fixture suite passes,
including all three markers in `.sh`, `.bash`, `.zsh`, `.ksh` and `.bats` paths
under `hooks/`, with clean working bytes hiding the staged markers. Existing
path-family and exception assertions remain intact. Bash syntax, the staged
marker gate, line-citation checking and `make spec-requirements` pass; the
generator reports no generated changes (existing capacity warnings retained).

The base-range affected plan reports UNKNOWN: 139 selected, 82 unknowns
(78 language frontiers, four unowned dirty paths). No Go package is changed in
this repair; repository-wide Go tests and fresh ranking evaluations are not run.
Independent review belongs to the orchestrator. Pre-change receipts from the
original lane remain observed and bound; this repair does not claim a new
pre-change baseline. Binding/check evidence and command exit codes are retained
outside the repository in `ranking-evidence/fix1-*` and the lane handback.
