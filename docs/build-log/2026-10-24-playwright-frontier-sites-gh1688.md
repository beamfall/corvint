# Playwright frontier sites and tag-closure precision

Date: 2026-10-24. GitHub #1688. Requirements `AFP-V0-423..426` in
`docs/specs/affected-plan-v0.md`, each accepted under the owner's standing auto-accept
(2026-10-11). AFP-V0-420..422 were taken by the open self-hosted-runner PR #1689, so this change
starts at 423.

## Item 1: the executable-config site

When `playwrightStaticMembership` refused a config, `observe` raised `FrontierConfig` at
`dynamicConfigMarker`, the first marker in the config by offset. That marker was usually the
literal `testDir`, even when the refusal came from a later `testMatch` on a setup project, a
`globalSetup`, a membership unknown, or a file the profile owns that the adapter does not classify
as a Playwright test. The gate now returns the blocking site and token itself, so the frontier
never names a modelled `testDir` or `projects:` marker. Which configs keep the frontier is
unchanged. A config whose only computed value is a project's `dependencies` already passed the
gate, and a test now pins that. `TestFrontierSitesNameFileLineAndToken_GH1604` asserted the old
`testDir` site for a config with `testMatch` on the same line, and now expects `testMatch`.

Accepting `testMatch` in path mode was not attempted. Playwright matches it against the absolute
file path, and the general adapter's units do not depend on the checkout root.

## Item 2: the dynamic-source detail

In Playwright mode, `FrontierConfig` and `FrontierE2ERuntimeDependency` are filtered out before
the shared selection runs. So the `dynamic-source-reachability` unknown the issue reported came
from another widening unknown of the shared plan: an unowned or unindexed dirty path, an
unobserved directory, or a language frontier reason. Its detail now lists those unknowns. A
language frontier reason is shown with its sites, using the same bound and pinned-first order as
the other frontier details. The issue's remaining UNKNOWN therefore names its real cause on the
next re-test. It need not be the config site of item 1, which never widened Playwright-mode
selection.

## Item 3: details objects with no tag

A probe showed that the issue's details object, `{ annotation: annotation('some-id') }`, already
passed the text rules of AFP-V0-409. The stop came from another construct in the same spec, and
the stop text did not say which one. Two changes follow:

- The stop now names the first rule broken and its line.
- The in-place array mutators no longer make a file inexact. Every literal tag array is the value
  of a literal `tag` key. A reference to it needs one of three things, and each still keeps the
  file inexact:
  - a `tag` token outside a key, which fails `playwrightLiteralTagProperty`;
  - a computed member access;
  - a reflective global, route or helper.

  So `push`, `fill` and the rest were redundant, and they rejected ordinary `page.fill` and
  `ids.push` calls. The GH #1647 hidden-tag cases that used `push` stay rejected by their computed
  access or `constructor` route.

The enumerating helpers and the global aliases (`values`, `window`, `JSON`) still reject. Whether
they can be narrowed soundly is left to a ticket.

## Item 4: the runner summary

The execution `e2e-runtime-dependency` unknown now folds every site whose token is
`@playwright/test` or the `playwright` runner identity into one count. Other runtime dependencies
are still named. Path mode still names each site.

## Encoding

No receipt, plan field or profile encoding changes. Only detail and frontier-site text changes.
