# More floor packages get measured read scopes

Owner request (2026-10-01, V1-0246, V1-0643): make CI run only the tests a change needs. This
follows AFP-V0-023 (`2026-10-01-declared-test-read-scopes.md`), whose derivation left fifteen root
locators on the floor as NOT_RUN because they failed unconfined in its container. The re-attributed
floor cost (`2026-10-01-behaviorfalsify-hermetic-fixture.md`) ranked them; this entry measures the
expensive ones again. `internal/behaviorfalsify` is handled separately by its hermetic fixture.

## Method

The method is the one AFP-V0-023 used, under CI's conditions: a `golang:1.27.1` container with
`--init`, a full clone with `.git`, an empty `GOFLAGS` and `GOPROXY=off`. An `strace -f` pass
proposed entries. Each candidate then ran unconfined and through the `.github/testconfine` wrapper
(Landlock ABI 4), and the per-test `go test -json` outcomes were compared. A package is declared
only when every outcome is identical.

## Doccorpus self test moves to its own package

`internal/doccorpus` was traced as reading the whole root. The cause was one test,
`TestCorpusSelfDocumentation` (DCP-V1-019 self), which indexes Corvint's own checkout at `HEAD`.
The other tests use the committed example corpus. The self test now lives in
`internal/doccorpus/selfcorpus` and is unchanged except for its package and its relative path to the
root. It stays a deliberate root locator and is selected for every change, but it takes about 8
seconds, while the whole package takes about 236 seconds on darwin/arm64. Without it, the trace of
`internal/doccorpus` reads only `examples/documentation-corpus/` outside its own directory.

With `examples/documentation-corpus/` declared, all 315 test outcomes of `internal/doccorpus` were
identical confined and unconfined, with no skips, so the package is declared.

## Results

| Package | Outcome |
| --- | --- |
| `internal/doccorpus` | Declared `examples/documentation-corpus/`. 315 outcomes, identical in both modes. |
| `cmd/corvint-postmerge-metrics` | Declared `[]`. 4 tests, identical in both modes. |
| `cmd/corvint-analyzer-c-jni` | Declared `go.mod` and `internal/analyzernativebridge/`. The test reads the bridge goldens, the bridge sources and `go.mod`. 1 test, identical in both modes. |
| `internal/tasks/authority`, `internal/tasks/store` | Rejected. They pass unconfined and fail confined: the Tasks store opens `/` to walk a path one component at a time, and Landlock cannot grant `/` without granting the whole tree. The repository literals that put them on the floor are fixture data. |
| `internal/companionrelease` | Rejected. Its tests read the checkout's `.git`, which the wrapper denies by design. V1-0643 tracks a hermetic fixture. |
| `internal/console` | Rejected. It reads the root directory. |
| `conformance/release-artifact-v0` | Rejected. It lists the top-level `.agent-evidence` directory. |
| `cmd/corvint-go-test-provider`, `internal/authoritystore`, `internal/dashboard/source`, `internal/lspstdio` | NOT_RUN. They fail unconfined in the container, so there is nothing to compare. |

Both measurements used this branch's planner with only `docs/specs/README.md` dirty:

- **At `5e7dfab1`, the AFP-V0-023 head:** 64 packages, 36 of them `UNBOUNDED_READER`.
- **On this branch:** 62 packages, 34 of them `UNBOUNDED_READER`.

The three newly declared packages left the floor, and `internal/doccorpus/selfcorpus` joined it.

## Hosted run of AFP-V0-023

At `5e7dfab1`, the head of the AFP-V0-023 pull request, all four hosted `go-product-shard` jobs
passed. Each built the wrapper, probed Landlock ABI 7, and ran the full suite with `go test -exec
test-confine -json -p 1 -count=1 -race`. This clears the "Hosted run" NOT_RUN residual in that
entry.

## Expected effect

The floor attribution put `internal/doccorpus` at about 168 package-seconds per merge. It now runs
only when its sources, their dependencies or the example corpus change. `selfcorpus` takes its place
on the floor at about 8 to 17 seconds per run, so the expected saving is roughly 150
package-seconds per merge. This is an estimate from the attribution; the merge replay was not
repeated. The two `cmd` packages run in seconds, so declaring them mainly shortens the floor list.

## Residuals

- **Tasks packages.** `internal/tasks/authority` and `internal/tasks/store` cost about 430
  package-seconds per merge from the floor. Confining them needs either a store that does not open
  `/` or a different enforcement mechanism. Neither is attempted here.
- **Container failures.** The four NOT_RUN packages need a runner image with their tools or
  services before they can be measured.
- **Focused checks only.** `tools/gate-affected-select` and `internal/doccorpus/selfcorpus` pass,
  and vet is clean. `make gate` was not run, following the owner's focused-test preference.
