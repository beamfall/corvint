# The three unproven README rows: root causes, fixes and what remains owner-commissioned

Base: `993fe6294abb4d112ea9d5a4ed15aefc7974bd7a`. Decision 0427. Owning intents: TCP-V0-004,
TCP-V0-022 (`docs/specs/task-context-packet-v0.md`); GPK-V0-040 and a GPK-V0-039 note
(`docs/specs/go-production-kernel-migration-v0.md`); CRT-V0-004, CRT-V0-005, CRT-V0-007,
CRT-V0-008, CRT-V0-012 (`docs/specs/cem-reviewer-trial-v0.md`). Ticket payloads: `.agent-evidence/decision-0427-tickets/` (native creation pending).

## Row 1: "Does CEM help a reviewer?"

What the 2026-09-04 pilot measured, read from `tools/cem-trial` and
`benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json`:

- The treatment prologue carried the `cem begin` map (`internal/cem/workflow/workflow.go`,
  `buildCandidate`: every hunk `disposition: unknown`, `reason: no-evidence`), its `cem status`
  worklist and `cem report`. No evidence was retrieved for either arm. The contrast was a worklist's
  wording against nothing.
- Gold was every test-or-spec file the commit touched regardless of git status, while source files
  had to be modified. The gold spans of c03 (`hardening_test.go 1:73`), c04 (`1:150`, `1:53`) and
  c05 (`widget-conformance.test.ts 1:80`) start at line 1: files the change created, absent at `C^`,
  which the reply grammar forbids citing. Those three changes scored miss 1.0 in both arms; the
  two changes that could move did (0.90 to 0.80, 0.60 to 0.50).
- Both arms lost to the stem guess (0.70 against 0.86 and 0.90). McNemar averaged five repeats
  per change first and reported 0 discordant pairs over 50 lanes.
- No human reviewer took part; the spec lists human reviewers and a judge model as non-goals.

Landed: `tools/cem-trial` selects gold only among files that exist at `C^` (status `M` or `D`) and
records the rest as `gold_unreachable`; a third arm `seeded` carries the rows `corvint context
--subject PATH --limit 8` returns in the prep clone at `C^` for each presented file, labelled
suggestions and never cited; the scorer estimates every arm against control, reports McNemar over
lanes as well as changes, computes `citable` and the verifier rate per map-carrying arm, and
decomposes the seeded arm into retrieval (`seed_coverage.fraction`) and adoption
(`miss_given_suggested`), with a `seeded_vs_treatment` contrast. Tests:
`TestSelectionRecordsUnreachableGoldSeparately`, `TestSeededArmSharesThePromptApartFromItsPrologue`,
`TestRunRefusesSeededArmWithoutCorvint`, `TestThreeArmSummaryKeepsPilotKeysAndDecomposesTheSeededArm`,
`TestTwoArmSummaryIsUnchanged`, `TestLaneLevelMcNemarKeepsDiscordance`; the pilot-shaped two-arm
report rebuilds byte-identically. No agent lane was run here: there is no Beamfall clone and no
model budget in this environment, and a run drawn now would be development data.

Independent review (experimental design): the pilot is "a placebo-sized effect on a
ceiling-censored outcome"; power the decision run for the seeded contrast at the corrected control
miss (about 55 pairs with two repeats for a 20% relative reduction from 0.50, about 26 from 0.70);
require the interval to exclude 0 and not exclude the 20% target rather than a point estimate; keep
lanes, not change means, for McNemar; report the retrieval/adoption decomposition or the seeded arm
over-credits the map; a human leg of about eight reviewers over about twelve sealed changes is the
only route to the wording "helps a reviewer". Product review: the `cem report` a reviewer reads
carries no cited text and no hunk anchoring, so a positive agent result would still not transfer to
a human; the reviewer projection with inline pinned spans, "unknown first" ordering and SARIF export
is the prerequisite for the human leg (the reviewer-projection ticket payload). Both reviews are recorded in full in the session's
scratch and summarised in decision 0427.

Wording the commissioned run can support. Positive: "In a pre-registered, sealed N-pair run on
Beamfall, an agent reviewer with a Corvint-seeded CEM missed X% of base-reachable co-changed
test/spec files against Y% unaided and Z% with an empty worklist (Δ, 95% BCa CI); Corvint's
suggestions carried V% of the gold. No human study." Null or negative: "Unproven. A sealed N-pair
three-arm run found Δ (CI) for the seeded map; the interval excludes a 20% gain." The row never
says "helps a reviewer" without the human leg.

