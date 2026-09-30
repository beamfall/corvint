# Workflow integration: remaining CI maintenance

The completed CI run `36713890494` at `23799bb6` exposed three further
integration failures in its final product shard. Native ticket V1-0531 retains
the exact failure logs, reproduction and acceptance criteria.

## Existing contracts and changes

- The dispatched experimental `breakage` verb was present in the maturity
  section but absent from the root `Commands` list. Add its discovery row;
  experimental status and command admission remain unchanged.
- The instruction-doctor source was the only new production `contextindex`
  input in the six-workflow change. `IDX-SNAP-V0-017` and its deliberate
  conservative source audit require review even for consumer-only additions.
  Advance the analyzer schema from 96 to 97 and update both audit pins. This
  invalidates old experimental pack engines through the existing refusal and
  fallback path; it changes no encoding, extraction dependency, retention bound,
  or default gob/sectioned engine rule.
- A separate-descriptor DEBUG trace showed the shell fixture stopped at its
  old `wording_status = 1` assertion. The exact canonical refusal now has the
  proposed QAT discovery-abstention disposition. Assert `NOT_PRODUCED`, profile,
  exit 2, original task and NUL-argv hashes, base/target and the report's artifact
  digest. Keep all malformed and unreachable trace cases blocking. This changes
  the fixture to follow the existing experimental contract, not the allowlist.

## Verification and limits

Gate A passed independent review with no HIGH concern. Focused schema audit,
unchanged-schema pack reuse and mismatched-engine refusal tests passed (0.898s).
Root verb discovery passed (0.480s). The complete dogfood shell fixture passed
(28.813s), including its existing hostile cases. Focused vet and the 18 use-case
receipt pins passed. Logs are retained under `/private/tmp/corvint-wow-manager/`
with the `ci-` prefix.

The implementation review, separate repair CEM and final CI are recorded by the
private task receipts and PR. Earlier sealed maps keep their original bindings;
no complete historical binding or behavioral qualification is inferred. Revert
this repair to roll back the catalogue and fixture, or restore schema 96 to
restore its prior engine identity; retain the failed CI evidence. None of these
repairs accepts proposed intent or promotes an experimental workflow.
