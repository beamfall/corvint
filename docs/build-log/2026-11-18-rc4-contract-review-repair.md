# RC4 contract comparator review repair

Date: 2026-11-18
Ticket: V1-5679, task 3.1 only
Authority: RCF-V1-011 and RCF-D2. Starting HEAD: 47e079ac561b3c1470feff92562cc658c7f99e81.

## Findings and decision

Independent review found two unsound abstractions: subset schema comparison ignored new
restrictions, and unioned output paths allowed one array member to conceal another member's
missing or retyped field. These findings supersede the corresponding coverage claims in
the original RC4 contract-test entry; its passing tests did not establish those guarantees.

Keep the original assertions and committed field lists unchanged. Add an exact structural
schema comparison, reconstructing the expected schema from those lists. Only description
annotations at recognized schema positions are omitted. Property names and constraint data
(including const, enum and unknown keywords) remain significant. Additional constraints,
including empty objects and unknown keywords, fail without a reviewed contract update.

Add recursive per-member output validation using the frozen field contracts. The closed
allowlist consists of the recorded primitive/container types and exact object fields, with
these explicit heterogeneous fixture variants:

- Search-with-claims and get: Subject and Claim record shapes from the corpus types.
- Optional info: the builder/profile record and the separate import-parity record.
- Compact results: evidence-bearing records and the count record.
- Compact anchors: plain and envelope-state records.
- Compact ranks: rank alone and rank plus matched anchor.
- Compact symbol checks: resolved with a string live reference, moved with a complete live
  location object, and unresolved without live; all three require anchor, state and reason.

Every array member and dynamically named dictionary value is checked independently. Unknown
fields, types and symbol states fail. Fixture variant counts prevent optional-field removal
from silently changing a member's variant. Result IDs, rank values and array order remain
unpinned. The additive type-field check for corpus structs remains unchanged; runtime or
schema additions now require explicit contract review rather than automatic certification.

## Evidence

The existing three RCF-V1-011 tests remain enabled. New regressions are
TestRCFV1011SchemaComparatorRejectsRestrictions, TestRCFV1011SchemaComparatorClosedForms and
TestRCFV1011OutputComparatorChecksEveryMember. They reject the reported query pattern and
unresolved-reason removal, retyping beside valid members, wrong live-location variants,
unknown states/fields/member forms, nested and dictionary omissions, and unknown schema
keywords. Original and reordered compact outputs pass. Description-valued constraints and
properties named description remain protected.

The first focused run failed while implementing the checker: Subject/Claim and optional-info
record variants needed explicit contracts, and comparator regressions needed JSON numeric
normalization. Those failures were resolved from the original types and query implementation;
no golden, fixture or existing assertion was changed. The next focused run passed (0.157s).
Additional constraint-data regressions were then included in the final whole-package run.

Final validation, without exported Git identity variables:

- The required affected plan selected corpusbridge by DIRECT_TEST_CHANGE. It retained UNKNOWN,
  303 selected units and 57 language-frontier unknowns; this is not repository-wide coverage.
- script/heavy.sh go test -count=1 -timeout 30m ./internal/mcp/corpusbridge: PASS, 37.750s,
  including all existing tests without a run filter.
- Staged citation repin: no moved citations. The initial unstaged attempt was not sufficient;
  the staged rerun checked the final spec text.
- make -s spec-requirements-check line-citations-check: PASS, including internal/specindex
  through heavy.sh (0.887s). Citation checking retained 184 pre-existing skipped basenames.

Raw logs remain outside the repository in /home/russell/lanes/rc4-contract-evidence/.
SHA-256 bindings:

| Evidence | SHA-256 |
| --- | --- |
| parity_test.go | f35deec16f21513c8599daaeac9971aa7da6e409ec8d943e293abb7cb0639c3e |
| rcf_contract_test.go | c5a08e667f155c9204b53b912fd2811d4d1650e31916cfde2edafb8f579a2597 |
| review-focused.log, failed implementation run | ce0dd2fba29530620ad5cdc7ec71884ea99ac0af43b0e055200512b322f8cb46 |
| review-focused-repaired.log | 97a54ed9b38390ede79d2fce9613cac0809a6d740c2f9828c22bc472b64e53f6 |
| review-package.log | 1bf33525e417a34de24e6ee3a0b668b810815e2602fa9492152d763616405b42 |
| review-gates.log | 937e7b6f8f0f9bb682dbf3b69554e20e32867a17f5a25f7957ba5380e33e07bd |

## Limits and rollback

No production behavior, schema, golden, stability label or qualification result changes.
Qualification, broader host verification and independent re-review remain outside this round.
Darwin runtime evidence is NOT_RUN on Linux. CEM binding is NOT_PRODUCED: decision 0594 and
docs/DOGFOOD.md removed that workflow. The external handback records the final target attempt.
No task-store write, push, PR or seal is authorized. Reverting this repair restores the previous
test coverage and reopens both findings; it requires no data migration.
