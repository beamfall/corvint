## 2026-09-29 HNE-V0, V1-0474: heading navigation pilot

The owner requested implementation and evaluation of the PageIndex-inspired recommendation:
structure-aware navigation over a frozen documentation slice before changing production retrieval.
The experiment lives in `tools/heading-nav-bench`; production query, corpus format, learning and
frozen release evaluations are unchanged. The owning proposed/experimental contract is
`docs/specs/heading-navigation-experiment-v0.md`.

### Frozen comparison and evidence

Source commit: `85cb2889444ad41ac001876fca2b0000c76a09a7` (origin/main at branch creation).
Six literal Markdown documents, eight author-visible questions, 13 full-span evidence obligations,
and 6,000 serialized bytes per question/arm including provenance, outlines and the question.
Both arms use actual native corpus lexical document ranking. Baseline consumes document prefixes;
candidate reorders section bodies by lexical overlap with heading ancestry and ancestor introductions.
It can inspect body terms offline. This is not adaptive LLM traversal or an outline-only navigator.

Fixture SHA-256: `d2076ac447ba490883cee12c6075f69df5179b00ab438ade63b9c357e48b6a8b`.
The original pretrial fixture hash and the independent gold-span correction are retained in the
harness README. Review also required a fresh process per question/arm and a frozen grading rubric,
preventing cross-case context leakage. These repairs preceded every trial. The prompt and rubric
hashes, source hashes, packets and outcomes are in `tools/heading-nav-bench/results/pilot/`.
No PageIndex source was copied and no dependency installed.

| Observed metric | Lexical + prefix baseline | Heading expansion |
|---|---:|---:|
| Full required evidence spans read | 3 / 13 | 7 / 13 |
| Fully answered questions | 4 / 8 | 5 / 8 |
| Partial-credit answer score | 10 / 16 | 11 / 16 |
| Unsupported conclusions | 0 | 1 |
| Authority promotions / invalid citation ranges | 0 / 0 | 0 / 0 |
| Total serialized packet bytes | 47,664 | 47,704 |
| Reported input tokens, including harness context | 149,009 | 149,929 |
| Reported cached input tokens | 35,456 | 24,832 |
| Reported output tokens | 1,647 | 1,438 |
| Sum of isolated reader wall seconds | 70.950 | 63.320 |

Each of the 16 reader sessions used gpt-5.6-sol/low, the same prompt and 160-word answer limit,
no prior case or gold, and one packet. All finished without a tool event or invalid citation range.
These settings were supplied to Codex CLI 0.153.2; no spend cap is claimed. Usage above is the
CLI observation, not billed-token/cost telemetry, which remains `NOT_OBSERVED`. Reported reasoning
output tokens were 214 baseline and 83 heading (a subset of reported output, not added again).
Cache differences, one repetition and the weak prefix baseline preclude a speed/cost claim.

### Interpretation and limits

The extra complete answer was the explicit offline-binary requirement. Baseline could already answer
the holdout and license questions using partial/digest evidence despite missing the selected full
gold spans; retrieval recall is not answer correctness. Neither arm answered build-log correction or
typed corpus misses: native document ranking chose a different document first. Large generic
`Requirements` sections also remain a weak navigation unit. An agent using ordinary search and
targeted ranges may beat either arm; that stronger baseline has not been measured.

Independent semantic review found a safety regression: the heading reader generalized the corpus
provider requirement for native test observations into a requirement for every downstream task pass.
The packet did not establish that general claim. This remains an unsupported conclusion in the
report, with partial answer score 1; the baseline reader explicitly abstained on the missing specific
observation. Baseline generated-authority also had a citation precision issue: its cited range stopped
before the imported-trust sentence even though that sentence was present elsewhere in its packet.

The evidence supports keeping the experiment available for further evaluation. It does not justify
promoting this policy into Core. A next experiment needs independent unseen tasks, requirement-level
spans and a search-plus-targeted-range agent baseline, with observable full-task billing and outcomes.
Those are follow-up options, not implemented capabilities or accepted new scope.

### Verification and dogfood disposition

Used: native pre-change query and tracked-path impact (original receipts retained in the worktree's
private Git directory), actual native corpus Inventory/Build/Query, affected advice, baseline
`internal/doccorpus` tests (PASS, 272.195 seconds), focused harness tests, harness vet, documentation
checks and independent protocol/code/result review. The standalone pilot is the relevant frozen
evaluation; no production ranking or registered golden changed. Broader product evaluation,
mutation, external provider and UI routes are not applicable to this isolated harness.

Affected advice conservatively listed repository-wide commands and transitive Go dependents.
Repository-wide `make gate` and exhaustive commands are `NOT_RUN` under the owner's explicit
scoped-work preference; this is not equivalent coverage. Final enrolled checks retain exact-target
binding separately. At change start, `make dogfood-change` reported `NOT_PRODUCED`: no change/CEM
existed yet (`git-diff-failed`/`cem-map-not-produced`), intent input was not supplied to that initial
make invocation, and verification/outcome inputs were not yet available. Original report retained;
these initial misses are not represented as success. Final CEM/OCM and outcome are recorded through
the enrolled local workflow and seal, not inferred from the pilot.

Native Tasks ticket V1-0474 is recorded in the primary store. Draft publication is authorized;
merge is not. Keep the ticket open while integration/native completion remains pending.
