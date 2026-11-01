# Spec-driven development

Corvint uses specifications to preserve human intent while code, generated knowledge, and agent
implementations change. The specification is not extra prose around the code. It is the reviewable
contract that states what must be true, what is deliberately excluded, and what evidence can promote
a claim from proposed to delivered.

## Authority

Authority descends in this order:

1. repository-owner accepted product intent and safety policy;
2. an accepted version or capability spec;
3. executable conformance, tests, and pinned measurement evidence;
4. implementation and generated technical documentation;
5. model inference, repository history, and agent traces.

Lower layers may reveal that a higher layer is stale or contradicted. They do not silently rewrite
it. In particular, generated documentation and spec backfill remain proposals until a human accepts
them.

## Four linked artifacts

| Artifact | Answers | Required content |
|---|---|---|
| Product contract | Why should Corvint exist? | user job, boundary, outcomes, kill criteria |
| Capability spec | What exactly must this slice do? | stable requirement IDs, inputs/outputs, failure modes, non-goals |
| Implementation plan | How will this repository deliver it? | file ownership, order, resource limits, rollback |
| Evidence record | What has actually been demonstrated? | tests, conformance, benchmarks, independent review, known gaps |

One document may fill more than one role when the scope is small. The roles must remain distinguishable.
Durable evidence-record content belongs in a new `docs/build-log/` entry file (decision 0423); entries from
before 1.0 are in the `beamfall/corvint-history` archive (decision 0587, `docs/build-log/README.md`); short per-task write-ups that back a
single completed change accumulate as tracked, uncited files under `.agent-evidence/` (for example
`.agent-evidence/CARVEOUT-summary.txt`) and carry no authority beyond what they cite.

## Status model

Intent and delivery are separate axes.

Intent status:

- `draft`: incomplete or still under design;
- `proposed`: reviewable and complete enough to challenge;
- `accepted`: explicitly approved by the repository owner;
- `rejected` or `superseded`: retained as history but not authoritative.

Delivery status:

- `not-started`;
- `experimental`: implementation may exist, but its contract or value is not proven;
- `implemented`: all in-scope requirements have executable evidence;
- `validated`: the named external or outcome gate has passed;
- `failed`: a binding gate failed and remains visible;
- `deferred`: frozen; spec and code are kept unchanged, get no new work, and are neither advertised
  nor built into a release archive or install. A `Disposition:` header names the return condition,
  and reopening needs a recorded decision.

`implemented` never means useful in the market. `validated` names the exact gate and evidence. A
failed held-out evaluation cannot be renamed development success.

A spec header's `Intent status:` and its Agent digest `Status:` hold the current status only,
optionally followed by the pointer `(history: REQUIREMENTS.tsv acceptance columns and
docs/build-log)`. A change that accepts or amends requirements records that in each requirement's
marker and in its build-log entry; it does not append to the header (`SRG-V0-011`). The Agent digest
`Status:` line contains the header `Intent status:` value verbatim and the `Delivery status:`
value, normally as `- Status: INTENT / DELIVERY`; a clause kept in an older header is therefore
repeated in its digest, and neither field takes a new clause (`SRG-V0-970`). Older headers
keep their tails, frozen: `docs/specs/INTENT-STATUS-FROZEN.tsv` records each such header's length
and the requirement IDs it names, and every `make spec-requirements` mode refuses a header or digest
`Status:` that names a requirement ID its frozen text did not, or an `Intent status:` longer than
160 characters and its frozen length, naming the spec and the repair (`SRG-V0-980`). A tail may be
cut back without touching the frozen file.

