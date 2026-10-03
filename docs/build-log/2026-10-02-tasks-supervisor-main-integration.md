# Supervisor effort and wall policy: current-main composition

This checkpoint composes PR456 with current main while preserving the accepted read-checkpoint
contract. It is an integration checkpoint, not completed qualification or native ticket completion.
The coordinator's renewed owner instruction to finish and unblock the issue work authorizes the
bounded identity correction; the original effort and wall intent remains owner issue 354.

## Immutable inputs and identity mapping

The ordinary merge has first parent `f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36` (MAIN) and second
parent `05bc84b1b0d5fca05747009fd84743d8c0a77843` (OLD). MAIN's S12 and CAL-V0-059..061 retain
all checkpoint semantics, trust limits and witnesses. OLD's unlanded S12 is now S13:

| OLD identity at 05bc84b1 | Current identity | Meaning |
| --- | --- | --- |
| CAL-V0-059 | CAL-V0-062 | Policy-bounded supervisor stage effort |
| CAL-V0-060 | CAL-V0-063 | Policy-bounded supervisor stage wall |
| S12 | S13 | Issue 354 effort and wall slice |

Only current owning clauses, catalog/status rows and the six current source/test files use these
new identities. Historical logs, maps and proof remain unchanged; their old IDs must be interpreted
at their recorded revision, not as the current checkpoint requirements. S13's text is byte-identical
to the OLD slice after the identity mapping. Defaults, limits, failure modes and rollback remain
unchanged: absent policy admits only low effort and a one-hour wall, configured stage walls stay
within the four-hour lane ceiling, and provider application of effort remains NOT_OBSERVED.

## Composition evidence

Merge `2ac75442bf670194507c62fd689e9ff8d1cffee5` preserves MAIN's complete S12 and newer S11
Gemini disclosures, workflow WithClock calls and CLI writerContext. The owning spec was staged
before `make spec-requirements`. Readback found exactly one catalog definition for every
CAL-V0-001..063, preserved the existing CAL titles, confirmed line targets, and found every
unrelated catalog row byte-identical to MAIN. No fixture extension or selected check ran at that
initial composition checkpoint.

OLD's seal was verified as a single R100 rename with identical blob IDs. Ordinary revert
`9af39b4f` restores its inherited shared CEM and removes only its current archive entry; the entire
archive subtree and public `.taskman` tree equal MAIN. Both immutable inputs remain ancestors.
At that source checkpoint, the shared CEM bound historical target `41913e8a34bd320f2bf9c17951272c3ab2f98674` against
`ff3da727e95a1999cbc4a58a471cdd498693f902`; it is not evidence for this integration target.

## Pending verification and retained limits

A distinct dogfood activity was enrolled on clean MAIN before edits, freezing all 12 selected
checks. The initial `make dogfood-change` at the empty range reported not-complete: CEM prepare
failed on the empty diff, citation/map and OCM evidence were not produced, and no outcome input
was provided. Its wrapper reports prechange query/impact NOT_OBSERVED; the coordinator's separately
retained current-main context and affected baseline do not erase those reported limits.
Evidence is retained under the coordinator's `builder456-main/` scratch directory.

The fixture extension was NOT_RUN at initial checkpoint `6d7fa398`; its later source proof is
recorded below. All 12 final-target selected checks, new CEM/OCM binding, fresh independent
composition review, keyed finish, strict seal, native gate and publication remain NOT_RUN. The seven deferred interaction/vet groups require the coordinator's
supported terminal admission before execution. Corvint context was used before the change; initial
dogfood enrollment/change and source-phase affected reads were used; final verification and
evidence binding remain pending at this log commit.
Mutation, broad evaluation and live-provider routes are not applicable to this source-only phase.

Original first-author enrollment/chronology remains unrecovered; raw historical bootstrap limits
remain unchanged. Historical OCM 0/60, unassessed obligations and OPEN frontier are not promoted by
this composition. Live non-low Codex effort, stages beyond one hour, multi-host/multi-repository
qualification and Linux-only traversal qualification remain NOT_RUN. V1-0475 remains OPEN/PARTIAL.

## Refusal checkpoint witness

The integration fixture now covers both an absent derived audit checkpoint and a valid checkpoint retained by a legitimate CREATE writer. It captures presence, bytes, file identity and modification time immediately around each refused configuration. The fixture refreshes after its legitimate policy mutation. Initial setup assumed administrative PolicyUpdate retained the cache; the observed absence required one fixture repair, using a normal CREATE writer rather than constructing an authoritative checkpoint. That administrative coverage question is tracked separately as SPIKE V1-0673, linked to V1-0644; correctness or a performance regression is not established.

Focused intent/store tests, the seven discovered CAL062/063 tests, both checkpoint cases and narrow vet passed on the source checkpoint. Five temporary controls were killed: creation of an absent checkpoint and removal, replacement, byte change or modification-time change of a present checkpoint. The original sandbox denied the supervisor test's process inspection; the same authorized baseline and final selection passed outside that sandbox. These are source-phase observations, not qualified final-target checks. All twelve frozen check groups still require fresh observations at the final binding.


## Terminal evidence boundary

The coordinator admitted the terminal phase through native receipt 2355, generation 235, with
all 21 future paths and 25 finite output, process, cache and gate resources reserved. Effects remain
INCOMPLETE. Native admission uses main `dd8cc0ca918a80faaa041c98a783de11550b2bcc`; it does not
change the frozen dogfood evidence base `f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36`, existing
activity key or its 12-check plan. The source checkpoint is
`14eaf5e251691976b6cc81421c9d07a44c701dbe`.

Only the supported CEM base-or-patch mismatch route may replace the inherited historical shared
map. The current owning spec already exists at the frozen base: added CAL-V0-062..063 clauses
are not base-absent intent bootstrap exemptions. The new binding must cite applicable surviving
MAIN authority, not present checkpoint requirements as effort or wall authority. Fresh selected
checks and CEM/OCM reports will be retained in the terminal evidence without claiming that this
pre-verification log commit already passed them. Review, finish, seal, native gate and publication
remain subsequent coordinator work. All original history and qualification limits above survive.
