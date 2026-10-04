# Unbounded-reader ratchet failed on its own merge (V1-0719 follow-up)

## Finding

`make unbounded-readers-check` failed on `main` at `6bf78ff7`, the merge of pull request 530 that
introduced it: 56 unbounded test packages against the recorded ceiling of 45. The ceiling was
measured on the pull request's branch. Eleven test packages reached `main` from other changes
after that branch was cut and never met the ratchet: `cmd/corvint-cem-candidate`,
`cmd/corvint-test-runner`, `internal/cem/gitauth`, `internal/cemcandidate`, `internal/testrunner`
and its `dynamic`, `mobile`, `native`, `platform`, `registry` and `sql` subpackages. The pull
request's last CI run predated them, so the required `doc-gates` job passed there and fails on
every change based on the merged `main`. The 45 counted before are a subset of the 56.

## Change

- `.corvint/test-read-scopes.json` declares 14 packages. Nine are among the eleven:
  `internal/cemcandidate` reads `protocol/cem-1.0/`; the other eight read nothing outside their
  directory. Five older packages also read nothing outside their directory:
  `cmd/corvint-corpus-parity`, `cmd/corvint-corpus-republish`, `internal/corpusrepublish`,
  `internal/tasks/archive`, `internal/tasks/journal`.
- `.corvint/unbounded-readers.json`: reasons for `internal/testrunner` and
  `internal/testrunner/sql`, whose product code opens the filesystem root `/`, which the wrapper
  cannot grant, and for `tools/cem-trial` and `internal/analyzerhtmlcss` (below). Ceiling 45 to
  42, the count after the declarations.

## Evidence and limits

- `make unbounded-readers-check` fails at `6bf78ff7` and passes with this change (count 42).
- Each of the 14 packages passes under the Landlock wrapper with exactly the committed
  declaration, also with `-race`, in a local Linux container (kernel 6.8, Landlock ABI 4, `/tmp`
  on tmpfs). With an empty declaration `internal/cemcandidate` failed on
  `protocol/cem-1.0/repository/base/app.txt`, and the two `testrunner` packages failed on
  `open /`. Hosted CI is the first hosted observation.
- A confined pass does not prove a declaration: a test that skips when a read is denied passes
  without it (an AFP-V0-023 residual). `tools/cem-trial` and `internal/analyzerhtmlcss` passed
  confined with an empty declaration for that reason and were declared in the first draft. The
  independent review found the first builds `cmd/corvint` and skips on a failed build, and the
  second reads `.git` and skips when git fails; both stay unbounded with a reason. The review
  read the test files of the other 14 for skipped, embedded and subprocess reads and found none;
  it did not trace their product code beyond what the tests call directly.
- The other 34 undeclared packages failed with an empty declaration in the same run and were not
  investigated further.
- The skew can recur: a pull request whose branch predates a new unbounded package passes its own
  check and breaks `main` on merge. With the ceiling equal to the count there is no slack to
  absorb one. Nothing here prevents that; it is filed as V1-0752.

## Rollback

Delete any declaration to return that package to rule (d), raising the ceiling by one for each.
