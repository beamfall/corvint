# Verified Absence Frontier V0

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: superseded by `docs/specs/change-frontier-v0.md`
Delivery status: superseded
Supersession note: its Stop-hook and acknowledgement-under-policy language contradicts
`CF-V0-005` and `CF-V0-027` and is not ported. Decision 0012 withdraws its imported
`0.70`/`0.50`/`0.25` outcome gates from the successor; the numbers below are historical only.
Authoritative successor: `docs/specs/change-frontier-v0.md`

## Agent digest
- Successor: `docs/specs/change-frontier-v0.md` is the authoritative replacement.
- Why: this draft's Stop-hook acknowledgement conflicts with `CF-V0-005` and `CF-V0-027`; decision 0012 withdraws its imported outcome gates, while its non-goals remain.

## Product claim

For a maintainer reviewing one agent-authored change against one pinned intent scope,
`corvint frontier` lists material hunks without a qualifying basis, intent IDs without a qualifying
change-and-test witness, and required tests without pinned observation. A harness Stop hook blocks
completion until each item is witnessed or explicitly acknowledged under policy.

V0 accepts one repository, one base revision, one target revision, and one Markdown intent scope.
Its output is an agent stop condition and a human review queue, not a general code-search result.

## Relationship to the product contract

This replaces the CEM-only 30-day experiment as Corvint's single launch bet. CEM and OCM remain the
portable evidence and obligation primitives; `frontier` is the user-visible decision they enable.
Proof-boundary repair remains a prerequisite, not a separate product trial. The experiment does not
advance to historical rationale, adapters, or distribution unless the frontier signal gates pass.

## Definitions

- **obligation universe**: the exact patch, the single pinned Markdown intent scope, required-test
  obligations declared by that scope, supplied pinned observations, exclusions, and one selected
  policy. Multi-contract discovery is outside V0;
- **witness**: immutable evidence that passes structural verification and a qualifying relation for
  one obligation. `cem-lexical-v0` qualifies only the narrow CEM hunk-evidence obligation by
  composing an allowed declared CEM basis relation with the lexical floor; bare lexical overlap
  supplies only a candidate;
- **frontier item**: an obligation that is missing, stale, unobserved, or outside the verifier's
  declared scope;
- **closed frontier**: no unresolved item in the declared universe;
- **verified absence**: the deterministic statement that no qualifying witness was accepted for a
  named obligation from the declared inputs and exclusions;
- **human assertion**: an attributable, immutable assertion that a later authenticated profile may
  permit under policy but never relabel as mechanical proof; current CEM/OCM wires cannot carry it.
- **structurally valid**: all CEM/OCM inputs pass their format and integrity verifiers;
- **policy pass**: every remaining frontier item has an explicit acknowledgement permitted by the
  selected policy;
- **frontier empty**: no frontier item remains; only this state may be described as closed.

## Requirements

### Trust boundary

- `VAF-001`: every result MUST bind the repository identity, base revision, target revision,
  canonical patch digest, governing contract identities, and exclusions.
- `VAF-002`: Corvint MUST distinguish `missing`, `stale`, `unobserved`, and `out-of-scope`. It MUST
  NOT collapse them into a confidence score.
- `VAF-003`: a closed frontier MUST be scoped to a named, finite obligation universe. The CLI and
  machine output MUST NOT state or imply whole-program completeness.
- `VAF-004`: unknown inputs, malformed evidence, drift, ambiguous relocation, unsupported Git
  objects, and verifier resource exhaustion MUST fail closed.

### Relevance floor

- `VAF-005`: a structurally valid supported CEM basis with an allowed declared relation that passes
  the narrow external lexical floor MAY witness the CEM hunk-evidence obligation as
  `cem-lexical-v0`. The relation proves typed lexical proximity, not semantic entailment. Bare
  lexical overlap, an OCM `linked` state, and an OCM lexical candidate cannot witness or close an
  intent, claim, test, or frontier obligation. CEM-only relevance MAY inspect legacy 0.1, but every
  OCM-consuming relevance or frontier path in V0 requires canonical CEM 0.2; OCM plus 0.1 is an
  unsupported context, not a relevance miss.
