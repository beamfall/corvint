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
   by nested `Dir` calls to the root, `ASD-V0-610`), and the dependency directories a test's
   `go list -deps` or `go build` loads (`V1-2037`, `AFP-V0-847`). A target built as
   `"./cmd/"+name` inside `for _, name := range []string{...}` loads only the listed packages
   (`ASD-V0-611`). It reports every finding in one run, including a go target it cannot resolve
   (`ASD-V0-612`). It runs in `make doc-gates` and the closeout precheck; run it by hand as soon as
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
the other `strings`/`bytes` search functions) names no read and needs neither (`ASD-V0-613`).

A package listed in `.corvint/unbounded-readers.json` because its tests `source` a computed path
(`shell:dynamic-source`) is not bounded by `.corvint/shell-read-scopes.json`, which bounds only
`shell:runs-toolchain` units (`TSP-V0-950`). `make unbounded-readers-check` names each computed
`source` line and the fix: source a literal path, such as a helper file beside the script
(`ASD-V0-614`).

## Whole top-level trees need a reason

An entry naming a whole top-level directory (`internal/`, `cmd/`, `script/`, `docs/`, `.github/`
and so on) selects the package's tests for every edit under that tree, so `make
test-read-scopes-check` fails on it, with a `fix:` line, unless
`.corvint/test-read-scope-trees.json` (`corvint-test-read-scope-trees/0`) gives a reason
(`AFP-V0-3010`). Prefer narrowing (`AFP-V0-3011`):

1. Measure what the tests open: compile the package's tests (`go test -c`) and run the binary from
   the package directory with `GOFLAGS=-buildvcs=false`, as the confined shard does, under any
   tracer that records file and directory opens below the root. Compare the result with the
   static check's findings for the package with the tree entry removed, and read the tests for
   darwin-only or opt-in reads a Linux run skips.
2. Replace the tree with the measured reads: each literal file, and each package directory of the
   import closure of a literal `go build` or `go list` target the static check models, as a
   subtree entry.
3. Run the static check and the package under the runtime twin above. A read you missed fails there
   with `permission denied`.

Keep the tree, and record why, when a test walks, lists or globs it (a Landlock grant that lets a
directory be listed covers its whole subtree), checks the existence of paths read from data
(`os.Stat` is not confined and a trace does not see it), builds a target the static check cannot
model (a variable, a helper argument, a `"./cmd/"+name` concatenation), reads an import closure of
64 or more package directories such as `./cmd/corvint`'s, or runs scripts that compute repository
paths. The reason begins with the tracked file that holds the read and then names it:

```json
"internal/diagnostic": {
  "internal/": "internal/diagnostic/gate_test.go treeSources walks internal/ and reads every non-test .go file outside testdata (AFP-V0-3011 (1))"
}
```

A reason whose package no longer declares the tree is reported for removal without failing.
