# Design: deadcode-ratchet

## The analyzer

`golang.org/x/tools/cmd/deadcode` builds the program's call graph by rapid type analysis from every
`main` package and, with `-test`, from every test executable. It prints the functions no root can
reach. With `-json` each record carries the package import path, the function name without the
package qualifier, the declaration position and a `Generated` flag. Generated files are left out
unless `-generated` is given; this design keeps that default.

The version is one Makefile variable, `DEADCODE_VERSION`. The proposal names `v0.52.0`, which both
the Mac and the WSL lane host already hold in their module caches. The lane may pick a newer exact
version if `v0.52.0` cannot analyse Go 1.27.3 code, and records the reason.

## Why not `go run pkg@version`, and why not a `tool` directive

`go run` builds the tool for the target named by `GOOS`, so `GOOS=windows go run ...` on a Linux
host produces a Windows binary and cannot start it. The target therefore installs the tool once
for the host (`GOBIN=<scratch> go install golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION)`
with `GOOS` and `GOARCH` unset) and then runs that binary with `GOOS`, `GOARCH` and
`CGO_ENABLED=0` in its environment. The package loader reads those variables.

A `tool` directive in `go.mod` would pin the version too, but it adds `golang.org/x/tools` and its
dependencies to the product module's `go.mod` and `go.sum`. Installing by exact version keeps the
product module's dependency set unchanged, and the Go command still verifies the download against
the checksum database.

## Configurations

Six runs: `linux`, `darwin` and `windows`, each with `amd64` and `arm64`. The tree has files
constrained to `linux && amd64` and to `linux && arm64`, so one architecture per operating system
would report a helper that only the other architecture calls.

Each run analyses every package of the module. The pattern `./...` skips directories whose names
begin with a dot, and the module has Go packages under `.github/cishards` and
`.github/testconfine`. The target lists those packages explicitly, found with
`git ls-files '.*/*.go' '.*/**/*.go'`, so that a function they call is not reported.

Six whole-program analyses are slow. The target is not part of `go test`; the lane records the
elapsed time.

## Identity and intersection

A function's identity is its package import path and the analyzer's name for it (`Name` for a
function, `Type.Method` or `(*Type).Method` for a method). The position is not part of the
identity, so moving a function inside its package changes nothing.

The report is the set of identities present in all six runs. Consequences, stated as known limits:

- A function declared on one operating system only is never in the report.
- A function with one declaration per operating system (`f` in `x_linux.go`, `x_darwin.go` and
  `x_windows.go`) is reported only when every variant is unreachable.

## Baseline and verdict

`tools/deadcode-ratchet/baseline.json`, profile `corvint-deadcode-baseline/0`:

```json
{
  "profile": "corvint-deadcode-baseline/0",
  "functions": [
    {"package": "github.com/Beamfall/corvint/internal/x", "func": "(*T).Unused", "reason": ""}
  ]
}
```

Entries are sorted by package then function and are unique. Decoding is strict: an unknown field,
a duplicate, an unsorted list or a wrong profile is refused.

The tool exits 0 when the report equals the baseline. It exits non-zero and prints:

- each reported function that the baseline lacks, with the position from one run, and
- each baseline entry that the report lacks, with the instruction to delete the entry.

The second rule makes the baseline shrink-only, the same way `tools/import-ratchet` treats a stale
edge. There is no flag that rewrites the baseline from the report inside `make deadcode`; a
separate, explicit `make deadcode-baseline` writes it, and the count test then fails until the
asserted number is changed in the same commit.

## Tests

The tests read fixture JSON in `tools/deadcode-ratchet/testdata`. They do not run the analyzer,
load the module or use the network, so they run inside `go test ./...` in well under a second.
Cases: a function reported in one run only; a new function; a stale entry; a malformed, unsorted
or duplicated baseline; a missing run; and the count of the committed baseline.

## Deleting the listed functions (V1-4827)

The list is the report regenerated at the lane's base, not the panel's list. A function is deleted
only when none of these holds:

1. It is declared under `protocol/**`.
2. It is declared in a generated file.
3. It is the subject or the target of a `//go:linkname` or `//export` directive, or an assembly
   file names it.
4. A file under `docs/specs/**`, or a top-level `docs/*.md` file, cites its name. Old build-log
   and decision entries are history and do not count.
5. Removing it breaks the build or vet of any configuration or custom build tag (see below).

A function kept by a rule stays in the baseline with the rule in its `reason` field. After the
deletion commit every remaining entry has a non-empty reason, and the count test asserts the new
number.

The analyzer already reports a function that only dead functions call, so one pass is enough. The
lane reruns `make deadcode` after the deletions to confirm.

Deleting a function can leave an import unused, which does not compile, so the lane removes those
imports. It does not remove constants, variables or types that the deletion leaves unused; it
lists them in the handback.

## Proof after deletion

- `go build ./...` and `go vet ./...` with `CGO_ENABLED=0` for each of the six configurations,
  with the dot-directory packages included.
- `go vet -tags T ./...` on the host for each custom build tag in the tree. At `4c3c20333` those
  are `race`, `live`, `beamfall_appstore` and `corvint_development`; the lane re-derives the list.
- `go vet ./...` inside `interop/cem01-go`. That module does not import the main module today; the
  lane confirms it.
- The tests of every package that lost a function, one package per `script/heavy.sh` command.
- A script that lists every function declaration the commit removed and checks each against the
  regenerated list.

## Copying the delta into the spec

The lane copies each statement of the spec delta (`specs/deadcode-ratchet/spec.md`) into the new
spec `docs/specs/deadcode-ratchet-v0.md`, inside its requirements section, as a numbered definition line that begins
`(proposed 2026-11-11; delivery not-started; V1-4801)` for DCR-V0-001 to DCR-V0-006 and
`(proposed 2026-11-11; delivery not-started; V1-4827)` for DCR-V0-007 and DCR-V0-008. The delivery
word changes only when the code and its tests are on the branch. The intent word stays `proposed`.

## Alternatives rejected

- **Union instead of intersection.** It would list a function that another operating system uses,
  and V1-4801 criterion 2 forbids that.
- **`staticcheck` U1000.** It works per package and does not see that an exported function has no
  caller anywhere in the program.
- **Running the analysis inside a Go test.** Six whole-program loads on every `go test ./...`
  would cost minutes on each run.
