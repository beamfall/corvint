# Frozen replay counterexample and conservative caller retention

Requirements: AZU-V0-004, AZU-V0-009, AZU-V0-8901 and AZU-V0-8903.

The first exact-tree replay found a safety regression, not acceptance evidence. At merges
`39c031980`, `6c33df33b` and `ea770fa17`, the candidate omitted
`go:github.com/Beamfall/corvint/internal/dogfoodflow` and
`go:github.com/Beamfall/corvint/internal/localcompletion`. The baseline selected them through
`script/batch-regen.sh`'s declared callers. A new directory-construction check invalidated the
historical declaration and discarded those previously verified edges. The retained report digest
is `0aa6b8d293dea9315c1e66a27736c5e07aa728819efc4f61db3a119327e48db9`;
raw plans and witnesses remain outside the repository in `azu-new-evidence/replay-before-safety-repair`.

The repair keeps caller edges admitted by the earlier literal/shell-expansion completeness check
when only the new directory-construction check fails. Both the invalid-declaration and inbound
unknown remain. The broader checker does not accept the declaration or hide its new caller; it
preserves verified selections while abstaining about completeness. Declarations that already
failed the old guard keep their existing refusal behavior. The whole existing shell package and
`TestConstructedDriftKeepsVerifiedCallers_AZU_V0_8901` exercise this distinction.

Authority checks also distinguished two non-callers: `repin_test.sh` only matches `make -n` output;
`dogfood-change_test.sh` copies the script into a confined fixture and never runs the original site
against repository data. They are mentions of those original sites. Existing scope assertions stay
intact, and the repository acceptance probe separately requires the confined fixture test to stay
selected when the original script source changes. Scope declarations govern fixture data reads;
this is not permission to drop an actual script-source reader.

The repaired exact-tree replay and final test results are recorded below when produced. Missing
historical full-suite/mutation observations remain NOT_PRODUCED and cannot qualify AZU-V0-009.

The first repaired replay retains every baseline selection on all eight merges. Its report digest
is `e7eb15ff8aaf9da25c6bf00321661676940618a73b6457c4986cc862b6f86b07`. Full-suite and mutation
cells still say NOT_PRODUCED. The safety comparator also rejects a parent test that disappears
inside an otherwise observed unit; a unit-level completeness statement cannot conceal that gap.

Whole-package functional checks passed before the calibrated incremental-selection budget failed
at p95 225.486 ms, 5.77 calibration probes against the unchanged 5.00 bound. The ownership matcher
now rejects exact-path and literal-prefix misses before allocating its general matching state;
existing wildcard cases and additional zero-component `**` cases retain the same semantics.
Runtime caller work also exits immediately for graphs without TypeScript. The existing budget is
remeasured; its limit and assertions are unchanged. This repair is confined to observed overhead
from the ownership declarations and inbound closure.

The unchanged owner-pattern grammar cases and incremental-selection budget passed together
(`selection-budget-repair.log`, 66.059 s total test runtime). No post-repair percentile is claimed:
the passing invocation did not emit its suppressed test log. Final whole-package verification and
the replay at the final implementation revision follow; historical safety observations remain
external requirements.


## Final measured outcome

The final implementation is `2eaaf9fba87340641ead2ea37821f37c0e6a85e1`. The exact-tree report
`azu-new-evidence/replay/report.json` has SHA-256
`d74a41bb4aef61cb63ba764f992044568853ca015375fcc44e92ae3ebbf0b8f7`. Its candidate binary digest
is `6e0f066e015f426699408ea7ac96adbdb6691fb0058f6433c2dc2fd4e7669ee3`; its exact-base binary
digest is `064bdab15d12c4af649bf4c75a3cd15328cd28235303acda86fa2453477fa3d1`.

| Merge | Unknown before / after | Selected before / after | Test count after | Wall ms after |
|---|---:|---:|---:|---:|
| `8e2c13aa9` | 92 / 87 | 339 / 563 | 3534 | 5902 |
| `39c031980` | 84 / 73 | 66 / 181 | 1489 | 5352 |
| `d436dff76` | 99 / 95 | 261 / 470 | 3017 | 6288 |
| `6c33df33b` | 83 / 80 | 185 / 390 | 2606 | 5356 |
| `ea770fa17` | 63 / 78 | 75 / 181 | 1542 | 4461 |
| `24d8c04f9` | 79 / 68 | 38 / 118 | 1264 | 4767 |
| `f13f52b16` | 86 / 82 | 46 / 131 | 1299 | 4942 |
| `9fdc2f02e` | 79 / 69 | 36 / 113 | 1243 | 4500 |

Every baseline-selected unit remains selected in these eight observations. The raw report names
all removed unknown entries and added selections; it does not establish unseen-reader safety.
Every historical safety cell is NOT_PRODUCED because bound full-suite and changed-line mutation
receipts are unavailable. Every merge still has unknown entries, and `accepted` is false.
Aggregate zero-unknown qualification depends on the sibling slices and historical evidence; this
lane does not rewrite their historical trees or declare AZU-V0-009 accepted.

Whole existing tests ran through the heavy semaphore, one package per command. Shell, replay
checker, sessiontest, capcatalog and declaration-compat-check passed. The affected package's
functional checks passed, but the whole run failed the unchanged incremental-selection budget:
p95 216.843830 ms, 5.75 probes against 5.00, total 353.797 s. Its log digest is
`01d28aea31b2095e114b953f9c9087c77e8a032492eaf068ec771aad04fed4b0`.
Focused profiled budget runs passed for both candidate (57.992 s) and exact baseline (176.283 s).
Their CPU samples are dominated by the existing path-token matcher (53.44% and 63.65% cumulative
respectively). These observations neither explain the whole-run failure nor establish a speedup:
quiet-host whole-package qualification remains pending. The candidate profile digest is
`ff44f46bd4a4143996e5ebc24a54382d132d33d1b274a2ebc5f17ee1de9f4493`; baseline is
`d13b14ffb983a52901bd0d7e22f813a7138faae314c1d32d36ef2bbc76334059`.

The CLI whole run failed (482.279 s), retained at digest
`575d40eae5d7b9421516faa8c21bab08f9245a2bb70a469ac486e6775bbce3a7`. Most failures report the
mandated scratch tree's outer Git discovery or unsafe group-writable parent. One additional frozen
assertion, `TestAFUV1024StrictAndCoverageBytesUnchanged`, differs only in graph identity: the
fixture declares real Makefile targets, so the integrated driver adds graph vertices. Omitting
those active vertices from the digest would give different graphs the same identity. The charter
does not authorize changing this golden; no golden or assertion is changed. This inherited driver
identity conflict is retained as PARTIAL for orchestrator disposition. The separate existing
application-flow golden without driver inputs passes unchanged. The CLI host-node failures also
include non-repository probes under the same outer Git root. No product qualification guard is
relaxed to accommodate this host.

Spec indexing, capability catalog validation and line-citation validation passed. The local outcome
is `failed`, preserving both whole-package failures. Independent review, native completion,
publication, a seal, and historical runner observations are not produced by this lane.

The terminal CEM pre-flight caught definition syntax the requirement index had reduced to table
rows: the inherited bold title form and the lane's em-dash form were not executable obligation
leaders. The spec now uses the existing `- **ID:**` grammar throughout, retaining every ID and
clause body, and regenerates the requirement index before binding. This repairs evidence
enumeration rather than changing acceptance or promoting the experimental spec.
