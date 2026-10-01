# Corpus CI qualification repair

The owner authorized one additional bounded repair cycle after PR #418 exposed two
CI failures (V1-0587 and V1-0588). This slice preserves the prior proof binding
`8b6ad1ce978ab34be13b4aef129d279e42ef6ec3` and HTTP documentation binding
`4aad8266cb49c82b8cbb7b9ad787b4413b1ab979`; neither is relabelled as this repair.

## Public integration and intent

The append-only integration merge `0b8fe85c65d326784bb85d619ede55a1a0356c3f`
joins public main `0b1244244066391da26f58b8e0cc8ebd737d9e8b` and published
PR head `d99b49cf60ca570a36f18346d3a0fc11fff44e94`. The primary checkout is
untouched. Native attempt generation 90 admits the complete PR delta plus conservative
primary import paths; those admission paths do not authorize unrelated source edits.
The fresh local enrollment fixes this integration merge as its base and names the
existing documentation-corpus and public-release intent specs before source changes.

## Observed defects and repair

The same-source ordinary capacity reproduction passed with 767,732,232 allocated
bytes; its race-instrumented reproduction and hosted Linux race run failed the
1 GiB ceiling. Instrumentation changes allocation costs. DCP-V1-040 keeps the
ordinary 1 GiB ceiling and every capacity, determinism, digest, query and complete
receipt parity assertion in both modes. A race run logs observed allocation bytes
and marks the ordinary budget `NOT_RUN`; it cannot qualify ordinary allocation cost.
The exact cause of the additional instrumented allocation was not separately profiled.

The real manifest/build/search CLI returned `corvint-corpus-receipt/2`, operation
`search`, and one result; the PUB-V0-024 smoke predicate required version 1. The
smoke now admits only versions 1 and 2 with exact `search` and nonempty results.
The extracted validation boundary is used by the real subprocess smoke. The new
version-2 control failed before the predicate change, while legacy, unsupported
profile, wrong-operation, empty-result, malformed and trailing-JSON controls had
their expected outcomes. Unknown versions remain refused.

## Acceptance evidence and limits

Selected verification covers the complete corpusindex package in ordinary and race
modes, real built-binary PUB-V0-024 discovery workflows and receipt controls, changed
package vet, focused documentation gates and the spec index: all passed locally on
macOS arm64. Ordinary allocation was 767,689,608 bytes; race allocation was
1,090,249,800 bytes with ordinary budget `NOT_RUN`. Each corpus had 30,001 claims
and 5,000 symbols. The actual installed fixture passed all five discovery steps.
One independent bounded review covers these changes and public integration metadata;
its exact content binding and final results are retained with the immutable handoff.

The pre-edit Corvint query and affected receipts are retained separately. Initial
`make dogfood-change` at the empty base range could not produce a CEM (`git-diff-failed`);
automatic agent prechange receipts remain `NOT_OBSERVED`. That refusal is retained.
The final source and CEM binding use this integration base rather than treating the
inherited seals as newly verified implementation.

No ordinary Linux allocation qualification follows from a macOS arm64 run. Hosted
CI is required on the published head. No external adopter, hosted deployment,
production index promotion, or full original issue #399 acceptance is asserted.
Repository-wide `make gate` remains `NOT_RUN` under the owner's scoped issue policy.
Rollback is an append-only revert of this repair, preserving prior proof and seals;
reverting restores the known CI failures rather than qualifying the previous predicate.
