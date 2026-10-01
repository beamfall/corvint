# Behavior-falsification tests use a fixture repository

Owner request (2026-10-01, V1-0246): make CI run only the tests a change needs.

## Finding

The 16-merge replay behind AFP-V0-023 (`2026-10-01-declared-test-read-scopes.md`) was re-attributed
by planner witness. The unbounded-reader floor still costs about 1,817 package-seconds per merge.
`internal/behaviorfalsify` accounts for about 796 of them: it was selected as `UNBOUNDED_READER` in
14 of 16 merges and takes about 910 seconds per serial run.

The cause was one test helper. `repositoryRoot` walked up from `os.Getwd` to the enclosing checkout,
and `testRequest` read three committed blobs and `HEAD` from it through git. Production code only
reads blobs at a revision from the repositories a request names. Because the tests ran git against
the real checkout, the package could not be confined by a declared read scope either, since the
AFP-V0-023 wrapper denies `.git`.

## Change

`fixtureRepository` commits synthetic copies of the three files the request names to a disposable
repository under `t.TempDir()`. It runs git with the package's own `isolatedGitEnvironment`.
`testRequest` uses this repository instead of the checkout. The tests never compared the content
of those files with the real checkout's: the request records digests that the test itself computes
from the same repository. No production code or requirement changed.

## Evidence

- `go test -count=1 ./internal/behaviorfalsify` passes in 43 seconds on darwin/arm64, and `go vet`
  is clean.
- `corvint affected` with only `docs/specs/README.md` dirty no longer selects the package. At the
  base, the package was a root locator and was selected for any dirty path.
- From the attribution, the expected saving is about 796 package-seconds per merge, roughly 17% of
  the 4,543 that remain after AFP-V0-023.

The package now runs only when its own sources or their dependencies change.
