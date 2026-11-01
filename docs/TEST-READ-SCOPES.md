# Test read scopes: what an entry allows and how to check it locally

`.corvint/test-read-scopes.json` (`corvint-test-read-scopes/0`, `AFP-V0-023`) declares, per Go
package, the repository paths its tests may read. The confined CI shard runs every declared
package's test binary under Landlock: below the repository root it can read only the package's own
directory and the listed entries. An **empty list** (`"internal/gateselect": []`) therefore means
the tests read nothing of the repository outside their own directory. A test that reads anything
else (a spec, `go.mod`, another package's sources, a repository walk) fails there with a
`permission denied` that looks like a content failure (V1-3585).

## Before you push a test that reads repository files

1. **Static check, every host.** `make test-read-scopes-check` fails naming the package, the test
   file, the out-of-scope path and `.corvint/test-read-scopes.json` for every string literal that
   names a tracked path outside the package's entries, including a path resolved from the
   repository root once a test locates it (`".."`, `"../.."` or the Git top-level lookup, `V1-3234`;
   a loop that climbs with `filepath.Dir` until `go.mod` exists, or a `runtime.Caller` path climbed
   by nested `Dir` calls to the root, `ASD-V0-1140`), and the dependency directories a test's
   `go list -deps` or `go build` loads (`V1-2037`, `AFP-V0-847`). A target built as
   `"./cmd/"+name` inside `for _, name := range []string{...}` loads only the listed packages
   (`ASD-V0-1141`). It reports every finding in one run, including a go target it cannot resolve
   (`ASD-V0-1142`). It runs in `make doc-gates` and the closeout precheck; run it by hand as soon as
   the test exists, because a lane's focused `go test` runs unconfined and passes.
2. **Runtime twin, Linux with Landlock ABI 2 or later.** The static check cannot see a path the test
   computes (a walk from the root, `git ls-files`, a path read from a file). Build the same wrapper
   CI builds and run the package under it:

   ```sh
   d=$(mktemp -d) && mkdir "$d/cmd"
   cp .github/testconfine/scopes.go .github/testconfine/landlock_linux.go .github/testconfine/landlock_other.go "$d/"
   cp .github/testconfine/cmd/main.go .github/testconfine/cmd/exec_unix.go .github/testconfine/cmd/exec_other.go "$d/cmd/"
   printf 'module github.com/Beamfall/corvint/.github/testconfine\n\ngo 1.27.3\n' > "$d/go.mod"
   (cd "$d" && GOENV=off GOWORK=off GOTOOLCHAIN=local GOFLAGS= go build -trimpath -buildvcs=false -o "$d/test-confine" ./cmd)
   "$d/test-confine" -probe   # prints the Landlock ABI; exits 1 below 2
   CORVINT_CONFINE_ROOT="$PWD" go test -exec "$d/test-confine" -count=1 -run 'TestName' ./internal/gateselect
   ```

   A host whose probe prints `landlock abi 0` (for example a container without Landlock, such as
   the Unraid lane host) cannot run the twin: record it `NOT_RUN` and rely on the static check and CI.

## Fixing a finding

Declare the paths the tests read as entries of the package (a directory entry ends in `/`), keep
the read inside the package (copy a fixture into `testdata/`), or, when the read cannot be bounded,
remove the package's declaration and list it in `.corvint/unbounded-readers.json` with a reason
(`AFP-V0-025`). A literal that is not a read (a fixture repository's own path) is recorded with its
reason in `.corvint/test-read-scope-literals.json`, or marked `//corvint:fixture-path` on its line
(`ASD-V0-895`). A literal that is only compared or reported (an operand of `==` or `!=`, an argument
of `t.Errorf` and the other report methods, or the searched-for argument of `strings.Contains` and
the other `strings`/`bytes` search functions) names no read and needs neither (`ASD-V0-1143`).

A package listed in `.corvint/unbounded-readers.json` because its tests `source` a computed path
(`shell:dynamic-source`) is not bounded by `.corvint/shell-read-scopes.json`, which bounds only
`shell:runs-toolchain` units (`TSP-V0-950`). `make unbounded-readers-check` names each computed
`source` line and the fix: source a literal path, such as a helper file beside the script
(`ASD-V0-1144`).
