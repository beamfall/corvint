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
Durable evidence-record content belongs in `docs/BUILD-LOG.md`. Follow the log's bounded reading and archival guidance; short per-task write-ups that back a
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
