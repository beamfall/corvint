# Expanded repository envelope amendment — review draft

Status: PROPOSED (publicrepair6); blocked before implementation or expansion admission
until independent review and implementing Gate A (see "Repair6 routing" below). This packet
resolves parameter ambiguity and exposes remaining runtime repairs. It does not
ratify unsafe or nondeterministic observed behavior. No runtime was edited.

Read PROPOSED-RULES.md for the proposed language-neutral contract and
OBSERVED-NATIVE.md for the distinct description of the frozen native prototype.
REQUIRED-REPAIRS.json identifies acceptance blockers without implementation code.
GAP-DISPOSITIONS.json maps U01–U14. FULL-RESULT-CASES.json contains complete expected
objects authored before evaluation, with explicit current/proposed labels.

The native R0 29-case and original 50-case packets, runtime-specific control bytes,
R1 test-only repair, and all 124 historical public files remain immutable. The
new cases are draft expected results, not measured qualification. No output field
may be erased, normalized or replaced during comparison. Original results do not
acquire the proposed stronger rules retroactively.

All files in this public directory are language-neutral specifications, result
objects, original fixture declarations or already-public Apache material. They
contain no native implementation or translated native code. The coordinating
reviewer has separate immutable source citations; the portable author should
receive only this directory and its public manifest.

A new implementing Gate A must accept the required rules, error precedence,
process ownership and exact source effects before either runtime changes.
Independent native repair qualification and portable construction remain separate.
No consumer/default migration, Tasks completion, external outcome or release
promotion follows from this draft.


Repair1: read PROPOSED-RULES.md, OPERATION-ERRORS.json, LOGICAL-LEDGERS.json,
FULL-RESULT-CASES.json and REVIEW-DISPOSITIONS.json together. Normative proposal
only: requires independent re-review and both implementing Gate A decisions.
No product outputs were used to fit these new expectations.


Repair2 routing: this directory is NOT yet self-sufficient. Routing order, highest
first: (1) PROPOSED-RULES.md, OPERATION-ERRORS.json, LOGICAL-LEDGERS.json and
FULL-RESULT-CASES.json; (2) RESULT.schema.json and stable.schema.json; (3)
OPERATION.json and ALGORITHMS.md; (4) inherited/ALGORITHMS.md; (5)
inherited/cem-0.1-algorithms.md. INPUTS.json /repair2Routing names exactly one
in-packet path and hash for every role except rank 3. For rank 3, two pinned
predecessor versions exist (inherited/stable-r1/ and inherited/s0e/, both reserved).
The public text does not say which one is authoritative, so OPEN-QUESTIONS.json
OQ-01 asks root to decide. The rank-3 files, inherited/fixtures/FIXTURES.pack.json
and the two positive maps under inherited/fixtures/maps/ were UNAVAILABLE to the
public author and are absent. Root must supply them byte-for-byte and verify them
against their pins (OQ-02, OQ-03). Until then, 50 of 54 cases and two of the four
ledgers cannot be checked from this packet alone. CHANGES.md lists every repair2
change. No expectedExit or expectedResult changed.


Repair3 routing: root decided OQ-01 to OQ-06, and the paragraph above is
superseded. Rank 3 is inherited/s0e/OPERATION.json and inherited/s0e/ALGORITHMS.md;
inherited/stable-r1/ is historical only. inherited/fixtures/FIXTURES.pack.json and
both positive maps under inherited/fixtures/maps/ are in the packet and hash-verified
(INPUTS.json /repair3Routing). One routing gap remains: the rank-3 S0E text refers to
REPOSITORY-ENVELOPE.md and REPOSITORY-ENVELOPE.cells.json, which are pinned but absent
(OQ-07, blocking). OQ-08 records that the S0E OPERATION.json replaces rather than
extends one stable R1 string. CHANGES.md lists every repair3 change.


Repair4 routing: root decided OQ-07 and OQ-08, and the routing gap above is closed.
Rank 3 also routes inherited/s0e/REPOSITORY-ENVELOPE.md and
inherited/s0e/REPOSITORY-ENVELOPE.cells.json below the rank 1 amendment files.
inherited/s0e/MIGRATION.json and inherited/s0e/PACKING.md are informative. All four
match their pins (INPUTS.json /repair4Routing). Both stable R1 envelope clauses remain
in force (OQ-08). No envelope cell conflict changes an expected result. CHANGES.md
lists every repair4 change.


Repair6 routing: independent public review rejected repair5 for inconsistent
ledger refusal rows, drift input-order dependence and stale packet status. Repair6
adds rank 1 boundary rules in PROPOSED-RULES.md, keeps the sidecarless fixed prefix
at 15 calls, records LOGICAL-LEDGERS as the attempted trace through the refused
reservation, and binds drift processing/emission to bytewise ascending evidence-ID
order. The affected ledgers and expected cases are regenerated to those rules.
CHANGES.md lists every repair6 change.
