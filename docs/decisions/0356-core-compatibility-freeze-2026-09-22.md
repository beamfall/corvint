# Decision 0356 — Freeze the Core command, wire and migration contract and label every other verb experimental

Date: 2026-09-22. Status: proposed (experimental delivery; ticket V1-0007). Owning contract:
`docs/specs/core-compatibility-freeze-v1.md` (CCF-V1-001 to CCF-V1-008). The Core boundary is an
assumption pending owner ratification in V1-0001. No runtime behaviour or wire output changes.

## Context

Corvint 1.0 needs a compatibility promise that an agent or CI job can rely on across releases.
Before this decision the promise was implicit. Byte parity cases in
`conformance/cli-parity-v0/manifest.json` pinned `query`, `impact`, `init` and `adopt`; a few
in-repository readers (`prove-observe`, `internal/attest`, the companion core smoke) matched profile
strings exactly; and root help said only that commands marked Experimental carry no stability
promise, while most of the 42 dispatched verbs were marked neither way. Nothing said which profiles
were Core and which belonged to companion or research slices, so any of them could be treated as
frozen by accident.

## Decision

1. Core is `init`, `adopt`, `query`, `context`, `impact`, `affected` and `prove`, the set the
   V1-0007 ticket names. The owner ratifies or corrects it in V1-0001; a correction amends CCF-V1-001
   and the test list together.
2. What is frozen is what 0.7.0 already emits: the per-mode identifiers in CCF-V1-002, the refusal
   envelope, exit classes and code families in CCF-V1-004, and the admission, freshness, omission and
   abstention members in CCF-V1-005. Modes and profiles listed in CCF-V1-003 stay outside.
3. The breaking-change rule (CCF-V1-006): byte changes on cli-parity-pinned output need a divergence
   register entry and a decision; elsewhere, adding optional members is compatible and anything that
   removes, renames, retypes, re-enumerates or re-identifies a frozen member needs a new profile
   version, an N-1 reader and a decision.
4. N-1 is the previous release, now 0.7.0 (CCF-V1-007). Derived state (the index snapshot) rebuilds on
   any mismatch; durable traces keep the legacy reader and the explicit digest-bound migration; each
   in-repository reader of a Core profile accepts N and N-1 in the change that bumps it.
5. Root help gains a `Command maturity:` section (CCF-V1-008). It lists the Core verbs and labels each
   other dispatched verb Experimental with the requirement prefix of its owning spec. No verb is
   hidden, because the repository has no hidden-verb mechanism and adding one would be new behaviour
   outside this ticket.

## Alternatives set aside

- Hide experimental verbs from default help. Rejected: no mechanism exists, and
  `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb` requires every dispatched verb in `Commands:`.
- Rename the `genesis-inventory/0.1-experimental` identifiers before freezing. Rejected: renaming is a
  wire change and would change every receipt id; the bytes are frozen as they are.
- Include `feature` in Core because it shares the platform qualification. Not done: the ticket names
  seven verbs; the owner can add it in V1-0001.

## Consequences

A new top-level verb cannot ship without a maturity label and an indexed owner, and a Core identifier
cannot change without failing `TestCoreVerbsEmitTheFrozenProfiles`. Rollback is a revert of the change
that adds the contract, this decision, the help section and the test; no state is migrated.
