# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-19 WQO-V0-046..048 repository work-queue adoption (decision 0321, Beamfall/corvint#20)

Before this change, `corvint work observe` could never return `VALIDATED_AT`. `workManifest.complete`
was never set, so WQO-V0-017 always added `SOURCE_UNQUALIFIED` and `propose-wave` always abstained.
`corvint work init` and `corvint work adapter` now give any repository a committed policy,
worklist, and adapter. The observer qualifies store scope only when it reproduces the
`repository-worklist-v0` documents byte for byte.

A first cut qualified both closed mappings. It turned Corvint's own `decision-0046-v0` observations
`VALIDATED_AT` and broke the WQO-V0-021/025/032 final-check witnesses in `cmd/corvint`, which rely
on an incomplete initial capture ("initial capture did not retain incomplete scope"; three
`WQO-V0-032` codes became `MALFORMED_INPUT`). Restricting qualification to the adoption mapping kept
those witnesses and the self-dogfood contract unchanged.

Pre-landing review reproduced a symlink escape and an interrupted-write residue in `work init`.
The repair roots every write with `os.Root`, rejects a symlinked `.corvint`, delays success output,
and rolls back files created by a failed or racing initialization.

A second independent review found that init still accepted a plain directory or repository
subdirectory, a partial committed adoption without the worklist surfaced `ADAPTER_FAILED` instead
of `SOURCE_UNQUALIFIED`, and the generated adapter unnecessarily required Bash. The repair refuses
non-root targets before writing, preflights the committed adoption worklist with the other source
inputs, and emits the POSIX-only adapter with `/bin/sh`.

The first canonical-gate attempt after that repair intentionally did not qualify: the release
artifact conformance test refused the staged repair as a dirty worktree. The repair was committed
unchanged before rerunning the gate from clean, frozen source.

After the final evidence seal, current `main` advanced with the accepted corvid artwork. Merging it
correctly invalidated the pinned README workflow citation because the themed logo moved that span by
one line. The canonical gate refused the stale pin; the repair relocates it to
`README.md:188-198@3297e31e` without changing the cited workflow.

`TestWorkAdoptedRepositoryWorklist` starts from a clean fixture and runs init, a refused second
init, commit, observe, and propose-wave over four verification tickets. One is a suite batch, one a
failure-classification repair, one a test-validity receipt, and one a cleanup/retry that shares
`internal/parser`. The result is `VALIDATED_AT/UNCHANGED_OBSERVED`, `ELIGIBLE_AT` with three
tickets selected, the cleanup/retry ticket `EXCLUDED` with its collision group, and a byte-identical
repository manifest. A scratch-repository probe of the built binary recorded the unknowns:
`ERROR/SOURCE_UNQUALIFIED` with no policy, with an uncommitted adoption, and with a tracked
modification, and `ERROR/ADAPTER_FAILED` when `corvint` is absent from the fixed `PATH`.

## 2026-09-19 CRB-V0-014: owner-selected corvid artwork

The owner selected the first, corvid direction from three generated concepts and then approved
adoption ("look good. use it"). The chosen silhouette
was redrawn as editable SVG with a custom lowercase wordmark; the light, dark and universal mark
and lockup variants share geometry. The README selects a theme-appropriate lockup; the 24 px editor
icon inherits its host color. `assets/brand/README.md` records usage, raster dimensions and rollback.

Manual asset checks passed: SVG parsing, accessible titles/descriptions, no font or external-image
dependencies, PNG dimensions/alpha, shared variant geometry and README references. Independent
Codex review passed with no material findings, including comparison to the selected concept,
24 px legibility and exact SVG-to-PNG rendering parity. Requirement-index regeneration and focused
spec-requirement, requirement-definition and decision-number checks passed using an isolated index
containing the updated spec; `git diff --check` passed. Go/runtime code is unchanged; the full Go
and release gates were not run for this manual artwork slice, and installed editor qualification
remains unclaimed.

