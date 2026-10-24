# p1-skh-b: site knowledge stages 3-5 — #1705 independent-review fixes

Date: 2026-10-24. Lane: local p1-skh-b (V1-1900..V1-1902 follow-up, stacked on #1705).
Spec: `docs/specs/site-knowledge-howto-v0.md`. Platform for every result: linux/WSL2 (go1.27.3).

## Owner authority

The Codex review of #1705 found three P2 defects. The orchestrator routed them to this lane;
#1705 itself is batched unchanged. New requirements `SKH-V0-140`..`142` come from this lane's
reserved block `SKH-V0-140`..`179` and are experimental refinements of accepted requirements.

## Decisions

- **Outcome identity keeps the name (SKH-V0-140).** An outcome's locator was `kind=value`, so
  `getByRole('button', {name: 'Success'})` and `{name: 'Failure'}` intersected into one shared
  `role=button` outcome, and the exported intent asserted only the role. The locator now appends
  ` name=NAME`, the same form `skActionLabel` already used for steps. A candidate whose variations
  assert different names has no shared outcome (`outcome_status` `UNKNOWN`). No reader parses the
  string back, so no profile version changes; an emitted request's outcome text is longer for
  named locators, which is the intended stronger assertion.
- **A helper must be the whole argument (SKH-V0-141).** `rubyTarget` accepted any rest starting
  with `(`, so `redirect_to(order_path(order) + '/edit')` resolved to the order screen. The rest
  must now be one balanced argument list ending the argument (quotes skipped); anything else is
  `dynamic`. This only moves edges from resolved to `dynamic-target`, never the reverse.
- **Setup projects honour testDir; testIgnore abstains (SKH-V0-142).** File selection used the
  config's directory and `testMatch` only, so a `testDir: './auth'` project with `/.*\.ts$/`
  classified every spec under the config as a setup file and stopped arranging them. The reader
  now joins the project's literal `testDir`, else the config's top-level literal `testDir`, to the
  config's directory. It does not evaluate `testIgnore` globs or expressions; rather than select
  files such a filter may exclude, the record is `UNKNOWN` with the new reason
  `unsupported-test-filter`, as is a non-literal or repository-escaping `testDir`. The
  dependent-project arrangement scope (config directory) is unchanged.

## Evidence

Each test failed first against `1e8651c7a`, then passed: `TestSKHV0_140_OutcomeKeepsSelectorName`,
`TestSKHV0_141_RubyHelperConsumesWholeArgument`, `TestSKHV0_142_SetupProjectHonorsTestDir`.
`go test ./internal/appmap/` ok (linux/WSL2).

## Rollback

Revert the commit. No artifact, profile or store encoding changes.