- `VAF-006`: identifiers MUST be derived from bounded syntax-aware or lexical rules, with stable
  language-neutral fallback behavior. Stop words, punctuation, generated hashes, and path boilerplate
  MUST NOT satisfy relevance alone.
- `VAF-007`: evidence that merely names its own CEM/OCM identifier, map path, or generated report
  MUST NOT witness the change that created that identifier or report.
- `VAF-008`: documentation and tests MAY carry `cem-lexical-v0` only under their exact declared CEM
  basis relation and the `producer-declared` authority class exposed in the canonical edge tuple;
  this witnesses only the CEM hunk-evidence obligation. It does not create claim, test, or intent
  closure. The current CEM/OCM wires cannot express authenticated attributable human authority, so
  no producer text may claim that class in V0. Evidence from the changed path alone cannot qualify
  because `cem-lexical-v0` requires an external basis.
- `VAF-009`: renames and mechanical changes MUST use byte- or syntax-proven rules. Every textual path
  admitted by CEM uses the same lexical fallback in WP3; inferred generated, vendored, minified,
  test, specification, suffix, and language classes have no V0 authority. Repository class policy
  is deferred.
- `VAF-009A`: an ordinary evidence span MUST be no larger than 512 bytes or 20 lines. A test claim
  with an empty body, an unconditional skip, or no pinned execution MUST NOT qualify as proof.
- `VAF-009B`: each witnessed intent obligation MUST reference at least one material hunk whose added
  bytes or final basename contain a non-trivial anchored term from that requirement. Material-hunk
  filtering excludes only the exact intent path and exact referenced claim paths. WP4 must define
  the caller-reported `test-report-matched-v0` qualification relation. That relation is non-closing
  under default policy and does not prove the claim or test; shared hunks or claims are evaluated
  independently for each obligation without upgrading their authority.

### Frontier command

- `VAF-010`: `corvint frontier` MUST deterministically report, in stable order: unwitnessed changed
  hunks, unsatisfied intent obligations, unobserved required tests, exclusions, and the exact next
  action for each resolvable item.
- `VAF-011`: default success requires an empty frontier and valid CEM/OCM inputs. Policy MAY permit
  named unknown classes, but the output MUST retain them and MUST NOT call the frontier empty.
- `VAF-012`: JSON output MUST be source-body-free, bounded, and suitable as one MCP primitive and
  one harness stop hook. Human output is a rendering of the same result, not a second computation.
- `VAF-013`: test observations MUST identify the exact command, revision, result, and available
  report artifact. A declared or discovered test without a matching observation remains unobserved.
- `VAF-013A`: exit `0` means frontier empty. A distinct non-zero code means structurally valid and
  policy pass with acknowledged items; another means unresolved frontier; invalid inputs retain a
  separate fail-closed code. JSON MUST expose all three states independently.

### Zero-ceremony evidence and history

- `VAF-014`: an agent-session producer MAY derive candidate CEM bases only from explicit, opt-in,
  task-scoped harness events recorded before the corresponding edit. Events are ephemeral by
  default. It MUST preserve an explicit no-evidence result and MUST NOT invent or backfill a read
  event after the edit.
- `VAF-015`: automatically derived candidates remain producer assertions until the normal CEM and
  relevance verifiers accept them. Session telemetry is local by default and is not a prerequisite
  for manual or third-party producers.
- `VAF-018`: the verifier, JSON contract, and fixtures MUST remain usable by an independent producer
  or consumer without Corvint retrieval, an LLM, a daemon, a database, or network access.

## Minimal delivery sequence

1. Deliver WP2 canonical patch binding, WP3 `cem-lexical-v0`, and WP4's caller-reported,
   non-closing `test-report-matched-v0` qualification; the current self-certified Corvint map must
   fail. WP5 frontier implementation cannot begin before all three, but WP4 delivery alone closes
   neither an OCM obligation nor the frontier.
