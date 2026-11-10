# Design: literal-temp-root

All proofs run per file, on the untyped AST that `testmetadata.go` and `locatorproof.go` already
parse. A proof that fails grants nothing, so every unproven case keeps today's selection.

## Decisions

1. Proofs work on each occurrence, and drop a token only if every occurrence is proven.
   - The scanner records each literal's byte offset. The temp-root and bounded-climb proofs return
     the set of offsets they have proven, as AFP-V0-2950's `components` already does.
   - A token is dropped from the unit only when every occurrence of it in every file of the unit is
     proven or already excluded.
   - An occurrence in production code always keeps its token (AFP-V0-110).
2. What counts as a proven temp root (AFP-V0-108). An expression is a temp root when it is one of:
   - `X.TempDir()`, where `X` is a parameter of type `*testing.T`, `*testing.B`, `*testing.F` or
     `testing.TB`;
   - the first result of `os.MkdirTemp`;
   - a local variable assigned once from a temp root, whose address is never taken and which no
     closure reassigns;
   - `filepath.Join` or `path.Join` whose first argument is a temp root;
   - a call to a same-package function whose every return statement yields a temp root and which
     takes no root parameter.
   The function test is the AFP-V0-828 summary from `tools/test-read-scopes-check/temp_root.go`,
   moved into a shared package rather than copied.
3. What counts as a proven literal (AFP-V0-107). A literal is proven when it is a trailing argument
   of a join whose first argument is a temp root, or the right operand of `root + "/x"`. The rule
   does not apply when the temp root reaches any of the following (AFP-V0-109):
   - `os.Symlink`, `os.Link` or `os.Rename` as a target or destination, or `os.CopyFS` as its
     destination;
   - an `exec.Command` argument, unless the command's `Dir` field is a temp root;
   - a function outside the package, other than the allowed standard library list in
     `pathJoinFuncs` and the `os` read and write functions.
4. Bounded climbs extend AFP-V0-2950 (AFP-V0-111, AFP-V0-112).
   - Covered: a join whose leading arguments are `..` literals and whose later arguments are plain
     literal components, and a single literal such as `"../../integrations/x.json"`. Either is
     resolved from the package directory. When the result lies inside the module root, the
     occurrence adds that root-anchored token (`/integrations/x.json`) in place of an unbounded
     reason.
   - The consumer must be a bounded read (`boundedJoinConsumed`).
   - A local assigned once from `filepath.Abs` of a climb, or from `filepath.Join(os.Getwd(), "..",
     ...)`, is a repository-root local. It is bounded when its only uses are bounded joins, which
     is the AFP-V0-2950 test.
   - A path that resolves above the module root stays unbounded.
5. Comparison-only climbs (AFP-V0-113).
   - A `..` literal is not a locator when its only consumer is one of: `==` or `!=`, a `case`
     label, or the second argument of `strings.HasPrefix`, `strings.HasSuffix` or
     `strings.Contains`.
   - All of these return booleans. The existing `literalComparison` handles the first two cases.
   - A string element of a composite literal stays unproven. A table-driven test can pass it to a
     read.
6. Every reason is reported (AFP-V0-114).
   - The plan witness of a selected unit lists each rule that would select it. The list is capped
     at 8 reasons, each with file:line.
   - The JSON keeps the first witness in its current field and adds a sorted `alsoSelectedBy`
     list. A consumer reading only the first witness sees no change.
7. Hub read scopes are the lever for `cmd/corvint`.
   - Its production code calls `os.Getwd` (`init_adopt.go:44`) and `git rev-parse
     --show-toplevel` (`runs_baseline.go:288`, `test.go:477`).
   - Tests that run that code read the real repository. No static proof inside one package can
     bound that.
   - An AFP-V0-023 scope, enforced by Landlock in full CI, can bound it, because project-owned
     authority outranks syntax.

## Expected effect (inferred, not measured)

- `internal/liveverify/affected`: after decisions 4 and 5, its remaining rule (d) occurrences are 2
  composite-literal elements (`readers_cache_test.go:98`, `testselection_test.go:196`). It stays
  an unbounded reader unless it declares a scope.
- `internal/contextindex`: 2 composite-literal elements remain (`impact_reverse_import_rules_test.go:243`,
  `reference_scan_test.go:40`), plus a package-level climbing path (`receipt_budget_test.go:175`).
  It stays an unbounded reader unless it declares a scope.
- `cmd/corvint`: it stays an unbounded reader because of decision 7.
- After rule (d) is gone, class (c) and (e) occurrences still keep the `docs`, `AGENTS.md` and
  `Makefile` tokens. So the code rules alone free no hub for the three measured paths. Task 6's
  scopes are required for each hub.
- The code rules still pay off without scopes. They remove rule (d) from smaller packages that have
  only literal climbs, which the measurement saw in `cmd/corvint-docs`, `tools/ci-reuse-plan` and
  `tools/test-read-scopes-check`. They also cut the number of tokens a scope must cover.

## Failure modes

- A temp root is later symlinked to the repository. Decision 3 refuses it.
- A test runs `git worktree add <tmp>` or `git clone . <tmp>` from the package directory. The exec
  screen in decision 3 refuses it unless `Dir` is a temp root.
- A helper returns a temp root on one path and a repository root on another. Its summary is not
  "every return", so it grants nothing.
- An environment variable or flag supplies the root. That is not a temp root, so it grants nothing.
- A climb resolves through a symbolic link. AFP-V0-9450 `literalLinks` still applies to the added
  anchored token, and the case stays uncertain when it does not resolve.
- A `..` literal is used both in a comparison and in a read. Each occurrence is judged on its own,
  and the read keeps the unit unbounded.
- A parse fails or a file has a dot import. No exemption for that file, following the AFP-V0-828
  rule.
- A declared hub scope is too small. Landlock in full CI fails the package. The scope is then
  widened, or the hub stays unscoped. The fallback is the current selection, never a miss.

## Acceptance evidence

- Unit tests in `internal/liveverify/affected/golang` for each requirement, one per refused case
  in the failure modes.
- The scratch probe re-run by the lane as a maintained test fixture. It covers the 30 hub pairs and
  the `qzx-probe-nothing.md` plan, before and after.
- An `AFP-V0-114` golden plan showing `alsoSelectedBy` for a unit with both rule (c) and rule (d)
  reasons.
- Landlock full-CI runs of each hub with its declared scope (task 6). The result is
  `NOT_OBSERVED` until those runs exist.
- A fault-injection check that adding a real read of `AGENTS.md` to a hub test still selects that
  hub.

## Rollback

- Revert the change's commits. The proofs only remove tokens and reasons, so a revert restores the
  wider selection.
- A hub scope is reverted by deleting its entry in `.corvint/test-read-scopes.json`. A plan with
  that file dirty already selects what it must.
- Requirement IDs AFP-V0-107 to AFP-V0-114 are retired, not reused.
- In the field, `CORVINT_SELECTION=full` (NMS-V1-008) covers a bad selection without a revert.

## Open questions for the owner

1. Should composite-literal string elements get a data-only proof inside one package? It would
   need to follow the slice to every consumer. Or should those hubs rely on scopes alone?
2. Is a 256-entry scope enough for `cmd/corvint`? Its tests read `docs/specs/INDEX.json` and run
   the CLI against the repository. The size is unknown until a Landlock run reports its reads.
