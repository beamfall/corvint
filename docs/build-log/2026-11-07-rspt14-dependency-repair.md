# Linux failed-operation and implicit-execution dependencies

Date: 2026-11-07
Scope: the two self-review P1 findings in lane rspt14; RSP-V0-021, RSP-V0-022 and
RSP-V0-012(f). No public wire, error code, golden, selector rule or qualification changed.

The failed-operation repair closes errno-based dependency loss: only ENOENT is lookup-only;
ENOTDIR keeps the existing-path read as well as lookup evidence, and every unknown errno retains
the read claim. The capture-to-selection regression uses a static native executable with no
startup reads to mask the defect, verifies exclusion for an unrelated edit, then requires rule f
for deletion and rename. Actual deletion changes the executable's errno assertion outcome.

The execution repair separates outside path classification from implicit-input inspection.
Every executable is inspected; only native ELF64/little-endian/x86-64, current version, EXEC or
DYN and System V/Linux ABI formats have a supported model. Other forms leave trace components
incomplete using the existing reason. A toolchain-root executable loads a repository-owned
interpreter; replacement changes its result from 0 to 7 and the interpreter remains a read.
A script and an unknown image format prove the fail-closed default outside the repository.

Validation: whole `internal/proctrace` and `internal/runnerselection` package suites passed through
`script/heavy.sh`, selected by `corvint affected` against
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`. Existing tests and assertions were unchanged.
The first proctrace run failed after an extra interpreter symlink traversal classified `/lib64`
as an undeclared outside read. That extra traversal was removed; the final whole suite passed.
Raw logs are outside Git in `/home/russell/lanes/rspt14-evidence/`.
The installed affected binary reported itself stale; its scope was UNKNOWN with retained unknowns.
No whole-repository validation or production qualification is claimed. Independent review and
host gates belong to the orchestrator under this repair charter. Rollback is reverting this repair;
it restores the two known dependency omissions and cannot authorize production narrowing.
