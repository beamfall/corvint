# 2026-10-21 idx-1022c: located Ruby frontiers (V1-2363, GH #1519)

## Problem

On a large Rails application, `corvint affected` returned `UNKNOWN` for every change, naming the same
five bare Ruby frontier reasons. Every reason was plugin-wide and had no path or line, so a caller could
not tell whether the change reached the site, fix the site, or declare it resolved.

## Decisions

- Every Ruby frontier entry names its site as `reason:path:line`, or `reason:path` when only the file is
  known. One frontier names at most 16 sites per reason, then `reason:+N` (RUBY-AFFECTED-V0-046).
- Per-unit sites (conditional load of a resolved target, unresolved `require_relative`, possible
  generated test, ambiguous framework) become that unit's `frontier` member. The AFP-V0-004 unit
  frontier rule names them only when the plan reaches the unit (RUBY-AFFECTED-V0-047).
- Sites that can hide an edge to any changed file stay plugin-wide: computed loads, `$LOAD_PATH`,
  unread autoload configuration, dynamic constant lookups, unmapped associations, edgeless tests,
  edge overflow, unreadable files and Cucumber features. Taking the issue's closure text literally for
  these sites would have turned the `dynamic-constantize` and `dynamic-const-get` negative conformance
  rows into `BOUNDED`, inventing certainty (product invariant 2). They now stay `UNKNOWN`, naming the
  located site.
- A literal load or counterpart with several candidates gets an edge to each, a sound superset.
  `ruby:ambiguous-load` is no longer raised.
- `.corvint/ruby-frontiers.json` (`ruby-frontiers/0`, strict) resolves named sites, either by exact
  line or by reason and path. An invalid declaration resolves nothing and raises the plugin-wide
  `ruby:frontier-declaration-invalid` (RUBY-AFFECTED-V0-048).
- `internal/rubyautoload` records the file and line of each unresolved configuration line. Its report
  and the impact output do not change.
- Requirements RUBY-AFFECTED-V0-046..048 and the AFP-V0-004 and RUBY-AFFECTED-V0-005 amendments are
  accepted under the owner's standing auto-accept (2026-10-11).

## Evidence

- `TestLocatedFrontierCountsOnlyCrossedSites_RUBYAFFECTEDV0047`,
  `TestRepositoryWideLoadSitesStayPluginWideAndLocated_RUBYAFFECTEDV0046` and
  `TestFrontierDeclarationResolvesNamedSites_RUBYAFFECTEDV0048` pass. All three fail with the base
  implementation (bare reasons, every change `UNKNOWN`).
- The ruby, rubyautoload and rubyprofile packages pass. The conformance rows now carry located details
  for the two negative classes.
- A probe on this repository left only the two nested conformance applications' `config/application.rb`
  as plugin-wide Ruby entries.

## NOT_RUN / limits

- The repository-wide site count (issue Expected 3, ticket AC3) is not delivered. The CLI receipt wire
  is closed (AFP-V0-003). Plugin-wide `reason:+N` entries count only the hidden-reach sites. This is a
  residual for a wire amendment, so the PR does not close #1519.
- A Rails application with any `constantize` stays `UNKNOWN` until each named site is declared.
- A declaration is unverified caller authority. A declaration keyed to a line goes stale after edits
  above that line.
- No live run on the reporter's Rails application (external repository).

## Rollback

Revert the change. Bare plugin-wide reasons return, `ruby:ambiguous-load` is raised again, and
`.corvint/ruby-frontiers.json` is ignored.
