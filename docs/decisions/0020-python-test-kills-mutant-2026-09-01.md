# Decision 0020 — `test-kills-mutant` for Python

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "do all
three in parallel" (2026-09-01), in reply to the recommendation "`test-kills-mutant` for Python.
The bench corpora where change-aware retrieval reaches gold are mostly Python (transformers,
fastapi, scrapy), and the mutation runner is Go-only, so the packet's strongest falsifier never
fires on the tasks the trial will mostly contain. A Python runner (AST mutants confined to the
hunk, sandboxed `pytest -k`) is the same shape as the Go one."

## Scope

The instruction accepts `FPK-V0-019` in `docs/specs/falsifiable-packet-v0.md`: a test row whose
path is a pytest file (`test_*.py`, `*_test.py`) claiming a `.py` source path gets
`test-kills-mutant`, and under `--mutate` the runner in `internal/liveverify/pymutate` judges it on
the same exported copy, sandbox, budgets, drift bracket, and hunk confinement the Go runner uses.
Mutants are one-line token rewrites produced in Go over the repository's Python lexer
(`negate-condition`, `swap-binary`, `replace-literal`, `delete-statement`); Python is run only to
confirm a mutant parses and to run the cited test functions under
`python3 -m pytest -q -x --no-header -p no:cacheprovider T -k "<names>"`. A mutant that does not
parse or collect, or whose run the per-run timeout ends, is never credited: pytest has no timeout
of its own, so the runner's kill is not the test's verdict. Without `python3` or an importable
pytest the row is `NOT_RUN` with the reason, as a Go row is without its module dependencies.

The Go runner is not edited: `internal/liveverify/mutate/shared.go` adds exported accessors and a
`Run` method on `Export` so the Python arm reaches the export directory, scratch, sandbox prefix,
bounded output, and process-group kill through one seam, with no copy of the sandbox profiles.

## Explicit exclusions

No Python AST in Go: the planner is a token rewrite over `internal/pythongrammar`, so a
`delete-statement` never removes a bare-name binding and `replace-literal` touches only `return`
statements, the conservative shapes the lexer can vouch for. No `pytest-timeout` dependency, no
package installation, no virtual environment discovery: dependencies absent offline make the row
`NOT_RUN`. No widening of impact's same-package test rows, which are emitted for `.go` changed
paths only (`internal/contextindex/impact.go`), and no change to change mode's range impact,
which still requires a slash-qualified Go module (`internal/contextindex/range_impact.go`); both
mean a Python claim reaches this runner today only through `prove --base` in a repository that
also holds a Go module, and both are open for the owner. No runner for Rust or any other
language; no promotion, cutover, merging, tagging, signing, publication, or release gate.

## Consequences

`falsifierFor` admits pytest paths, `affectedRow` asks `mutationClaim` whether the test and the
changed path share a language, `judgeMutation` dispatches on the changed path's suffix, and
change mode reads hunks for `*.py` beside `*.go`. `TestMutationVerdictVocabulary` and
`TestAffectedRowMakesNoMutationClaimForAChangedTest` now expect `test-kills-mutant` for a Python
test row; every other falsifier assignment is unchanged.
