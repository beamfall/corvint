# Decision 0068 — The context packet withholds its rows on an unsupported conjunction of the task's names

Date: 2026-09-05. Status: accepted 2026-09-12 by the amendment below (experimental contract; promotion held). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies `docs/reviews/b1-retrieval-breakthroughs.md` section B4 (answerability from rare-term conjunction absence) to the `context` verb only.

## Context

The `context` packet never abstained: the Agent Retrieval Bench `v2_abstention` release (50
natural no-gold samples, 32 counterfactual) scored its `selective_success` on the natural stratum
at 0.0, and the B4 review names the reason: a word-count floor is a threshold, and the ARB
authors report that thresholds calibrated on counterfactual controls do not transfer to natural
no-gold cases. Section B4 asks for a proof of absence instead: compute the task's specific terms
and test whether any single source carries their conjunction.

Two shapes in the bench's own tasks bound the design. Long issue texts carry thirty to ninety rare
plain words each, so a conjunction over plain words is met by chance in every repository and
proves nothing; and a pasted trace names symbols the tree does not yet define (the undefined
symbol the build fails on, the test the fix adds), so an absent name alone is a feature as often
as an absence. `query` is oracle-pinned; `context` is Go-only under
`docs/specs/task-context-packet-v0.md`.

## Decision

1. `TCP-V0-016`: the specific terms are the task's names only, backticked tokens and camel-case or
   underscore identifiers, below a cut of one sixteenth of the indexed sources (never below two);
   a source carries a name as an identifier, as one lowercased body term, or in its path, and a
   tracked path the index does not read (`.java` is not an indexed suffix) still makes the name
   known.
2. The conjunction is over the known names; the required support is all of them, or all but one
   (two) from three, and both are reported. The slot rows are withheld in exactly one case: two
   or more names, none known, and no row admitted by a relation (a mentioned path, its pair, a
   definer, an importer). A relation row is evidence of a locus a vocabulary absence cannot
   overrule: six `v2_code2test` positives (gin-gonic/gin, spring-boot, tokio) named a changed
   file whose identifiers the index could not see, because the bench corpus truncates each file
   at about 8 kB and Corvint does not index `.java`, and the base answered them through `mentioned`
   and `pair`. A single unknown name answers (a feature is asked for as often as an absence is
   described). A second withholding case, a refuted name
   beside three or more known names no source carries two of together, was measured in the same
   session and withdrawn: on `v2_trace2code` it abstained on four pallets/click positives (three
   of them hits before), because the source that joins the names, `tests/test_options.py`, is
   over the index's source size bound and has no postings; the arm's `hit@k` fell 0.901 to
   0.861. Scattered known names are an index gap as often as an absence, so they do not withhold.
3. The verdict lives in `coverage.answerability` with every specific term's source count, the
   support found and required, a reason naming each name no source carries, and on withholding
   the `nearest_claims` (the first withheld rows, what each supports and lacks). Reserved rows
   stay; `state` keeps TCP-V0-006's two values; the bench's `context` arm reads the verdict as an
   abstention. No number is calibrated on the bench: the cut is a fixed fraction and the pair is
   the smallest conjunction.

## Evidence and falsifier

The promotion gate written into `TCP-V0-016`: on `v2_abstention` the `context` arm's
`selective_success` on `natural_no_gold` must exceed 0.0, while `v2_trace2code` and
`v2_code2test` `hit@k` and `recall@20` fall by no more than 0.01 and the positive abstention rate
stays below 0.05. The measured tables are recorded under `docs/BUILD-LOG.md` when the run lands;
this record moves to accepted on a pass and is withdrawn otherwise. Two bench artefacts are
disclosed with the numbers: the query JSON's label values (`counterfactual_wrong_repo`,
`go_test_package`, `local_test_reproduction`) read as refuted names, which inflates the
counterfactual stratum and neutralises the refuted-name guard on trace2code, so the natural
stratum is the reading that counts.

## Consequences

- A packet can now hold zero slot rows for a task the tree cannot answer, with the reason and the
  nearest partial matches beside it; every other packet gains the answerability member and is
  otherwise unchanged.
- `tools/retrieval-bench` reads `coverage.answerability.verdict` and records the verdict in the
  arm's `state`.

## Amendment 2026-09-12: accepted as the current contract; historical discrepancy reconciled

Authority: delegated owner call (the repository owner delegates open calls to be decided,
recorded and applied). This amendment documents the current Go behaviour; it changes no
code, threshold, wire or golden file.

1. The whole-subset falsifier above passed twice on development data: the original 2026-09-05
   lane run (natural 4/50, counterfactual 10/32, trace2code `hit@k` and `recall@20` each −0.010,
   one positive abstention of 101; `docs/build-log/2026-09-05.md`) and the registered paired guard
   on source-pinned base `300e1813` and its repair (same 4/50 and 10/32, 14 no-gold wins, the
   same single trace2code loss;
   `docs/reviews/reliability-research-2026-09-05/uncertainty-development-guard.md`). By this
   record's own rule the decision is accepted and the withholding stays on. `TCP-V0-016` stays
   experimental: all evidence is `v2` development data, the post-run subgroup HOLD on that loss
   (trace2code fold A, gin-gonic/gin) is not cleared, and 4/50 is not reliable abstention.
   Advertising or promoting the verdict as delivered abstention needs a non-development
   qualification under decision 0070 that clears the HOLD; the withdrawal clause above still
   governs a failed qualification.
2. The accepted contract is the implementation after two repairs, and `TCP-V0-016` now states it
   exactly: (a) only the seven named relations (`mentioned`, `pair`, `definition`,
   `reverse-import`, `reference`, `cochange`, `sibling`) rescue an unsupported conjunction; a
   `test` row (`TCP-V0-015`, decision 0067), which can derive from lexical anchors alone, the
   lexical fill, documentation, reservations and any unlisted kind do not (repair `09a1a55b`);
   (b) the required support is `min(known, 2)` (the support-disclosure repair), not "all but one"
   for four or more known names; (c) the verdict `supported` means the withholding rule did not
   fire, including one unknown name and scattered known names; it is not a claim that any source
   carries the names, and `known`, `support`, `supporting_sources` and `reason` carry the
   evidence (invariant 2).
3. Decision 0078's 0/50 and 0/32 are explained at source level. Its pinned `0f9b40bb` counted
   every row kind except `lexical`, `documentation` and the two reservations as a rescue
   (`git show 0f9b40bb:internal/contextindex/taskcontext.go`, `relationRows`), so the `test` rows
   of decision 0067 rescued zero-known tasks; the paired guard's base `300e1813` carries the same
   rule and reproduces 0/50, and the seven-kind repair restores 4/50. The exact source of the
   original lane run (a scratch tree described as the decision-0066 tree without the verdict) is
   not recoverable; the inference that it lacked the `test` relation is not confirmed and is not
   pursued further. The historical numbers stand as reported; no claim of exact causal
   reconstruction of that run is made.
4. The reconciliation needs no answerability signal beyond the lexical conjunction; the parked
   non-lexical signal ideas stay ideas.

## Rollback

Delete the answerability section of `internal/contextindex/taskcontext.go`, the `answerability`
field and its two call sites in `TaskContext` and `packet`, and the bench's verdict read; no
wire, snapshot or oracle migration is required.
