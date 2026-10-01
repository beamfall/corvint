# Decision 0427 — Close the three unproven README rows: root causes, amendments and the owner-commissioned runs

Date: 2026-10-01. Status: proposed (owner acceptance pending for every amendment named below);
the findings and the development measurements are recorded. Tickets: five payloads under
`.agent-evidence/decision-0427-tickets/`, to be created natively from the primary checkout (this
clone carries no `.git/taskman` journal); their IDs are recorded here once assigned. Build-log entry: `docs/build-log/2026-10-01-unproven-rows-root-causes.md`.

## Context

`README.md`'s "Status, stated plainly" table carries three rows that `1.0.0-rc.1` cannot defend:

1. **Does CEM help a reviewer?** "Unproven. A five-pair pilot scored mean missed evidence of 0.90
   for control and 0.86 with CEM."
2. **Retrieval quality.** "The latest held-out attempt beat the exact-search baseline on top-5
   (0.571 vs 0.343) and met the abstention and latency bars, but returned forbidden results on 7 of
   36 must-exclude checks."
3. **Core jobs.** "The rc.1 evaluation failed overall. Orientation missed critical test files in
   3/20 cases on go-chi/chi and 1/20 on Beamfall ... the Corvint run aborted before scoring."

The owner asked for the root causes and for the plan that makes each row honestly pass or be
replaced by a claim the evidence supports. Four independent reviews were taken (experimental
design, retrieval evaluation, code-review product, and a prior-art survey); their verdicts are
summarised in the build-log entry. This record keeps the decisions.

## Findings

1. **The pilot did not measure CEM.** Its treatment arm received a `cem begin` map in which every
   hunk was `unknown` with reason `no-evidence`, the `cem status` worklist of that map, and
   `cem report`: a worklist's wording, with zero retrieved evidence. Its gold included test files
   the change itself created, which do not exist at the base revision the lane runs at and which
   the reply grammar forbids citing; three of the five changes (c03, c04, c05) scored miss 1.0 in
   both arms on exactly such files, and both arms lost to the naming-convention guess (0.70). The
   McNemar test averaged five repeats per change first, so it saw zero discordant pairs over fifty
   lanes. No human reviewer was involved. The result is uninformative about the question, not
   evidence of a null effect.
