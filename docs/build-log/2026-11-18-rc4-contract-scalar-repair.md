# RC4 named scalar contract repair

Date: 2026-11-18
Ticket: V1-5679, task 3.1 only
Authority: RCF-V1-011 and RCF-D2.
Starting HEAD: 252f5beaf601650d6e1c002263c93ef057355e07.

## Finding and decision

Self-review found that the corpus field guard recorded named scalar identities without their
underlying kinds. A string enum could become numeric while keeping its name. The Markdown
runtime fixtures did not serialize the reachable jstestreceipt.Attempt.state field.

Keep the existing field assertions and frozen goldens unchanged. Add an independent explicit
scalar-kind contract over every type reached by the field walker, including map keys. Compare
named scalar kinds before container dispatch, so changing a pinned scalar into a container also
fails. Only these scalar forms are certified:

- bool, string, int, int64, float64 and uint8 with their matching underlying kinds;
- jstestreceipt.ExecutionState with underlying string;
- no exported methods on either scalar values or pointers, excluding custom encoders.

Unknown scalar identities and kinds fail closed. Pointers, slices, arrays, maps and structs are
traversed for scalar leaves. Dynamic Receipt.results values and raw JSON contents remain
NOT_PROVEN by this static scalar check; runtime contract tests cover only exercised fixtures.
This repair does not claim that a reflection walk proves arbitrary custom container encodings.

Check representative zero/nonzero values, negative numbers, booleans and escaped strings with
both encoding/json and doccorpus.Encode, passing values and pointers. Serialize all seven real
ExecutionState constants in Attempt records through both encoders and require JSON strings.

## Regression evidence

TestRCFV1011CorpusFormat now runs the additional scalar contract. The new
TestRCFV1011ScalarContractClosedForms keeps a synthetic named numeric type's field identity
unchanged and requires rejection against its independently pinned string kind. Direct, pointer,
slice, array and map-key positions all reject with `underlying kind got int, want string`.
Unknown named bool and complex128 cases prove the default refuses unreviewed scalar forms.
A pointer MarshalJSON method on a string scalar is also rejected rather than inferred from
passing samples. TestRCFV1011AttemptStateJSON covers the real Attempt.state wire field.

## Limits and rollback

Production behavior, JSON shapes, error codes, stderr, goldens and original test assertions are
unchanged. No qualification, promotion or ranking-order claim is added. Independent review and
broader host verification remain with the orchestrator. CEM binding is unavailable under
retired docs/DOGFOOD.md and decision 0594; the handback records the requested target attempts.
Reverting this repair restores the old coverage gap and requires no data migration.

## Validation

- Required affected plan: corpusbridge selected by DIRECT_TEST_CHANGE; scope UNKNOWN,
  303 selected units and 57 language-frontier unknowns retained.
- script/heavy.sh go test -count=1 -timeout 30m ./internal/mcp/corpusbridge:
  PASS, 43.968s, all existing tests without a run filter, using go1.27.4.
- Staged script/check-line-citations.sh --repin: PASS, no moved references.
- make -s spec-requirements-check line-citations-check: PASS, including internal/specindex
  through heavy.sh (1.130s); 184 pre-existing skipped basename citations retained.
- Frozen fixtures and goldens are unchanged. No production source is changed.
- make -n dogfood-finish: exit 2, target absent. Diagnostic only; no bind or seal.

All commands used TMPDIR=/home/russell/lanes/rc4-contract.tmp; Git identity variables were
absent during tests and gates. The host semaphore configuration was used without overrides.
Darwin execution is NOT_RUN on this Linux host. Whole-repository gates, Windows vet and
independent review are NOT_RUN in this round under the focused repair instructions.

Raw evidence stays in /home/russell/lanes/rc4-contract-evidence/. SHA-256 bindings:

| Evidence | SHA-256 |
| --- | --- |
| parity_test.go | c698cb7ef59583c19a2e02993e84d27247f5d945853c26e9701450f51ee8e565 |
| rcf_scalar_test.go | 165510699ae6cc68283c09379ccf3744695695b4d8602ffcd3edab27c3e41a3c |
| self-review-affected.json | f33ba8c63c47326765c2cebcd4db728a0fa4297966798fe1100320add2ec8068 |
| self-review-package.log | 9800238c3144624f6e45e39e79f1c1df2837a0f04c4282b914bd3c4090ed25d3 |
| self-review-gates.log | 28a2355f8c8345556c5b31a9a610ce334be26bdbc0607b167ece82853f9a5cf3 |
| self-review-repin.log | dce4b7fe3eadc26103692787d6775a7701ed62b33f01ccc227f7182433b8ea7a |