## Row 2: forbidden results on 7 of 36 must-exclude checks

Decision 0307's breakdown: four same-path siblings (`scaffold.py:post`, `views.py:as_view`,
`app.py:do_teardown_appcontext`, `active_help.go:AppendActiveHelp`) and three expected-`OUT_OF_SCOPE`
tasks answered `READY` (`v6-flask-dispatch-requests-typo-oos`, `v6-cobra-json-docs-oos` twice). The
sealed case texts are outside this repository.

Root causes in code: `evalConfidentSymbols` admitted every declaration on a frontier path scoring at
least 60% of that path's top, pruning only values; sibling declarations share a context window and
tie. The relevance floor counted a declaration's window vocabulary as support, so one identity word
plus one window word cleared the two-word bar. The development corpus (`benchmarks/cases/*.json`,
24 cases) carries no must-exclude selector at all, which is why neither class was visible.

Landed: the sibling explain-away rule (a same-path sibling whose name support is a strict subset of
a neighbour's and whose full support is contained in the neighbour's is not admitted, unless the task
names it; four-byte prefixes answer, so `config` answers "configuration"). Tests:
`TestEvalExplainAwaySiblingsDropsNeighboursWithoutOwnEvidence`, `TestEvalNameSupportAnswersPrefixes`;
the existing floor and window tests pass unchanged. Two earlier forms were discarded: counting
identity words alone under the floor withdrew the designed three-word window rescue
(`TestSymbolWindowTableReceiptComparisonHasTeeth`), and a top-relative explain-away dropped
`GetActiveHelpConfig` behind the decoy that outscored it by three points; the symmetric containment
rule with prefix answering is what survived the corpus.

Tried and not adopted: a window-licence bar for the floor (a window licenses a declaration only when
it carries three task terms). It left the development corpus unchanged and withdrew the
within-repository near miss below, but it also withdrew `Original consumer` in the prove-bundle
fixture (`TestProveBundleQueryReproducesOnASecondClone`): a task that names a declaration and a word
its body carries is a dependency question the repository answers, so the rule costs recall of a
kind the floor is meant to keep. It is recorded in GPK-V0-039 as considered and not adopted.

Development corpus, four public repositories, `go run ./benchmarks/runner --repos flask,cobra,zod,execa`:

| configuration | top-5 | recall | critical misses | must-exclude | byte-weighted precision |
|---|---|---|---|---|---|
| `main` (`993fe629`) | 1.000 | 1.000 | 0 / 25 | 0 / 0 | 0.6998 |
| sibling rule | 1.000 | 1.000 | 0 / 25 | 0 / 0 | 0.7133 |
| sibling rule + window-licence bar (not adopted) | 1.000 | 1.000 | 0 / 25 | 0 / 0 | 0.7133 |

Case-level changes under the sibling rule: `cobra-active-help` returns `GetActiveHelpConfig` alone
(the blind-v6 decoy `AppendActiveHelp` and `activeHelpEnvVar` are gone); `zod-from-json-schema` and
`zod-treeify-errors` drop irrelevant same-file siblings; `execa-normalize-parameters` drops
`mapDestinationArguments`; `cobra-bash-completion` keeps its five relevant generators because they
share identical support. The 45 cross-repository transplant probes (every positive development task
run against each other repository, expected out of scope) answer `READY` 23 times on `main`, 23
times with the sibling rule, and 22 times with the window-licence bar that was not adopted; under
that bar the within-repository near miss `render command docs as JSON files` on cobra moved from
`READY` (`command.go:Command`) to `OUT_OF_SCOPE`, and `generate JSON documentation for every
command` was already withdrawn on `main`. The transplant `READY`s are mostly
declarations carrying two task words in their own names (`run_command`, `Options`,
`isAsyncFunction`): a lexical floor cannot refuse them, which is why blind-v7's out-of-scope probes
must be within-repository near misses reviewed by a second person, and why a transplant set is
not a measure of the sealed class. The two flask `dispatch requests to the view function` probes,
with and without the typo, stay `READY` with `dispatch_request` first; the typo probe's forbidden
status in blind-v6 is a case-authoring defect, not an engine one.

