# Selection hardening: review fixes for headline counts, root context and glob dialect (V1-5401)

Date: 2026-11-11
Ticket: V1-5401
Kind: independent finding (review FIX, three P2 and two P3)
Spec: `docs/specs/selection-hardening-v1.md` (SLF-V1-003, SLF-V1-004, SLF-V1-008)

An independent review of the first round asked for three P2 fixes and noted two P3s. Each P2 test
failed on the first-round head (9dd4ddbc3) and passes after the fix. Selection fails closed: where
a form cannot be read exactly, the planner refuses and keeps the frontier, or reads a wider
membership. Code behind a held gate was fixed too, because it ships once FIH-V1 passes.

## P2-1: the headline cut removed the counts (SLF-V1-008)

The counts were appended after ` revision=<40> base=<40>`, and the 200-byte bound (AFP-V0-3642)
cut them off a real plan's line. Moving them ahead of ` revision=` would not have been enough,
because the summary line itself carries one `reason=N` per unknown reason and has no bound. They
now follow the summary line's `selected=` total, inside its fixed opening (at most about 100 bytes
with the prefix). The headline keeps its ten-line bound, so the counts did not get a line of their
own. Tests: `TestAffectedHeadlineCountsSurviveTheLineBound_SLFV1008` (new),
`TestAffectedHeadlineCountsTestsAndDriversApart_SLFV1008` and `TestAffectedHeadlineIsBounded_AFPV03642`
(whose own fixture had cut the counts to `drive...`).

## P2-2: root `require.context(".", false)` missed root files (SLF-V1-003, held)

The non-recursive membership glob was `anchoredDirectory(root)+"/*"`, which is `/*/*` at the
root and needs two path components. At the root the glob is now `/*`, which every path matches.
Tests: `TestSLFV1003RequireContextMembershipGlobs` covers every root and recursion combination,
and a new root file reselects the importer with `globExpansionNarrowing` on.

## P2-3: glob dialect translation narrowed without notice (SLF-V1-003 held, SLF-V1-005 held)

- A brace range such as `{1..3}` was read as the literal text `1..3`. `globSource` now refuses a
  brace body that contains `..`, so the call keeps `typescript:dynamic-import` and a config entry
  keeps `typescript:executable-config-unresolved`. `membershipGlobs` falls back to the literal
  directory for the same form.
- `path.Match` reads `[!x]` as the characters `!` and `x`, not a negation, so a membership glob
  with one missed the files it admits. A component whose class is negated, or which starts with
  `]` or holds a `[`, now falls back to the literal directory.
- A POSIX class such as `[[:alpha:]]` was split at its first `]`. `globSource` now refuses any
  class body that holds a `[`.
- Tests: `TestSLFGlobRegexpDialect` (brace ranges, POSIX classes, membership fallbacks),
  `TestSLFV1003NegatedClassMembership` (a new file a `[!h]` class admits reselects the importer)
  and two new computed calls in `TestSLFV1003ComputedGlobKeepsTheFrontier`.

## P3s

- Manual-mock edges ignore design decision 6's restriction to Jest and Vitest. This only widens a
  plan, and the spec now says so.
- `jest.setMock`, `jest.unstable_mockModule`, `vi.importMock`, and also `jest.requireMock` (the
  Jest twin of `vi.importMock`), are now read as `jest.mock` is: a literal target is an edge and a
  computed one raises `typescript:mock-target-unresolved`. Before the change a computed target
  there raised nothing. Test: `TestSLFV1004OtherMockCalls`.

## Not changed

`internal/testsource/unselected_glob.go` has its own `globRegexp` for the unselected-runner
census. It is not part of this change and was left as it is.
