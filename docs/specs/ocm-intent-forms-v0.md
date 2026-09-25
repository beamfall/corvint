# OCM intent forms V0

Owner: Russell Lewis
Date: 2026-09-25
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md` invariants 3 and 6, `docs/specs/ocm-v0-dogfood.md` (`OCM-V0-001`),
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, decision 0386, ticket V1-0259

## Agent digest
- Claim: `ocm prepare --intent-form` lets an OCM intent be an ADR's numbered decisions or a roadmap shard's Acceptance tickets instead of only a `## Requirements` spec.
- Status: proposed/experimental; the owner has not accepted the intent, and the capability must not be advertised as delivered
- Exists: `internal/lrfrepo/ocm_forms.go`, the `--intent-form` flag, an optional `intentScope.form` wire member, focused tests, and one live Beamfall sweep.
- Blocked on: owner acceptance and the open decisions below; LRF and the change universe refuse declared forms.
- Read next: Declaration mechanism; Requirements; Acceptance evidence.

## User and job

A repository that states intent as architecture decision records or roadmap tickets, not as
Corvint-shaped specs, needs OCM to enumerate those obligations without rewriting its documents.
`OCM-V0-001` accepts only one `## Requirements` section of `- \`PREFIX-NNN\`: ` lines, which conflicts
with invariant 6 (no new spec language). The first such repository is Beamfall: ADRs under
`docs/adr/` carry `## Decisions` with `### N. Title` items, and roadmap shards under
`docs/plans/roadmap/` carry `- [ ] **ID — title.**` tickets with `  - **Acceptance:**` sub-bullets.

## Verified starting state

At Corvint `9e355d03`, `requirementsFromBlob` (`internal/lrfrepo/ocm.go`) is the only intent reader,
and every OCM action re-derives from the pinned blob through it. At Beamfall `2a8e06b28`, all 215
ADRs carry a front-matter `adr: NNNN` line; 102 use `## Decisions`, 42 `## Decision`, 21
`## 2. Decisions`, 1 `## 2. Decision`, and 49 other headings (such as `## Decision (proposed)` or
`## Decision 1 — Title`); no ADR has two of those four. `## 2. Decision` is the singular of the
numbered form with the same `### 2.1` items, so it joins the `OIF-V0-005` set and its ADR reaches
the item grammar instead of a heading refusal. The 44 roadmap shards hold 2453 top-level tickets;
71 carry more than one Acceptance line, and most carry only `**Verify:**`.

## Declaration mechanism

The form is a `corvint ocm prepare --intent-form FORM` argument, recorded in the map. Reasons:

- It is explicit per intent and needs no new configuration file, glob language, or extra pinned
  blob. A repository-owned mapping file was rejected as heavier and as one more authority surface.
- `--intent` stays a pure Git path. A `form:path` prefix was rejected because Git paths may contain
  `:`, so the prefix is ambiguous.
- Recording the form in `intentScope` makes every later action (status, link, mark, verify, report,
  resume) re-derive with the same grammar without the caller repeating it, and makes a changed form
  a binding change that `--replace` must acknowledge.
- Invariant 3 holds: the form only selects which of the project's own grammars is read. Every
  requirement and statement is project-owned text at a pinned blob, and a strict parse refuses
  anything it cannot read instead of guessing.

## Requirements