Pre-change Corvint query ran against `6a423ac091d848b8ac5b49e8002c61c252993ac3`, with three ranked
results omitted and two test-path candidates withheld. Dirty-Go advice, mutation and retrieval
evaluations are not applicable. The initial `make dogfood-change` returned `not-complete`
(`cem-prepare: git-diff-failed`, missing intent scope and outcome input); it is not passing evidence.
Post-commit CEM/OCM qualification is NOT_PRODUCED for this manual asset slice; native
CEM's PNG binary-patch limitation remains explicit. Original query, generation prompts and manual
asset hashes are retained in `/tmp/corvint-logo-20260919/` for this task.

## 2026-09-19 AFP-V0-012 fast tier: the CEM sidecar's readers, the cutover-test frontier, and the unresolved floor

Decision 0322 narrows rule (c) for `.corvint/change.cem.json` to readers whose literal resolves
to it or can form it under rule (c)'s outer partial-component semantics. The independent review
found that exact resolved-string comparison omitted a package constructing the path as
`filepath.Join("..", ".corvint/change.cem") + ".json"`; the repaired selector conservatively pairs
a naming fragment with a root-climbing or compatible root-anchored token in the same package.
These are selector-only package counts, with no tests run, on `Russells-Mac-Studio.local`,
`go1.27.1`. Selection counts come from `tools/gate-affected-select` over one `corvint affected`
receipt. The script end to end with `GO_TEST_COMMAND=:` agrees. A branch from `3f30a02` changing
`internal/touchsurprise/compute.go` plus the sidecar: 121 → 116 packages, where 116 is the count
for the Go change alone. With only the sidecar dirty: 112 → 104. `feat/build-number` against
`05e17d0` (29 changed paths): 150 → 148.

The report that "the sidecar selects nearly every package" overstates its share. The audit prints
only the first cause per package, so 105 `reader` lines do not mean 105 packages added by
literals. Of the 30 packages that named the sidecar by fixture tokens, most were also reached by
another rule. The dominant width is rule (d): 103 packages are `unresolved` at `3f30a02` and are
selected whenever any path is dirty. 37 call `runtime.Caller` or `os.Getwd` themselves. The rest
carry a root-reaching literal or depend on a package whose non-test code locates the root, 10 of
them through `cmd/corvint`. With 2 or 3 packages from the plan, any change therefore selects at
least about 105 packages.

`cmd/corvint/go_only_cutover_test.go` puts 32 packages on the frontier: `cmd/corvint` and 31
dependents. `cmd/corvint` is `package main` and has no importers. Every dependent comes from a
token edge, a literal naming `cmd/corvint` such as `go build ./cmd/corvint`, or from importers of
those packages. This is justified under the current rules. `go build` skips `_test.go`, but
`go test` and `go vet` of that package compile it, and the index cannot tell which command a
literal feeds. Those direct namers are also rule (c) readers of every path under `cmd/corvint`. A
throwaway variant stopped propagation through tokens that occur only in `_test.go` files, since
test files are never imported. It moved the single-path selection from 119 to 114. The 5 packages
it drops are `cmd/corvint-analyzer-python`, `cmd/corvint-docs-mcp`, `conformance/frontier-v0`,
`internal/dogfoodocm`, and `internal/frontiernextrepo`. It is not adopted here.

## 2026-09-18 EEP-V0 provider-to-impact workflow: synthetic fixture evaluation, and unsupported cases

