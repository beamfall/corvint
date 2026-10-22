# 2026-10-21 lane ghissues-1021q: Ruby autoload edges, one-argument help topics, corpus mismatch refusal

## Problem

- GitHub #1475 (V1-2306, P1): `corvint affected` excluded Ruby specs that reach a changed class
  only through autoloaded constants, with scope still BOUNDED. The Ruby plugin built autoload
  constant edges only for a Rails application (root `config/application.rb`). A Zeitwerk-style
  `app/` tree without it had no constant edges, so the exclusion read as proven.
- GitHub #1476 (V1-2308): `corvint help "docs corpus"` was refused, although the refusal lists
  `docs corpus` as a valid topic.
- GitHub #1479 (V1-2314): `docs flows generate --corpus` with a multi-repository corpus whose
  primary is another repository failed with a misleading checkout or bad-object error. Corpus Open
  rederived the corpus against `--root` before the source equality check ran.

## Decisions

- RUBY-AFFECTED-V0-045: without a Rails application, the plugin still maps autoload constants
  when the repository has an `app/` root or a `.corvint/ruby-autoload.json` declaration. It uses
  the same `internal/rubyautoload` mapper and inflections as `corvint impact`. Edges are advisory
  and transitive (`ADVISORY_CONSTANT_REFERENCE`). Unresolvable mapping (constantize, an invalid
  declaration) stays a `ruby:rails-autoload` frontier, so the scope fails closed. A lib-only gem
  gets no convention edges. Rails boot edges stay Rails-only. Impact's per-path baseline
  `dynamic-dispatch-unresolved` unknown is deliberately not copied: it would make every Ruby plan
  UNKNOWN (ticket criterion 6 reading, recorded in the requirement).
- DCP-V1-057 amended: a nested topic passed as one argument resolves like its separate-word form.
- AFU-V1-091: generate parses the corpus and refuses a primary repository/revision mismatch with
  the typed `flowdocs-corpus-repository-mismatch` code. The message names both repositories and
  says when the generation repository is declared only as an additional repository. The
  additional-repository form is not accepted (non-goal; it needs a corpus-root input and
  cross-repository anchor rules). FLOW-DOCUMENTATION.md states the supported shape.
- All three requirements are accepted under the owner's standing auto-accept (2026-10-11).

## Evidence

- #1475 live repro: the base binary selected 1 of 2 specs. The lane binary selects both, the second
  with `ADVISORY_CONSTANT_REFERENCE`, and the plan stays BOUNDED.
- New tests, each shown failing without its fix:
  - `TestConventionAutoloadEdgesWithoutRailsApplication_RUBYAFFECTEDV0045`,
    `TestConventionAutoloadFailsClosed_RUBYAFFECTEDV0045`,
    `TestRailsSignalsWithoutApplicationStayUnbounded_RUBYAFFECTEDV0045` (ruby plugin).
  - `TestAffectedSelectsEveryImpactAutoloadTest_RUBYAFFECTEDV0045` (rubyprofile, parity with
    impact).
  - `TestListedHelpTopicResolvesAsOneArgument` (cmd/corvint; every listed topic, both forms).
  - `TestFlowDocsCorpusRepositoryMismatchIsTypedUpFront` (flowdocs; without the fix it failed with
    `corpus-refused: invalid additional repository declaration`).
- Focused package runs, affected-selected sibling and importer packages, and doc gates are listed
  in the PR body.

## Not delivered

- V1-2192 (#1406) was already delivered by PR #1413 / 4692b0071, which is in the base.
- V1-2201 (#1406 remainder) was skipped. Its criteria need a live Linux `/proc/loadavg` run and a
  live OpenCode qualification under synthetic CPU load, which this macOS lane host cannot run. The
  same-window baseline changes live verdict semantics and can only be validated by that live run.

## NOT_RUN

- Tier 2 affected units are left to batch CI, as is the exhaustive `make gate`.

## Rollback

Revert the lane commits. Each requirement section names its own code and tests. The Ruby plugin
then returns to Rails-only autoload edges, help to separate-word nested topics, and flowdocs to the
post-Open equality check with the old messages.
