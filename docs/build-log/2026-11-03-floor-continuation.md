# 2026-11-03 floor continuation: shared temporary parents and final replay

The owner requested completion of V1-0643 and V1-2744 against base
`6f12f4704235dc8f9bea19fc12995bfb68c746c3`, preserving the inherited work. The inherited
round-3 fixes cover data and embedded files (AFP-V0-2377), transitive constants and types
(AFP-V0-2373), all `os/exec` uses (AFP-V0-2373), subprocess writes and additional file APIs
(AFP-V0-2374). Requirement IDs stay unchanged in `affected-plan-selection-v0.md`.

## Additional finding and decision

Creating a private temporary file or directory still changes entries in its parent directory.
The unfinished AFP-V0-2374 amendment at `1d3d3cefc` requires recording that write unless the
parent is proved private. The implementation and regressions are committed at `fd6d5dec0`.
`io/ioutil.TempDir` and `TempFile` also read the environment, including when taken as values.

`TestAFPV02374TemporaryCreationKeepsParentReaders` keeps `TestCreate` and `TestReadParent`,
and drops the independent `TestControl`. The retained fault-control run disables just the
temporary-parent write rule and fails this regression: the reader is incorrectly dropped.
The complete typed-reach suite passes with the rule enabled. Cross-package constant/type
coverage and `io/ioutil`, `CopyFS`, `OpenInRoot` and `os.Root` effect assertions also pass.
The CLI data-file regression now requires an actual dependent selection and includes a mixed
code/data edit; it cannot pass merely because the dependent was absent from the plan.

## Fresh measurements with the delivered code

Artifacts and the reproducible drivers are retained privately under
`/home/russell/.cache/floor-lane/finish/`. Both binaries were built from `fd6d5dec0`.

| Real range | Store model: kept / total | Dropped | Product plan |
|---|---|---|---|
| `6a2c582ef^..6a2c582ef` | 648 / 666 | 18 | selected, 18 of 666 excluded |
| `c38c32203^..c38c32203` | 657 / 676 | 19 | whole: names `internal/tasks/intent/export_test.go` |
| `d1ccab3b6^..d1ccab3b6` | 648 / 666 | 18 | whole: names `docs/specs/corvint-tasks-dispatch-signals-v0.md` |

Planning the model took 4.5, 5.9 and 7.7 seconds, respectively. `cmd/corvint` and
`internal/tasks/cli` remain whole through their existing reader refusals. The per-test fault oracle
ran every dropped test and four kept controls on the first two ranges: 22 and 23 tests,
respectively, four observed pass-to-fail controls each, zero missed observers, recall 4/4 each.
These observations cover those faults and individual test executions; they do not establish
recall for every possible edit or an ordered suite's cross-test effects.

V1-0643's unrelated-path probe selects 27 floor units and none of `internal/doccorpus`,
`internal/companionrelease` or `conformance/release-artifact-v0`. The frozen 60 first-parent
merge list ending at the lane base contains one empty range, `9e354679f1da824e1f1b532a7099229cdcb713db`.
That range has no path selection to measure and is excluded from selection denominators.
Of the 59 nonempty ranges, the delivered plan selects companionrelease 47 times,
release-artifact-v0 37 times and doccorpus 26 times. The two converted packages previously
were unbounded floor readers for each nonempty range. With the retained measured package
times of 124.7 and 14.2 seconds, the avoided work is
`12 * 124.7 + 22 * 14.2 = 1808.8` package-seconds: about **30.1 per listed merge**, or
30.7 per nonempty merge. These are weighted selection savings, not new elapsed-time timings.
This replay supersedes the earlier round-3 45/60, 36/60 and 37-second estimates.

The measured criteria for both items remain MET. Native ticket completion is intentionally
not performed, and independent review of the final continuation is reserved for the orchestrator.

## Verification and limits

The complete typed-reach package and the focused CLI selection/use-case/help tests pass through
`script/heavy.sh`, one package per command, with `-count=1 -timeout 30m`. Windows vet passes.
The pre-continuation fixture-confinement and fault evidence for V1-0643 is retained; those source
files have not changed in this continuation. Spec regeneration reports no generated drift.

The inherited independent reports at `568eebf39` and `94f557e0b` are static review evidence,
not execution evidence. They identified the subsequent standard-library conservatism work;
the final independent review is NOT_RUN here under the owner's explicit orchestration rule.
The ordered whole-suite versus sliced-suite oracle, narrowed-suite elapsed saving, optional
package export and non-Linux host qualifications remain NOT_PRODUCED or NOT_RUN for the
reasons retained in the handback. No repository-wide gate, publication, ticket mutation or seal
is authorized in this continuation. Full friction repros are in the external lane handback.
