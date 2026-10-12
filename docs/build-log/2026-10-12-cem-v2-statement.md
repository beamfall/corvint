# 2026-10-12 CEM `cem/v2` statement (V1-1265)

Branch `claude/v1-1265-attest-v2`, based on 056b11643. Decision 0523 P1. The requirement is
`CEM-V1-065` (proposed) in `docs/specs/cem-stable-v1.md`. Decisions 0354 and 0365 carry amendment
notes, and FPK-V0-033 names the new URI.

## Decisions

- Statement:
  - the predicate type is `https://corvint-context.dev/attestation/cem/v2`;
  - the subjects are the map (name and sha256 of its exact bytes), then every declared artifact in
    map order (name = declared path, sha256 = declared digest);
  - the predicate is the closed object `{"baseRevision","patchSha256","target":{"gitCommit","gitTree"}}`.

  The artifacts are subjects rather than predicate members because in-toto consumers already match
  on subjects. A runner-receipt attestation swap therefore changes either the map bytes (old
  envelope gives `attest-cem-mismatch`) or a subject (old map gives
  `attest-cem-subject-mismatch`).
- Emission: `prove --attest-cem-v2` signs only what the stable verifier accepted.
  - It runs the same verify session as `cem verify-stable`, with full-OID `--expected-base` and
    `--target` and the `--artifacts` root.
  - It brackets the tree, the dirty set and the map bytes.
  - It refuses anything other than ACCEPT with a target revision (`attest-cem-v2-not-accepted`).
  - The target tree comes from the accepted commit.
  - A `cem/0.x` map is refused with `attest-cem-v2-requires-stable-map`.
  - `cem/v1` still refuses a stable map, so it is not widened.
- Verification:
  - `--verify-cem-attestation` takes `--artifacts ABS` and `--target REV`;
  - `artifactsStatus` and `targetStatus` are NOT_RUN unless the check is supplied;
  - giving either against a `cem/0` or `cem/v1` envelope is refused with the v2 code, because
    those envelopes sign neither.
- Anchor: `wire.ParseRouted` sends a map declaring `cem/1.0` to `ParseStable` and leaves every
  other map on `ParseMap`. `cem anchor` therefore records `map_spec` `cem/1.0` and the map sha256,
  and `cem provenance` reads the pointer back verified. There is no verdict ledger.
- Error codes:
  - reused: `attest-cem-mismatch`;
  - new: `attest-cem-subject-mismatch`, `attest-cem-target-mismatch`,
    `attest-cem-v2-requires-stable-map`, `attest-cem-v2-not-accepted`.

  All of them are named in the CEM-V1-065 section.

## Walkthrough

The public `protocol/cem-1.0/stable/receipt-authentication/` recipe was materialized as a
repository with `stable.json` untracked, and this command was run:

```
corvint prove --cem stable.json --expected-base 467cbb73… --target 1f181cd0… \
  --artifacts <case dir> --attest-cem-v2
```

It exited 0 and printed two lines:
- the proof statement, with state READY at revision `1f181cd0…` and `verifier-accepts` PASS 2;
- the `cem/v2` statement, with four subjects (`stable.json`, `artifacts/runner-plan.json`,
  `artifacts/runner-receipt.json`, `artifacts/receipt-attestation.json`) and target tree
  `c1949cc4344b78776b6691be676a5f911d2ea27d`.

`TestCEMV1065*` repeats this in `cmd/corvint` with an in-test key and also checks:
- the receipt swap under the old envelope;
- a foreign `--target`;
- a `cem/0.x` map;
- the flag conflicts.

## Limits

- No signing producer, key management or independent `cem/v2` consumer.
- Emission requires full OIDs and an accepted map.
- The target tree is read from the repository, not re-derived from the artifacts.
