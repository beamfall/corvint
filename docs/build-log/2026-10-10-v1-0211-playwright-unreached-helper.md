# V1-0211: Playwright unreached helper selection boundary

Intent: proposed, pending owner acceptance (V1-0211).
Base: `32bfc85a44fd82b5793c7e30b2231200e16c025a` (HEAD and local origin/main).

The existing shared selector reports `NO_SELECTABLE_TEST` for a changed non-Go
unit with no test in its reverse dependency closure. Playwright deliberately
ignores that reason in `graphFrontierUnresolved`, but its owning spec had not
decided how to describe the result. This was not already delivered as a contract.

The smallest option amends TJAA-V0-013 and AFP-V0-020: the separate Playwright
profile explicitly excludes an unreached helper from selection. With matched
discovery and otherwise resolved reachability, it retains `BOUNDED`, `NONE`
fallback, and no selection for that helper. A full E2E suite cannot exercise it.
This is not evidence that the helper is tested. Other selection and execution
unknowns, discovery fallback, reached-helper selection and project relations
remain unchanged. No runtime behavior, wire field or new requirement ID is added.
Rollback removes the proposed clarification and regression; runtime stays as it was.

`TestPlaywrightUnreachedHelperBoundedExclusion_V1_0211` exercises the public
discovery-reconciled API against an independently enumerated fixture. Unreached,
reached and mixed dirty sets assert exact selections, matched discovery, bounded
scope, no fallback, exclusion counts, and retained execution uncertainty.

Verification uses Go 1.27.1 with `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`:

- Baseline: the existing project-selection, discovery-reconciliation and
  dynamic-import fallback tests passed.
- Focused `TestPlaywright` tests passed; shared `TestSelect` tests passed.
- No concurrency changed; race instrumentation is not applicable to this patch.
- Documentation gates passed against the rebuilt temporary index: spec
  requirements, requirement definitions, traceability tests, decision numbers,
  line citations, error-code ownership, unbounded readers and use-case receipts.
  Receipt fixture tests passed all five cases separately without inherited Git
  index settings. Line-citations-check was repeated successfully after staging
  generated REQUIREMENTS.tsv in the temporary index, removing its unstaged-file
  warning. The separate `sh script/repin-use-case-receipts.sh --check` passed
  (all 18 receipts current).
  `git diff --check` passed. Final check output is retained in
  `/private/tmp/v1-0211-doc-final.log`, `/private/tmp/v1-0211-receipt-test-final.log`
  `/private/tmp/v1-0211-line-citations-final.log` and `/private/tmp/v1-0211-repin.log`.
- Independent native review found no issues in the two specs, comment and test.
  The inherited model/effort was retained for this bounded contract review;
  account usage telemetry was unavailable, and no cost/savings claim is made.

Corvint context and affected routes were used; original output is retained in
`/private/tmp/v1-0211-query.json` and `/private/tmp/v1-0211-affected-final.json`.
The query reported four omitted results and an index revision different from
HEAD; original requested source/spec evidence was read directly. The affected
plan was `UNKNOWN`, retaining language frontiers and unowned spec paths.
Full gate and repository-wide tests were explicitly excluded by the task.
New IDs were unnecessary; local origin/main and the open PR inventory were read.

Installed Core is `1.0.0-rc.3` build 407; Tasks is
`0.0.0-tcp01-unverified+build.407`. Official release inventory lists Core
`v1.0.0-rc.2` and Tasks `tasks-dev-20260929.2`; no downgrade was attempted.
Both updater checks failed their version smoke with
`process-after-start-failed: descendant snapshot unavailable`; freshness remains
unknown and no update or verification bypass occurred.

Normal Git staging and `make dogfood-change` cannot write this worktree's Git
metadata outside the sandbox. The generator therefore uses a private temporary
Git index/object directory, with changed specs staged there before
`make -s spec-requirements`. The new test is staged there for traceability checks.
macOS `mktemp` ignored the requested temp directory; the check environment uses
installed GNU `mktemp` through a temporary PATH entry. Initial generator and
traceability failures are retained in `/private/tmp/v1-0211-doc-gates.log`.
The receipt fixture test rewrote the first temporary index through inherited
Git settings; the real index was untouched. Final gates use a rebuilt index,
and the receipt fixture runs separately without those settings.
Post-commit CEM/OCM/seal are NOT_PRODUCED: commits are expressly prohibited.
No ticket-store writes, publication, decisions or plugin-manifest edits occurred.