- `OIF-V0-001`: `corvint ocm prepare` accepts `--intent-form FORM`, where FORM is exactly `requirements`, `adr-decisions`, or `roadmap-acceptance`; an absent flag or `requirements` selects the `OCM-V0-001` reader, and any other value is an argument error. The form is never inferred from the intent path or content, and no other OCM action takes the flag.
- `OIF-V0-002`: A non-default form is recorded as the string member `intentScope.form`; the default form omits it, so default-form maps, prepare envelopes, and re-derivations stay byte-identical to `OCM-V0`. A reader accepts `form` only with a non-default form name; an explicit `requirements`, empty, unknown, or non-string value fails `invalid-intent-form`. The spec string stays `ocm/0.1-experimental`.
- `OIF-V0-003`: Every OCM read and write action re-derives requirements, span, and span digest from the pinned intent blob with the recorded form, and a mismatch fails `intent-scope-mismatch` as in `OCM-V0`. Obligation IDs are checked against the recorded form's ID grammar (`invalid-obligation-id`), and a declared form's obligation statement is the item text defined below.
- `OIF-V0-004`: `adr-decisions` requires the blob's first line to be `---`, a later closing `---` line, and exactly one `adr: NNNN` line (four digits) between them; otherwise it fails `invalid-adr-number`. The number comes from the front matter, not the file name.
- `OIF-V0-005`: `adr-decisions` requires exactly one unfenced line equal to a member of the closed set `## Decisions`, `## Decision`, `## 2. Decisions`, `## 2. Decision`; the span runs from it to the next unfenced level-2 heading or the end of the blob, with the `OCM-V0-001` fence rules. No such line, more than one (the same or different members), or any other shape (such as `## Decisions:`, `### Decisions`, or `## Decision (proposed)`) fails `invalid-decisions-section`. The member read changes neither the `OIF-V0-006` item grammar nor the requirement IDs: a span with no level-3 heading fails `missing-requirements` (`OIF-V0-009`), and a `### 2.1`, `### D1 —`, or `### A.` item fails `invalid-decision-item`.
- `OIF-V0-006`: Every unfenced level-3 ATX heading in the Decisions span must match `### N. text` or `### Na. text` at column 0, where N is 1 to 999 without a leading zero and a is one lowercase letter; otherwise it fails `invalid-decision-item`. Each item becomes requirement `ADR-NNNN-D<N><a>` in document order, its statement is the heading through the byte before the next item or the span end, and a repeated ID fails `duplicate-requirement`.
- `OIF-V0-007`: `roadmap-acceptance` treats each unfenced line that starts `- [c] `, with c one character, as a ticket. It must continue `**ID — title**` with the closing `**` on that line, the first ` — ` (em dash) followed by a non-empty title, and an ID matching `^[A-Z][A-Z0-9]*(-[A-Za-z0-9]+)+$` of at most 64 bytes; otherwise it fails `invalid-ticket-item`. A repeated ID fails `duplicate-requirement`, and the span is the whole blob.
- `OIF-V0-008`: A ticket's body is the contiguous following lines that start with a space. Its Acceptance block is each unfenced body line that is `  - **Acceptance:**` or starts with `  - **Acceptance:** `, plus the body lines indented at least four spaces that directly follow it. A ticket with a non-empty block becomes a requirement whose ID is the ticket ID and whose statement is the block. A ticket without one is not an obligation and is listed in document order in the prepare envelope member `excludedTickets`, which only this form emits.
- `OIF-V0-009`: A declared form that yields no requirement fails `missing-requirements`, more than 256 requirements fail `too-many-obligations`, and a non-UTF-8 blob fails `invalid-intent`. The `OCM-V0` bootstrap rule for an intent absent at base applies unchanged.
- `OIF-V0-010`: The `lrf` OCM leg and the change-universe (Frontier/TCQ) OCM surface refuse a map that declares a form with `unsupported-intent-form` before re-derivation; LRF keeps reading only `OCM-V0-001` requirement lines.
- `OIF-V0-011`: The capability is experimental: help labels the flag experimental, and no release note, README, or product claim may advertise it as delivered until the owner accepts this spec and the promotion evidence passes.

## Non-goals

- Guessing a form from a path, name, or content, or falling back from one form to another.
- Other ADR shapes (headings outside the `OIF-V0-005` set, `### D1 —`, `### 8A.`, `### 2.1`), `*` or `+`
  task bullets, indented top-level tickets, and nested sub-tickets as separate requirements.
- Selecting one ticket or decision from a file; treating `**Verify:**` as acceptance.
- LRF evaluation of declared forms, or a change to the dogfood wrapper (`internal/dogfoodflow/`).
- Any new configuration file or spec language.

The simpler baseline is to convert Beamfall intent into Corvint spec files by hand, which breaks
invariant 6 and drifts from the owner's documents.

## Failure modes

All refusals fail closed before a map is written, with the codes above. A shard with any malformed
top-level ticket fails as a whole, since skipping it would hide an obligation. Tickets without
Acceptance are visible in `excludedTickets` on prepare only; status and verify do not repeat them.
A changed form for an existing map is a binding change and returns `map-outdated` unless
`--replace` is given.

## Acceptance evidence

- `internal/lrfrepo/ocm_forms_test.go`: `TestOIFV0*` cover `OIF-V0-002` to `OIF-V0-010` with fixtures
  minimized from Beamfall ADR-0224 and roadmap shards 41 and 42, hostile cases, a default-form
  identity check, a wire round trip, and a prepare/mark/resume/read run on a Git fixture.
