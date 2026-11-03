<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision: import-graph readers, anchored and temp-root literals, deletions

Date: 2026-11-03. Lane selprec. Tickets: V1-4409, V1-4362, V1-4173, V1-3578, V1-3601, V1-4283,
V1-4284, V1-4278, V1-4280, V1-3864, V1-3282. Requirements: AFP-V0-2200 to AFP-V0-2206 in
`docs/specs/affected-plan-selection-v0.md`; AFP-V0-2203 amends AFP-V0-072 (2) and AFP-V0-2205
amends NSA-V0-006.

## Decisions

- **`go list` guards are import-graph readers (AFP-V0-2200 to AFP-V0-2202).** A modularity guard
  such as `exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")` observes
  the import graph, not the built binary. Its literal was a command edge, so every body edit in
  `cmd/corvint`'s closure selected it as `BINARY_EXEC` (V1-4362). `internal/swiftimpact` names
  most of the binaries it lists by a computed string (`"cmd/" + name`), so no edge reached it from
  an import change in `cmd/corvint-lsp` (V1-4409). The new witness `IMPORT_GRAPH_READER` follows
  only import-set changes: a per-directory package header (file names, build-constraint lines,
  package clause, imports) compared at base and head, plus module files. Unknown comparisons
  assume every dirty `.go` file changed an import set.
- **Anchored Go literals resolve from module bases (AFP-V0-2203).** A Go token such as `"/repo"`
  (from `"/repo/.git"`), `"/testdata/"` or a bare `"/"` matched a same-named component at any
  depth. Now an anchored token joins the root, the unit's directory or an ancestor (where its module
  lies), or passes the AFP-V0-072 placement test. The fall-through after a rejected anchored token
  no longer reaches the lone-name placement test unless a lone token really names the file.
- **Shell temp roots name no repository path (AFP-V0-2204).** `"$tmp/repo/..."`, where `tmp` is
  assigned once, unconditionally at top level, from `$(mktemp -d ...)`, adds no path token, unless
  the file can fill a directory from the repository without naming each file (recursive copy,
  archive, worktree, link, move, `xargs`).

## Independent review

A separate review of the staged diff found three under-selections, all fixed before handback:
- Shell temp roots that receive whole-tree copies (`cp -r`, `tar`, `git worktree`, `ln -s`) lost
  the only token that named the copied file. Fixed with the `populates` refusal.
- A "proven" `go list` could still print more than imports: `{{.}}`, an `Env` carrying
  `GOFLAGS=-json`, or output fed to a second command. Fixed with `bareDot`, `isolatedList`,
  `literalEnvironment` and the per-unit GOFLAGS/GOENV check.
- An embed pattern that stops matching makes `go list` fail. The header now carries
  `//go:embed` lines, and an added or removed data file below an embedding unit counts as a
  change.

It also found two edge cases, both fixed: `plan.deleted` now covers a path whose ancestor became a
file and is left out of `--path` plans, and a temp variable assigned conditionally (which could
come from the environment) no longer counts as a root. The fixes cost some precision: the `lib.js`
probe is 64 + 25 rather than the first draft's 51 + 26.
- **Extensionless shell executables are units (AFP-V0-2205).** `script/release-checklist` and
  its kind have a shebang and are run by path; they were unowned, which put the whole frontier in
  every plan touching them.
- **`plan.deleted` (AFP-V0-2206)** lists dirty paths absent from the observed tree.

## Measurements

The same two binaries were used throughout: `before` built at f89db15bd, `after` built from this
lane's tree. Runs (a) to (d) used a detached worktree at f89db15bd; (e) and the by-path probe used
one at PR 86's queue commit 64321325b. Each line gives selected / unknown, then witness tallies.

