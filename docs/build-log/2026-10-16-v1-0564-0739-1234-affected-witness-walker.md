# V1-0564, V1-0739, V1-1234: live-worktree witness rules and one Playwright listing walker

Base `5b6b85441`. No requirement is added or amended; both changes make code and tests conform to
accepted rules: `AFP-V0-012` rules (a) and (b), and `TJAA-V0-021`.

## V1-0564 and V1-0739: the live-worktree witness check

`TestSelectionOnTheLiveDirtyWorktree` resolved every witness other than the reader and declared-scope
kinds by requiring `OwnerOf(DirtyPath) == Via[0]`. The Go structural rules of `AFP-V0-012` select
units for dirty paths that no unit owns, so the check rejected witnesses that `Select` produces
correctly:

- Rule (b) for a non-Go path inside a package gives `ENCLOSING_PACKAGE` (V1-0564). It also gives
  `DEPENDENCY_PATH` chains that start at the package holding the path, or at any enclosing package
  that embeds.
- Rule (a) for a Go path in a directory with no package gives `DEPENDENCY_PATH` with `Via=[importer]`
  for each unit with an edge to an absent package of that name (V1-0739). `.github/cishards` is not
  a unit, so editing `order.go` there triggers this.

The check is now `witnessResolves`. It re-derives each rule in the test from the public graph, and
does not call the product rule:

- Reader, unbounded-reader and declared-scope checks are unchanged.
- Every chain must end at the selected unit.
- An `ENCLOSING_PACKAGE` chain is the enclosing package alone.
- Otherwise the chain's first unit must own the path. Failing that, for an unowned Go path, it must
  be the package in the path's directory, or, with none there, an importer of an absent package
  named by that directory. For a non-Go path, it must be the package directly holding the path or an
  embedding enclosing package.

`TestWitnessesResolveUnderTheGoStructuralRules_V1_0564_V1_0739` is the bounded fixture. A temp
module has a data file in `core`, a dependent `app`, and a `user` that imports an absent `gone`. It
checks that the three produced witnesses are exactly the expected ones and resolve. It also checks
that six forged witnesses, which no rule produces, do not resolve.

## V1-1234: one listing walker

`collectPlaywrightListUnits` (affected discovery, `PlaywrightDiscoveryFromList`) differed from the
corpus walker (`PlaywrightListedTests`) in two ways:

- It took a nested suite's `file` as the spec's file.
- It recorded a spec with no execution as a listed case, so an empty spec could explain a
  `grepInvert` omission.

Both entry points now use `walkPlaywrightListing`. The rules for attributing a file, the describe and
case titles, the path refusals, and the rule that only a spec with an execution is a case all live
in that one walker. Each caller adds only its own checks through a visit callback. The corpus caller
requires an ID and a location. Discovery records (project, file) units. Affected discovery
therefore now refuses a listing in which a nested suite claims another file. It also refuses a
`grepInvert` omission that only an empty spec would explain.

`TestPlaywrightListedTestsCasesMatchExecutions_TJAA_V0_021` runs its three cases through both entry
points: a nested suite file, an empty spec, and a title-only top-level suite.

## Evidence

At the base, both live-worktree failures reproduce:

- Dirty `conformance/delta-v0/README.md` fails with the `ENCLOSING_PACKAGE` witness for
  `go:.../conformance`.
- Dirty `.github/cishards/order.go` fails with `DEPENDENCY_PATH Via=[go:.../internal/sessiontest]`.

After the fix, both dirty states and their union pass the witness checks. The test then skips only
the absolute 100 ms budget under V1-1343, at a host load average of 75 to 109.

At the base, the parameterized Playwright test failed 2 of 6 subtests, both through
`PlaywrightDiscoveryFromList`: the nested suite was accepted, and the empty spec explained the
omission. After the fix, 6 of 6 pass.

One mutation per fix, each restored afterwards:

| Mutation | Result |
|---|---|
| Enclosing rule disabled | The fixture fails on `core`'s `ENCLOSING_PACKAGE` witness. |
| Absent-package origin disabled | The fixture fails on `user`'s `DEPENDENCY_PATH` witness. |
| Empty spec recorded as a case | Both entry points fail the empty-spec subtest. |
| Nested suite `file` attributed | Both entry points fail the nested-suite subtest. |

## Limits

`witnessResolves` checks a chain's origin and its end, not each edge in between. The live test
never checked edges either.