To amend an accepted requirement, edit its own definition line: rewrite its leading note to name
the amendment and its anchor, for example `(accepted, decision NNNN; amended under the owner's
standing auto-accept (YYYY-MM-DD); V1-NNNN)`, which `REQUIREMENTS.tsv` carries as `standing` with
every anchor in `acceptedBy` until the owner reads the amendment back (`SRG-V0-981`). Two changes
that amend non-adjacent requirements of one spec then touch different lines and rows and merge
without a conflict; amendments to adjacent definition lines (or adjacent `REQUIREMENTS.tsv` rows)
still conflict in those lines only, and resolve by keeping both notes and regenerating. The same
holds for `Makefile` targets: each phony target is declared on its own `.PHONY:` line next to its
rule, never appended to a shared line (`SRG-V0-982`); two targets added at the same spot still
conflict there. `script/spec-requirements-conflict_test.sh` replays both merges and the adjacent
limit (`SRG-V0-983`). Renaming a spec moves its frozen row to the new path in the same commit; that
is the only edit the frozen file takes.

A qualified `Delivery status:` header (more than one word, for example `experimental (file transport
only)`) appears verbatim in the digest `Status:`; the digest may add only a new sentence after it,
never a `;`, `,` or further words, so a stale truncated header is caught (`SRG-V0-014`). A one-word
header may be elaborated.

Requirement acceptance mode. Since the owner's standing auto-accept of 2026-10-11, `accepted` on a
spec no longer separates intent the owner read from intent an agent wrote under that approval, so
each requirement records its own mode in the leading parenthetical of its definition line, and
`docs/specs/REQUIREMENTS.tsv` carries it in the `acceptance` and `acceptedBy` columns
(`SRG-V0-009`). The modes are closed:

- `owner`: the parenthetical says accepted or approved, or names an owner decision, and names a
  decision number or a date, for example `(accepted, decision 0506)`;
- `standing`: the parenthetical names a standing approval, for example `(accepted under the owner's
  standing auto-accept (2026-10-11); V1-NNNN)`; the word `standing` decides whatever else it says;
- `unrecorded`: anything else, including no parenthetical or a marker after the text; the
  requirement inherits the spec header's intent status.

Accepted forms. A definition line is `` - `ID`: (note) text ``, `- **ID:** (note) text`,
`- **ID.** (note) text` or `ID. (note) text`; a note between the id and its colon, as in
`` - `ID` (V1-NNNN): text ``, is not a definition, and `make requirement-definitions-check` names
such a line when the id has no other definition (`RDG-V0-022`). The acceptance note is the first
parenthetical after the id, in one of these forms:

- `owner`: `(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`;
- `standing`: `(accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN)`.

`make spec-requirements` refuses three near misses, naming the line and these forms
(`SRG-V0-042`): an auto-acceptance note without the word `standing` (`(accepted, owner lane
auto-acceptance 2026-10-11; ...)`), an accepted or approved note with no decision number or date
(`(accepted, V1-NNNN)`), and an unparenthesized note (`accepted, decision NNNN. Text`).

A lane writes `standing` for its own requirements, never `owner`. A `standing` requirement becomes
`owner` only when the owner reads it back: the marker is edited to name the read-back in the same
commit as the build-log Owner acceptances entry that records it. `make spec-acceptance-audit` lists
the requirements not yet read back, by spec, and reports a spec whose delivery is `validated` while
it still holds a `standing` requirement as `NOT_PROMOTABLE` (`SRG-V0-010`). The mode is a record of
what the marker says, not an approval: it is derived from the marker text and carries no authority
of its own.

## Required capability-spec shape

Every substantive capability spec contains:

1. owner, date, intent status, delivery status, and authoritative inputs;
2. affected user and measurable job;
3. verified current state with repository paths or pinned evidence;
4. numbered requirements using a stable capability prefix, under one heading that is exactly
   `## Requirements`, each as a `- \`PREFIX-NNN\`: text` line (continuations indented two spaces);
   the OCM intent reader (`ocm prepare`, `docs/specs/ocm-v0-dogfood.md`) sees only that form, so a
   numbered heading or a `**PREFIX-NNN.**` paragraph is invisible to dogfood coverage;
5. explicit non-goals and simpler baseline;
6. trust boundary, resource limits, failure modes, and fail-open/fail-closed choices;
7. deterministic acceptance criteria and a testing matrix;
8. rollout, rollback, compatibility, and maintenance/drift rules;
9. a traceability table from requirement IDs to implementation and evidence;
10. unresolved decisions and promotion or kill criteria.