2. Run a manual local frontier against 20 historical pull requests with sealed reviewer findings.
3. If signal gates pass, add one read-before-edit producer and one harness Stop hook, then run the
   prospective paired trial.
4. If must-use and overhead gates pass, add `corvint why` after at least 20 verified artifacts exist.
5. Add one specification adapter only after at least three pilot repositories report manual-ID
   friction. Evaluate a second adapter separately.
6. Add an independent consumer and GitHub distribution after the frontier wire stabilizes.

## Acceptance and kill gates

- Preserve the current Corvint dogfood map bytes, then freeze an edge-identical canonical CEM 0.2
  rebound. That rebound MUST fail `cem-lexical-v0` or the OCM candidate filter until its evidence is
  narrow, external, and lexically proximate. WP4 may add only its caller-reported, non-closing
  qualification; it cannot turn an OCM candidate into default closure. The rebound 12/12 map is the
  mandatory fabricated-fail fixture; the original 0.1-bound OCM is an unsupported LRF context and
  cannot satisfy this gate.
- The preregistered WP3 corpus MUST contain exactly 40 tuning and 20 sealed held-out immutable hunk
  or obligation subjects selected before evidence production. Held-out precision MUST reach at
  least 0.85 and critical-subject recall at least 0.90; zero
  denominators remain `NOT_RUN` and cannot promote.
- The initial historical cohort is 20 pull requests with reviewer findings sealed before Corvint
  runs. Recall is eligible findings intersecting at least one frontier item divided by all eligible
  findings. Precision is reviewer-actionable frontier items divided by all surfaced items. Promote
  at recall at least 50%, precision at least 70%, and median authoring overhead at most three minutes.
- Kill or narrow to CEM-only at recall below 25%, evidence gaming above 20% of audited material
  hunks, or any falsely closed critical item. An intermediate result gets one additional sealed
  20-PR cohort, then must promote or stop.
- The prospective paired trial uses at least 40 assigned tasks with the same SHA, harness, and tool
  allowlist in both arms, and evaluates every assigned task. Require at least 20% fewer reviewer
  misses, no more than 15% median latency increase, and zero treatment-only critical misses.
- At least 80% of treatment tasks MUST reach an allowed stop without disabling or overriding the
  hook. Record blocked-stop count and time from first frontier to closure. Kill the stop-hook claim
  if bypass or disable exceeds 20%.
- Read-before-edit production MUST cover at least 70% of legitimately citable material hunks while
  producing zero fabricated read events. One fabricated event is a release blocker.
- An independent consumer MUST reproduce the reference frontier fixture bytes. Corvint-authored
  producer-to-Corvint-verifier tests do not count as interoperability.

## Non-goals for V0

- whole-application behavioral completeness or a behavioral-twin model;
- embeddings, a vector or graph database, a daemon, hosted control plane, UI, or team ACLs;
- Jira, Confluence, incident, onboarding, migration, release narration, or multi-repository backfill;
- replacement of code search, LSPs, Spec Kit, OpenSpec, agent harnesses, or mandatory project gates;
- outcome-model training or a shared context commons before the local trial passes.

## Promotion-triggered follow-ons

- `corvint why` returns only revision-pinned historical links and current drift, and returns `unknown`
  when no committed rationale exists. It starts only after 20 verified retained changes exist.
- The first Spec Kit or OpenSpec adapter preserves native requirement IDs and lifecycle state; it
  starts only after three pilot repositories show demand. Corvint does not replace either authoring
  flow. The second ecosystem is a separate demand decision.

## Rollback

The frontier command and adapters are experimental views over plain CEM/OCM artifacts. If the
relevance or historical-PR gates fail, retain the underlying verifier and explicit unknowns, remove
the stop-hook claim, and redesign the obligation universe before adding product surface.