Retrieval review: must-exclude over all emitted results is the right count for a proof-carrying
packet; generate development decoys from the gold (nearest-name same-file sibling, adjacent
declaration, same-named symbol elsewhere) with one human pass; commission blind-v7 from a
preregistered pool of repositories never indexed by Corvint, authored on a machine without the
binary (the blind-v7 ticket payload). Prior art: ContextBench, CORE-Bench and CoIR score no decoys; Agent Retrieval
Bench scores abstention and reports that thresholds calibrated on counterfactual controls do not
transfer to natural no-gold cases, which is this cycle's finding in another corpus; CoREB's
graded qrels (hard negatives rank but earn nothing) are the scoring precedent for sibling decoys.

## Row 3: Core jobs at rc.1

`benchmarks/untouched-repository-v1/runs/run-001.json` records 3 treatment-only critical misses in 2
chi cases (README says 3 cases); every miss is a test file. The Corvint self-run abort (harness
crash and a loop target containing a sealed CEM) is fixed on `main` by V1-0432 and V1-0433 and waits
on the next candidate.

Orientation job rerun locally over all 20 frozen chi cases with the harness's own `orientation_case`
and `lexical_topk` (chi is development since decision 0425; the candidate here is a local build, not a
verified release artefact, so this is development evidence):

| binary | treatment-only critical misses | cases | which |
|---|---|---|---|
| `1.0.0-rc.1` (run-001, darwin/arm64) | 3 | 2 | `wrap_writer_test.go`; `middleware_test.go`, `recoverer_test.go` |
| `main` `993fe629` (with V1-0431) | 4 | 3 | the three above plus `mux_test.go` on `Don't duplicate methods in Allow: header for 405 responses` |
| this change | 0 | 0 | — |

Mechanisms, from an instrumented compile of the two failing cases: in `record response status when
flushing` the counterpart pass produced candidates `context_test.go`, `tree_test.go`,
`logger_test.go`, `throttle_test.go`, `wrap_writer_test.go` in packet order and the cap of three was
spent before `wrap_writer.go` (lexical rank 15, but 42 occurrences) had its turn; in the Allow-header
case `mux_test.go` was the strongest lexical hit (five task terms, rank 0) but, because its source
`mux.go` entered as a `test`-kind row rather than a lexical one, it was unprotected, and on the third
counterpart the only unprotected test left was `mux_test.go` itself, which the "insert at first,
remove last" rule deleted and re-inserted past the limit. `Replace "interface{}" with "any"` is a
tokeniser fact: with `CORVINT_CONTEXT_ANCHORS=on` the packet carries all eight modified files,
each with `anchor: \`interface{}\` xN verbatim` in its reason; without it, five.

Landed: the rank-relative counterpart rule (`admitLexicalPairs`, `lexicalRank`) and the anchor
field on by default with `off` as the pre-amendment bytes. Tests: `TestTaskContextSelectedLexicalPairs`
(limits 1/2/4/8/12/30; all five counterparts admitted at 12 and 30; three withheld at 8),
`TestTaskContextLexicalPairPromotion`, `TestRunTaskContextSubjectlessCounterparts`,
`TestContextAnchorsDefaultBytes` (both directions). `TestWorkFinalCheckClosingContext` fails on the
base and on this change alike in this container (`ADAPTER_FAILED` on its deadline and cancelled
subtests) and is unrelated.

## Verification

`corvint affected --base 993fe629` recommends the whole repository gate; under the owner's scoped
preference the runs were: `go test ./internal/contextindex` (whole package), `go test ./tools/cem-trial`
(whole package, builds `corvint`), `go test -run 'Context|Query|Eval|TaskContext|SlotWeight|Lexical|Anchor|Dogfood'
./cmd/corvint`, `go vet` over the three packages, the development benchmark three times, the chi
orientation corpus three times, and the transplant probes three times. `make gate`, the exhaustive
`./...` test and `interop/cem01-go` are `NOT_RUN`. The analyzer schema moves to `corvint-analyzer/101`
with its audit digest, because the audit pins every contextindex production source.

## What only the owner can run

Each is a ticket payload under `.agent-evidence/decision-0427-tickets/`: the rc.2 candidate and the
cobra and Corvint V1-0019 runs; commissioning blind-v7 with the authoring guide; the three-arm sealed
CEM run; development decoy generation and a within-repository out-of-scope probe set; the reviewer
projection and the human leg. They are created natively from the primary checkout, which holds the
journal this clone lacks. Decision 0427 holds the order and the wording each report can support.

## Dogfood

Pre-change `query` and `impact` receipts were retained under the private Git `corvint` directory
before the first edit. The dogfood loop outcome for this change is recorded in the section appended
below once the CEM is bound, checked and sealed.