Requirements describe observable contracts, not preferred internal structure, unless the structure
is itself a safety or interoperability property.

## Change loop

```text
human intent
  -> accepted or explicitly experimental spec
  -> implementation and tests
  -> deterministic verification
  -> independent review where risk requires it
  -> measured outcome
  -> spec/status update
```

For a behavior or wire-contract change, update the spec before or in the same commit as the code.
Tests cite requirement IDs in test names, docstrings, or the spec traceability table. Failed and
`NOT_RUN` evidence stays visible. An agent may repair implementation to meet an accepted spec; it
must request human review before changing accepted intent.

A spec amendment committed before the commit that adds its test may cite that test in its
traceability table: `make traceability-tests-check TRACE_RANGE=BASE..HEAD` resolves every commit's
rows against the change head, so the spec-first commit passes once the test lands and a test that
never lands still fails (`TTG-V0-950`); without the range the index check names that route.

Evidence that names a test in a nested Go module (a directory with its own `go.mod`, such as
`tools/local-authority`, `interop/cem01-go` or `benchmarks/snapshot-reader`) names the module
directory, because `go test ./tools/local-authority` from the repository root fails with `main
module does not contain package`: cite `(cd tools/local-authority && go test -run '^TestName$' .)`.

Requirement IDs are numbered by hand, so parallel lines of work can take the same next number.
A lane takes IDs only from the block its charter assigned, which `make requirement-ids-free
PREFIX=... RANGE=...` checked free on `origin/main` and on every unmerged local branch and worktree
(`ASD-V0-952`). After staging a spec that adds an ID, and again before its final commit, it runs
`make requirement-id-collisions`, which names each ID this checkout and another open local branch
or worktree define differently (`ASD-V0-950`); an orchestrator runs the same check in the batch
checkout before it integrates each lane. A collision found at merge is refused by the generator
(`SRG-V0-547`) and costs a renumber of spec rows, tests and build logs.

`docs/specs/REQUIREMENTS.tsv`, `docs/specs/INDEX.json` and the `docs/specs/README.md` spec table are
generated. A change edits its spec, stages it, and runs `make spec-requirements`, which regenerates
the index and projects each header's intent and delivery status and its digest claim onto the two
row files; `spec-requirements-check` fails when any of the three is stale (`SRG-V0-012`). The
other INDEX.json fields and README.md cells stay hand-owned. The index carries no line numbers and
orders rows by definition position, so changes to different specs or different sections of one
spec merge without conflict (`SRG-V0-015`, `SRG-V0-016`). Changes whose new rows are adjacent
still conflict there; resolve the spec and regenerate the index, never hand-merge it
(`SRG-V0-013`). The generated rows are projections of
the spec headers and never a source of intent.

## Backfill and generated documentation

Legacy repositories rarely begin with accepted specs. Corvint therefore separates:

- `PROVED`: mechanically established structure or identity;
- `OBSERVED`: behavior seen in a pinned execution;
- `INFERRED`: a cited proposal requiring review;
- `CONFLICTED`: authoritative or observed sources disagree;
- `UNKNOWN`: evidence is absent or outside the bounded search.

Backfilled specifications start as `draft` or `proposed`. Passing tests do not prove exhaustive
behavior, and source code does not establish product intent. Corvint preserves orphaned code and
unknown behavior as queryable gaps instead of inventing a specification.

## Proportionality

## Proposed amendment: solo falsifiability

Owner acceptance required. A new proposed spec MUST NOT use a promotion or kill gate that a
single-owner repository cannot close. A gate requiring an external actor, independent organization,
or unavailable corpus MUST be labelled `EXTERNAL_DEPENDENT`, remain outside promotion/kill logic,
and report `NOT_RUN` until its dependency exists. This amendment does not weaken accepted specs or
convert owner-labelled evidence into independent validation.

A typo or mechanical refactor does not require a new spec. A new wire field, user-visible behavior,
security boundary, compatibility promise, evaluation claim, adapter contract, or autonomous action
does. Extend an existing spec when it already owns the behavior. Create a new spec only when the
capability has an independent user outcome or promotion gate.