- `internal/lrfrepo/ocm_forms_test.go`: `TestOIFV0005ADRDecisionsHeadingVariants` covers each member
  of the `OIF-V0-005` set, and `TestOIFV0005ADRDecisionsHeadingRefusals` covers near misses and two
  different members in one ADR.
- `cmd/corvint/ocm_test.go`: `TestOIFV0PrepareIntentFormFlag` covers `OIF-V0-001`.
- Existing OCM tests (`./internal/lrfrepo/`, `conformance/ocm-v0`, `cmd/corvint -run 'OCM|Help|Prepare'`)
  pass unchanged, which is the default-form byte-identity evidence.
- Live run (2026-09-25) with the branch binary on a scratch clone of Beamfall `2a8e06b28` plus one
  probe commit: ADR-0224 yields 14 requirements (`ADR-0224-D1` to `D13` and `ADR-0224-D6a`), and
  `ocm mark` and `ocm status` re-derive from the recorded form. Shard 41 yields 6 with 4 excluded;
  shard 42 yields 13 with 13 excluded. Across the corpus, 89 of 215 ADRs prepare (565 requirements),
  113 fail `invalid-decisions-section`, 6 fail `invalid-decision-item`, and 7 fail
  `missing-requirements`. 11 of 44 shards prepare (265 requirements, 935 tickets excluded), 8 fail
  `invalid-ticket-item`, and 25 fail `missing-requirements`.
- Corpus sweep (2026-09-25) after the `OIF-V0-005` heading set, running the reader in a scratch test
  over the same 215 ADRs (not an `ocm prepare` run): 99 derive (620 requirements), 49 fail
  `invalid-decisions-section`, 42 fail `invalid-decision-item` (all 22 numbered-heading ADRs use
  `### 2.1` or `### A.` items), and 25 fail `missing-requirements` (18 `## Decision` ADRs have no
  level-3 item). The pre-change reader gives the live run's 89, 113, 6 and 7 on the same files.

Promotion needs owner acceptance, answers to the open decisions, and one real change dogfooded with
a declared form. `NOT_RUN`: that dogfooded change, and independent review.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `OIF-V0-001` | `cmd/corvint/ocm.go` `parseOCMPrepareFlags` | `TestOIFV0PrepareIntentFormFlag` |
| `OIF-V0-002` | `parseIntent`, `parseIntentForm`, `intentScopeValue` | `TestOIFV0IntentScopeWireForm`, `TestOIFV0DefaultFormIsUnchanged` |
| `OIF-V0-003` | `deriveIntent`, `verifyOCMIntent`, `parseObligations` | `TestOIFV0ObligationIDsFollowTheDeclaredForm`, `TestOIFV0PrepareRecordsAndRederivesTheDeclaredForm` |
| `OIF-V0-004` to `OIF-V0-006` | `adrDecisionsDerivation`, `adrDecisionsHeadings` | `TestOIFV0ADRDecisionsDerivation`, `TestOIFV0ADRDecisionsRefusals`, `TestOIFV0005ADRDecisionsHeadingVariants`, `TestOIFV0005ADRDecisionsHeadingRefusals` |
| `OIF-V0-007`, `OIF-V0-008` | `roadmapAcceptanceDerivation` | `TestOIFV0RoadmapAcceptanceDerivation`, `TestOIFV0RoadmapAcceptanceRefusals` |
| `OIF-V0-009` | `finishDerivation` | refusal tables above |
| `OIF-V0-010` | `refuseDeclaredIntentForm` | `TestOIFV0LRFRefusesDeclaredForms` |
| `OIF-V0-011` | `cmd/corvint/help.go` | this spec's status |

## Rollback

Revert the change. Default-form maps are untouched. Maps that carry `intentScope.form` then fail
`unknown-field` on every OCM action, a fail-closed refusal; remove them and run `ocm prepare` again
against a `## Requirements` intent.

## Open decisions

1. Should tickets without Acceptance stay excluded, or become obligations whose statement is the
   ticket line or its `**Verify:**` block?
2. Should a caller be able to select one ticket or decision (for example `PATH#TICKET-ID`)?
3. Resolved: should `adr-decisions` accept `## Decision` and `## 2. Decisions` as declared variants?
   Answered yes 2026-09-25 by owner delegation; closed variant list in `OIF-V0-005`.
4. Should LRF evaluate declared forms, which needs an `internal/lrf` statement grammar per form?
5. How should the dogfood wrapper pass a form through its intents file?
