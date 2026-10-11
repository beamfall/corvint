# V1-0452: OpenCode shipped-file version checks

Base: `32bfc85a44fd82b5793c7e30b2231200e16c025a` (`origin/main` at task start).

## Contract and implementation

The reported gap was present: OpenCode's package manifest lists `mcp.example.json` and
`skills`, while the version gate's explicit shipped paths omitted both. The gate now includes
both paths; Git's directory pathspec covers new nested skills. The resulting explicit list
matches all five OpenCode manifest entries; Pi's existing list already covers all twelve.
No package content, version manifest, decision file or task-store data changes.

AHI-020 retains its ID and adds only the named-path clarification, marked
**accepted, decision 0514**. Its traceability row names the regression
coverage. No new requirement ID is allocated. Broader manifest parsing is outside this repair;
future published-list edits must keep the explicit gate paths aligned. Rollback is to revert
this patch, restoring the known missed-bump gap.

## Verification and review

- The extended fixture suite against the original gate failed with
  `FAIL: OpenCode skills/added/SKILL.md shipped without a bump` after the existing cases passed.
  An earlier baseline run overlapped a test-file edit and was invalidated; it is not passing
  baseline evidence.
- Regression coverage adds skills-only and MCP-example-only changes, package-specific failure
  diagnostics, successful subsequent bumps, and an unpublished-file negative control.
- `sh script/check-host-package-versions_test.sh`: all 14 cases passed.
- `make -s spec-requirements` regenerated the TSV after staging the spec in the disposable
  checkout. All requested gates passed there: `spec-requirements-check`,
  `requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
  `line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
  `use-case-receipts-check`, and `use-case-receipts-test` (five cases).
  The separate `sh script/repin-use-case-receipts.sh --check` passed in the original worktree
  with all 18 receipts current. Shell syntax and `git diff --check` also passed.
- Independent read-only review found no actionable issues in the explicit lists, regression
  assertions or AHI-020 clarification. No concurrency code changes; race testing is inapplicable.
- Tests use `GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`.
  No repository-wide gate or `./...` run is claimed.

## Context and limits

Corvint query and affected were used against the base. The query retained three omitted results
and 23 withheld test candidates; affected returned `PLAN_ONLY`, `EMPTY_SELECTION` for Go, and
language-frontier unknowns. Original outputs are local scratch evidence at
`/private/tmp/v1-0452-query.json` and `/private/tmp/v1-0452-affected.json`.
Non-Go impact and retrieval-learning evaluations are not applicable to this shell-path repair.
The owner's explicit focused-check scope overrides affected's static full-gate suggestions.

Dogfood change was attempted but refused when creating evidence in the sandbox-protected Git
metadata directory. CEM/OCM/seal are `NOT_PRODUCED`; post-commit binding is also excluded by the
owner's no-commit instruction. Staging in the original worktree was similarly refused; a
disposable checkout is used for staging, generation and index-based documentation checks.
Its four modified tracked files were byte-compared with the delivered worktree. Initial
documentation checks failed because this macOS `mktemp` used an unwritable default directory
even with `TMPDIR` set. A scratch PATH wrapper supplies `${TMPDIR}/tmp.XXXXXX` only for
argument-free `mktemp` calls; all other calls pass through to `/usr/bin/mktemp` unchanged.
No repository tooling was modified for that workaround. Gate output is retained at
`/private/tmp/v1-0452-doc-gates.log`; focused output is `/private/tmp/v1-0452-green.log`.
Ticket completion, publication and commits are intentionally not performed.

Installed tools report Core `1.0.0-rc.3` build 407 and Tasks
`0.0.0-tcp01-unverified+build.407`. Official release metadata lists Core `v1.0.0-rc.2` and
Tasks `tasks-dev-20260929.2`; the updater's checks failed with
`process-after-start-failed: descendant snapshot unavailable`. Freshness remains `UNKNOWN`;
no downgrade, installation or qualification is claimed. Account usage telemetry was unavailable.
Tooling-friction filing is deferred because this task explicitly prohibits task-store writes.
