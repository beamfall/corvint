# Tasks: `ticket-taxonomy`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order. Group 1 happens
in this repository. Group 2 happens in the corvint-tasks repository. Nothing in group 2 starts
before the owner accepts this change.

## Tasks

- [ ] 1.1 File the ticket in the corvint-tasks store, reserve prefix `TTX-V1` (001 to 018), and link
      it from a `v1-0` ticket in this repository. Requirements: all. Proof: `corvint-tasks ticket
      show` in both stores.
- [ ] 1.2 Write `docs/specs/ticket-vocabulary.json`: components (including `rig`, C21; its product
      is recorded first, TTX-V1-002) with product and root paths, types
      with kind, runners, agents, bug classes, slices, chore actions, and the component of each
      change folder. Requirements: TTX-V1-001, TTX-V1-002, TTX-V1-005. Proof: a check that every
      change folder under `docs/specs/openspec/changes/` has exactly one component.
- [ ] 1.3 Write the gate script (seed gate and interim create gate). Requirements: TTX-V1-016,
      TTX-V1-017. Proof: fixture tickets, one per refusal code, each refused with its code.
- [ ] 1.4 Run the gate on the survivor mapping table of `tasks-store-cutover`. Requirements:
      TTX-V1-017. Proof: the `seed` and `held` lists and exit status, retained with the table's
      SHA-256.
- [ ] 1.5 Update the ticket-filing instructions and skills to the new payload and templates.
      Requirements: TTX-V1-011, TTX-V1-012. Proof: one ticket of each type created through the gate.
- [ ] 2.1 First commit in corvint-tasks: create `docs/specs/ticket-taxonomy-v1.md` from the spec
      delta. Requirements: TTX-V1-001 to 018. Proof: the Tasks repository's spec index check.
- [ ] 2.2 Policy block `facets` and its create checks. Requirements: TTX-V1-001 to TTX-V1-005.
      Proof: store tests for missing and unknown facet values.
- [ ] 2.3 Field `identityKey`, the key index, `DUPLICATE_KEY` and `distinctFrom`. Requirements:
      TTX-V1-006 to TTX-V1-009. Proof: store tests, and the two-process test with exactly one
      success. Due on corvint-tasks main by 2026-11-27 with 2.2 and 2.6 (build items B1 to B3;
      decided 2026-11-09 under R42, C18).
- [ ] 2.3a Backfill script from label `key:<k>` to `identityKey`, and the queue health count of
      keys with more than one open holder. Requirements: TTX-V1-015, TTX-V1-016. Proof: a seeded
      fixture store backfilled, and the count on a fixture with one shared key (C18, C19).
- [ ] 2.4 Sibling list and the component-scoped lexical check for every create. Requirements:
      TTX-V1-010. Proof: tests built from the three missed pairs in the sample.
- [ ] 2.5 Effects, template and acceptance checks at create. Requirements: TTX-V1-011 to
      TTX-V1-013. Proof: store tests per type.
- [ ] 2.6 Search filters and their timed reads. Requirements: TTX-V1-014, TTX-V1-015. Proof: the
      timing record on a store of at least 5,000 tickets.
- [ ] 2.7 Add the field, the block and the filters to schema v1 and the supported-command list.
      Requirements: TTX-V1-018. Proof: the `tasks-conformance` fixture and list checks. Schema v1
      freezes on 2026-12-01 (C18).
- [ ] 3.1 Retire the interim gate for rules the store now enforces. Requirements: TTX-V1-016. Proof:
      the gate script's rule list, and a create refused by the store with the script absent.
- [ ] 3.2 Build-log entries in both repositories; complete the tickets through the Tasks gate.
      Requirements: all. Proof: `corvint-tasks ticket show` and `receipt audit`.
