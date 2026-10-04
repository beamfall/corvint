# Proposed pool member exclusion intent (issue 480)

The owner-selected per-claim exclusion alternative is seeded as CAL-V0-065/S15
before the first implementation enrollment. Independent Gate A passed with no HIGH
findings; the plan retains authoritative replay before current-policy membership
validation and dedicated repeatable parsers in both CLI families.

Baseline: `1fda1b94984245d0cd0ac0a6d17cc72572ce6619`. Root reserved CAL-V0-065
after the unlanded CAL-V0-062/063 and CAL-V0-064 slices. This seed preserves their
reservations and every earlier requirement ID. It does not promote implementation
or native qualification: both remain NOT_RUN.

Original Corvint query and impact receipts were retained before edits in the
leaf's private Git directory. The change-start dogfood observation is retained
with incomplete reasons in `/private/tmp/corvint-480-start-dogfood.log`; it is not
a passed gate. Native effects remain INCOMPLETE and retain the full future PATH,
SCHEMA, generated-output and shared-gate union. Source work uses an isolated leaf;
the primary checkout remains under the coordinator's source hold.

Acceptance will require the fixed old request-preimage witness, allocation and
health filtering, preview purity, prepared-admission revalidation, both next-claim
selectors, replay after successor and policy changes, both CLI parser families,
a disposable native fixture, and independent implementation review. No such
acceptance test is claimed by this seed. Rollback and failure modes remain in S15.