2. **The orientation misses were two defects in one pass, and one of them was introduced after
   rc.1.** On the go-chi/chi corpus (consumed by run-001, development since decision 0425) the
   current `main` scored 4 treatment-only critical misses in 3 of 20 cases, against rc.1's 3 in 2:
   the V1-0431 counterpart admission spent its three-counterpart cap on the counterparts of weaker
   sources before the strongest source's turn (`record response status when flushing` still
   missed `middleware/wrap_writer_test.go`), and its last-position victim rule evicted the
   packet's strongest lexical test when it was the only unprotected one left (`Don't duplicate
   methods in Allow: header for 405 responses` newly missed `mux_test.go`). The third case,
   `Replace "interface{}" with "any"`, reaches five of eight modified files because the tokeniser
   reduces the quoted literal to the common word `interface`; the opt-in TCP-V0-022 anchor field
   reaches all eight. The Corvint self-run abort is already fixed on `main` (V1-0432, V1-0433) and
   waits only on the next immutable candidate.
3. **The seven forbidden hits are two classes with two different owners.** Four were same-path
   siblings admitted by a 60%-of-path-top score ratio, a constant fitted to one sample of the kind
   GPK-V0-039 forbids; the development corpus had no must-exclude selector to catch it. Three were
   expected-`OUT_OF_SCOPE` tasks answered `READY` because one identity word plus one word from a
   declaration's context window clears the two-word floor, and one of those three is a misspelling
   of a behaviour the repository does have, which is a case-authoring defect rather than an engine
   one. The sealed packet's case texts are not in this repository, so the engine changes below are
   validated on the development corpus and on probes, never on the consumed partition.

## Decision

1. **Context packet (Core job 1).** TCP-V0-004 is amended to a rank-relative counterpart rule with
   no fixed cap: a counterpart displaces only an unrelated lexical test the task matched more
   weakly than the counterpart's source, weakest victim first, and never grows the packet or evicts
   a source. TCP-V0-022's anchor field is on by default with `CORVINT_CONTEXT_ANCHORS=off` as the
   exact pre-amendment bytes; decision 0333 item 3 is superseded on that point and its 0070-ladder
   condition is replaced by the measured orientation evidence below. The full chi orientation
   corpus scores 0 treatment-only critical misses over 20 cases under both amendments, against 4
   on `main` and 3 at rc.1; the chi corpus is development evidence and the held-out qualification
   remains the owner's cobra and Corvint runs on the next candidate (V1-0019, PRS-V1-008).
2. **Retrieval (row 2).** GPK-V0-040 gains the sibling explain-away rule (set containment in the
   query's own words, with four-byte prefix answering, never a score ratio). On the 24-case
   development corpus it leaves top-5 at 1.0, recall at 1.0 and critical misses at 0, raises
   byte-weighted precision from 0.700 to 0.713, and removes the blind-v6 decoy
   `active_help.go:AppendActiveHelp` from beside `GetActiveHelpConfig`. A window-licence bar for
   the relevance floor (a context window licenses a declaration only at the three-term bar the
   ranking already holds a window-only candidate to) was tried and is not adopted: it also left
   the development corpus unchanged and turned the within-repository near miss `render command
   docs as JSON files` (cobra, which has markdown, man, ReST and YAML docs and no JSON) into a
   correct abstention, but it moved the cross-repository transplant probes only from 23 of 45
   `READY` to 22 of 45 and withdrew `Original consumer` in the prove-bundle fixture, a dependency
   question the repository answers. Transplanted tasks are mostly answered by declarations that
   carry two task words in their own names, which no lexical floor can refuse. The honest
   reading is that the sibling class is closed on development evidence and the out-of-scope
   class needs case design before another rule: blind-v7's out-of-scope probes must be
   within-repository near misses reviewed by a second person, not misspellings and not
   transplants, and the development corpus needs such probes first (the decoy ticket payload).
3. **CEM reviewer trial (row 1).** CRT-V0-004 restricts gold to files that exist at the base
   revision and records the rest as `gold_unreachable`; CRT-V0-005 adds the `seeded` arm, whose
   prologue carries the evidence `corvint context --subject` retrieves at the base revision for
   each presented file, labelled a suggestion and never cited, so `treatment` becomes the format
   control between `control` and `seeded`; CRT-V0-007 estimates every arm against control and
   adds the lane-level McNemar; CRT-V0-012 decomposes the seeded arm into retrieval (how much
   gold the suggestions carried) and adoption (how much of that the agent cited); CRT-V0-008 is
   re-powered at about 55 pairs with two repeats for a 20% relative reduction from a corrected
   control miss of 0.50, and the gate reading moves to "the interval excludes 0 and does not
   exclude 20%". The scorer keeps the pilot's verdict wording until the owner accepts that
   reading. The pilot report and manifest stay as history and are not rescored.
4. **What only the owner can do, and in what order.** (a) Build the `1.0.0-rc.2` candidate from a
   `main` carrying these changes and run V1-0019 on spf13/cobra and on Corvint at the fixed loop
   target; row 3 changes to what that run supports. (b) Commission blind-v7 under GPK-V0-073
   with the authoring guide in the blind-v7 ticket payload: five repositories never indexed by Corvint, about thirty
   positive cases with one to three reviewed decoys each, ten out-of-scope probes that are
   within-repository near misses, a preregistration digest, one run, and must-exclude counted
   over all emitted results; row 2 changes to that run's numbers, pass or fail. (c) Select a
   held-out CEM set with the amended rule, excluding the pilot, sized by amended CRT-V0-008, and
   run it once in all three arms; row 1 changes to the agent-reviewer result with its interval and
   its retrieval/adoption decomposition, and says "agent reviewer" until the human leg in (d) runs.
   (d) The human within-subject leg: about eight reviewers over about twelve sealed Corvint changes
   with known evidence gaps, counterbalanced, primary outcome gap recall in a twenty-minute box,
   with sealed task counting; only that leg can make the row say "helps a reviewer".
5. **README.** No row changes in this decision: the rows describe rc.1's evidence truthfully and
   the new evidence is development evidence. The build-log entry carries the wording each
   commissioned run would support and the wording for a null or negative result, so the row is
   written from the report and not from hope.

## Consequences

The three rows now have named defects, landed fixes with regression tests, development
measurements that bound the fixes' effect, and one owner-commissioned run each whose report
rewrites the row. Nothing here is a held-out pass: the chi corpus and the development corpus are
development data, the transplant probes are a diagnostic, and the sealed partitions this cycle
needs do not exist yet. The analyzer schema advances from 100 to 101 because the audit pins every
contextindex production source.

## Rollback

Each amendment reverts alone: the counterpart rule and the anchor default in
`internal/contextindex/taskcontext.go` and `context_anchors.go` with their TCP-V0-004 and
TCP-V0-022 text; the sibling rule in `internal/contextindex/eval_query.go` with its GPK-V0-040
text (GPK-V0-039 carries only a considered-and-not-adopted note); the trial amendments in `tools/cem-trial` with their CRT-V0
text. The pilot report, the rc.1 run-001 evidence, and the blind-v6 record are untouched by any
rollback, and no commissioned run is invalidated by one because none has been drawn.