`TestImpactProviderEvaluation` (`internal/extevidence/extevidence_test.go`) runs the
provider-to-`impact` composition path (`Section`) over the 5-relation mock fixture
`internal/extevidence/testdata/mock-provider.json`: 3 relations that must be admitted (one
`declared` `implements`, one `inferred` `mockdocs:enables`, one `observed` `verifies`) and 2 that
must be excluded (one `learned` relation, `EEP-V0-007`; one relation from a foreign provider
endpoint, `EEP-V0-006`). Results on commit `4a2c00f` (`Russells-Mac-Studio.local`, `go1.27.1`):
precision 1.000 (3 of 3 admitted rows expected), recall 1.000 (3 of 3 expected relations admitted),
zero false-positive relationships, zero `learned`-evidence admission, abstention accuracy 2 of 2
(the learned relation reports `excluded` and the foreign-provider relation reports `unresolved`,
both under `unknowns`, neither admitted), latency about 0.68 ms, and a 3022-byte `context.external`
section. `TestSelectionEvaluation` was rerun the same day over its existing corpora (63 cases:
see the two entries below) with the same results already on record. The fixture used here is one
record with five relations, not an independent corpus; the entries below already exercise a larger,
independent labelled set for the fail-closed selection profile. This shows the exclusion and
authority-assignment rules hold on the documented worked example; it is not an adopter outcome.

Unsupported in this slice, per `external-evidence-provider-v0.md` and `external-test-selection-v0.md`:
command, MCP, and remote provider transports (file transport only); multi-hop obligations beyond one
relation hop; and checkout worktree inspection for a V1 checkout binding (a checkout's canonical path
is echoed, never opened). None of these are measured above; none are estimated. The Change Frontier
sidecar for external obligations landed separately (EFO-V0, entry below) and is not measured here.

## 2026-09-18 EFO-V0 external obligations sidecar: reference-only join to the frontier

Decision 0313. `corvint obligations --cem FILE --impact FILE` writes `external-frontier-obligations/0`
(`docs/specs/external-frontier-obligations-v0.md`). The CEM and frontier wires are unchanged; the
sidecar cites the frontier through `binding.cem_sha256` (the `CF-V0-019` raw-copy digest) and CEM hunk
ids, and nothing reads it. Tested by `TestObligations*` in `internal/extevidence` and `cmd/corvint`;
no adopter receipt or labelled review sample exists yet, so promotion stays open.

## 2026-09-18 EEP-V2 path-to-path relations: synthetic conformance evaluation

Decision 0312. `TestSelectionEvaluation` now runs over both labelled corpora: the 23 ETS-V0 cases,
plus 40 cases in the independent two-repository fixture
`internal/extevidence/testdata/conformance-path/cases.json`, 63 in all. Ten of the path cases
cover directory scopes (`EEP-V2-012`, `EEP-V2-013`): a held path, a changed test inside the scope,
a scope that widens, and missing, dirty, other-repository, and slash-less cases that must not
narrow. Results: precision 1.000 (36 of 36 selected tests expected), unsafe narrowing 0 of 49 cases
that must not narrow, abstention accuracy 6 of 6, latency p50 about 98 ms and max about 181 ms per
case on one loaded development host, and a largest `test_selection` member of 5393 bytes. The V1 and no-provider CLI
tests keep their bytes. A V1 record carrying the same path relation stays an `unsupported`
unknown, which is why `TestAffectedSelectionPathRelation` gives `full` under V1 and `narrow` under
V2. The corpus is synthetic and was authored with the feature. It shows that the per-side and
worst-side rules hold. It is not an adopter outcome.

## 2026-09-18 ETS-V0 external test selection: synthetic conformance evaluation

Decision 0311. `TestSelectionEvaluation` over the 23 labelled cases in
`internal/extevidence/testdata/conformance-selection/cases.json`: precision 1.000 (16 of 16 selected
tests expected), unsafe-narrowing 0 of 20 cases that must not narrow, abstention accuracy 3 of 3,
latency p50 about 35 ms and max about 125 ms per case on one development host, and a largest
`test_selection` member of 4470 bytes. The corpus is synthetic and authored with the feature, so
these numbers show the fail-closed rules hold. They are not an adopter outcome, and promotion needs
a corpus drawn from a real change history.