| Range | Before | After |
|---|---|---|
| (a) `--base 7169198944` (PR 84) | 179 / 5; DEP 111, BINARY_EXEC 26, PATH_LITERAL 25, DECL 7, DIRECT 5, UNBOUNDED 5 | 179 / 5; DEP 111, PATH_LITERAL 24, BINARY_EXEC 21, DECL 7, IMPORT_GRAPH 6, DIRECT 5, UNBOUNDED 5 |
| (b) body-only edit, `internal/necessity` (in `cmd/corvint`'s closure) | 70 / 1; BINARY_EXEC 32, UNBOUNDED 17, DECL 15, PATH_LITERAL 4 | 70 / 1; BINARY_EXEC 26, UNBOUNDED 19, DECL 19, PATH_LITERAL 4 |
| (c) import added in `internal/tasks/scopes` (`cmd/corvint-tasks` closure) | 69 / 1; swiftimpact by BINARY_EXEC | 68 / 1; swiftimpact by IMPORT_GRAPH_READER |
| (d) `cmd/corvint-lsp/swift_impact.go` deleted | 57 / 68; **swiftimpact not selected** | 58 / 68; swiftimpact by IMPORT_GRAPH_READER |
| (e) PR 86, `--base f89db15bd` | 183 / 83; PATH_LITERAL 87, DEP 44, BINARY_EXEC 31 | 170 / 83; PATH_LITERAL 69, DEP 44, BINARY_EXEC 25, DECL 11, UNBOUNDED 7, IMPORT_GRAPH 2 |
| `--by-path --path internal/jslinecov/testdata/repo/lib.js` at PR 86 | 84 additions (57 PATH_LITERAL: Go 24, shell 28, TypeScript 5) + 16 shared | 64 additions (34 PATH_LITERAL: Go 6, shell 23, TypeScript 5) + 25 shared |
| `_test.go`-only edit (`internal/necessity`) | 59 / 1; DECL 29, UNBOUNDED 28 | 59 / 1 (unchanged) |
| `script/heavy.sh` edit | 75 / 1 | 71 / 1 |
| `script/release-checklist` edit | 51 / 75 (74 frontier, 1 unowned) | 51 / 2 |
| body edit in `internal/contextindex` (`flowWordBefore`) | 151 / 4 | 151 / 4 |

Fault injection: with (c) applied, `TestNGIV0988TasksClosureExcludesPlannerAndImpactBinariesLinkReader`
fails ("corvint-tasks links internal/swiftimpact"). With (d) applied it fails ("corvint-lsp does not
link internal/swiftimpact"). In (d) the before binary does not select it; the after binary does.

## Findings that did not narrow

- (b) stays at 70. Six guards left `BINARY_EXEC`, but four have declared read scopes covering
  `internal/` (`go list` opens every file of the closure), and two are unbounded readers. A
  header-only scope would be a new owner declaration, enforced by `.github/testconfine`.
- V1-3578: the compile argument (a `_test.go` file is built only into its own test binary) is
  ACX-V0-852 for path literals. The test-only plan's remaining 57 units are declared-scope and
  unbounded readers, which may read test files as data (traceability and requirement scanners do).
- V1-3601: fast-tier (tier 0 and 1) reader-only selections are 0 on (a), (b), (d) and (e), and 1 on
  (c). The shapes the aff-w23 entry named (suffix and temp-fixture literals) are what AFP-V0-2203
  and AFP-V0-2204 remove in every tier.
- V1-4283: the test-only plan's 28 unbounded readers equal `.corvint/unbounded-readers.json`'s
  units, each with a recorded reason (Git reads of `.git`, opens of `/`, nested sandboxes, whole-tree
  walks). None can be bounded without a confined run that proves a declaration.
  `internal/tasks/cli`'s cause is a help string (`"../lane"`), but its recorded reason (unknown
  runtime reads) stands, so it stays.
- V1-4284: 25 of the 71 units come from `heavy.sh` → `batch-regen.sh` → `internal/dogfoodflow` →
  `cmd/corvint` → its binary's runners. The middle edge is the owner's declaration in
  `.corvint/shell-foreign-callers.json` ("internal/dogfoodflow replays a parent's copy"). The 17
  TypeScript entries the ticket names are absent at f89db15bd.
- V1-3282: in range (a), `internal/contextindex` gains an import (`IMPORT_NOT_ALREADY_IN_PACKAGE`):
  falling back is sound. For a body-only edit, the ADN-V0-660 symbol record now falls back on
  `init in internal/contextindex/dialect_hooks.go`. That init registers `webImpact` and
  `webNormalizedLines` in `internal/langplugin/dialect`'s registry, a value escape. Today only
  contextindex calls `dialect.LookupImpact`, `LookupRegion` and `ImpactNames`.
