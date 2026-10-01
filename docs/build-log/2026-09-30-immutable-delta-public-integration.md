# Immutable delta public integration and qualification

Final public base: `7bd7f5e03ad177e7496ce5dbd1563b8466f591a4`.
Reviewed source: `77a4693191bee21730894feb52bc35e948e1de4e`, preserved by normal merge.
Public source/CLI qualification: `cc34e34c6a7a9289ae8421578db88ad47c33de97`, build 228.
Approved public merge: `03820be1f5e097cebc482eaa97216f8ec4e975a5`; all 296 public sealed maps
remain byte-identical. Native generation 88 widened atomically (receipt 1579); ticket effects
remain INCOMPLETE. Scope includes the three registries, fixed CEM and this change's own seal.

## Authority and chronology

The owning DLT contract remains proposed/experimental. Public dispatch uses the existing root
preamble and option-value classification, then the reviewed immutable compiler. Help marks the
command experimental. Source review PASS_R2 applies to the unchanged compiler; the independent
integration review covers public wiring, changed dependencies and actual qualification evidence.
No third semantic repair or production promotion is included.

Actual pre-edit query and positional main/help impact outputs remain in
`/tmp/delta389-prechange-query.json` and `/tmp/delta389-prechange-impact.json`.
Both packets report context revision `6776375514a49dc777c8ab8ff2369cd2d9c5d0a4`;
query history_tip reports public base `01f557cb47272d44c1316cc58d2b44529661756f`.
These are the observed bindings, not invented immutable-public-base receipts. Wrapper agent receipt
slots remained NOT_OBSERVED. Initial `make dogfood-change` refused empty diff and absent proposed
DLT intent, wrote no fixed CEM, and retained every NOT_PRODUCED reason in
`/tmp/delta389-prechange-dogfood.log`. The source import preceded its new integration enrollment.

The `01f557` enrollment passed six selected checks at cc34 and left four checks/reports/final check
unmet. It was cancelled explicitly as non-success before distinct final-base enrollment; its plan,
observations and unknowns remain retained. The final workflow uses the same ten inspected selected
checks at the assigned 7bd base. Earlier receipts never imply new-base satisfaction.

## Actual native evidence and limits

At cc34/build 228, all six bounded native cases passed: public argv/help/refusals, immutable merge
two-run equality including dirty ambient bytes, missing/incomplete/stale provider conservatism,
actual base-bound flow generation admission/refusals, no-op/four wire decision vectors, and schema
validation. `/tmp/delta389-native-qualification/result.json` retains 25 actual calls and 14 unchanged
read-only snapshots. Duplicate base values prove identical duplicate acceptance; distinct-value
precedence is NOT_OBSERVED. Actual compiler-input comparison includes 259 Go/embed inputs and
11 differences from reviewed source; focused checks passed for the eight providers and full
Git/external-evidence packages. These are Darwin arm64 observations with no OS network enforcement
or complete transient-write proof. Final-base qualification requires fresh bound observations.

Three valid JS positive attempts emitted conservative findings. Their exact scope/provider/unit
gaps remain in `/tmp/delta389-native-positive-qualification/result.json` and
`/tmp/delta389-native-positive-test-closure/result.json`. The latter added a helper under tests/
without runner evidence, creating an unknown-runner frontier; it does not prove universal
all-touched test-unit impossibility.

A fourth, reviewer-derived construction produced actual native tests-needed twice with identical
bytes: change only a Node test invocation label under checks/, keep its imported helper's lexical
body unchanged, and supply fresh explicit external path asserts. It has one affected unit, zero
lexical flows/gaps, no full-suite requirement and only exempt runtime uncertainty.
`/private/tmp/delta389-native-derived-tests-needed/result.json` and its driver retain exact accepted
inputs, three calls, hashes, schema validation and matching repository/external snapshots.
Native classes witnessed are no-op, tests-needed and findings; wire vectors remain separate.

## Remaining docs-only profile conflict

Strict provider completeness requires each changed-path obligation to be qualified
(`internal/extevidence/selection.go:157-165`, `:823-835`). Direct path qualification selects a test
row (`:468-499`); path/entity discharge also selects a runnable test (`:240-251`, `:412-434`),
and entity-only evidence cannot qualify (`:510-522`). Delta projects selected rows into Tests
(`internal/delta/compile.go:249-266`), while absent/non-narrow evidence yields consequential
unknowns (`:195-239`). Any selected test overrides docs-only
(`internal/delta/record.go:109-110`); unknowns or gaps yield findings (`:113-120`).
The same independent reviewer retained this current-profile structural contradiction and verified
the derived tests-needed witness in `/tmp/delta389-independent-integration-review.json`.

V1-0579 retains the native witness and remaining docs-only conflict. DLT-V0-009 and original
four-class acceptance remain PARTIAL. No profile/classification change or acceptance reduction
is authorized. The full affected-package oracle defect V1-0564 also remains open; selected filtered
checks are not full package coverage. Owner acceptance and external outcome validation remain
separate. Final evidence, ready publication, coordinator integration and native completion must
succeed before this ticket can close.

Rollback reverts this capability's owned source/integration changes and its own seal, preserving
public foreign seals, unrelated work and retained historical evidence.
