# Bounded descendant retention for V1-0689

## Decision and evidence

Owner-requested V1-0689 repair retains the existing observed PID/start cleanup contract and wire
fields. Independent Gate A resolved resident zombie continuity and recommended keeping the
existing 4095 displayed-descendant maximum, 4 MiB ingestion and current identity representation.
The coordinator accepted that smaller scope on 2026-10-03. Resident ownership remains bounded
including visible zombies; displayed witnesses are separate and omissions explicit. Duplicate
PID snapshots cannot establish retirement. Transient recovery semantics remain unchanged.

The bounded diagnostic at public base32aa5a3f2b7f3fe658d208049fb009e2c9e24c13 reproduced cumulative
4096-entry failure with at most three simultaneous rows. Nested three-second off/on observation
retained 2/90 identities; zero named sampler matches leave attribution inconclusive. Both
cancellation cases passed with independent absence readback. Claim230 returned at receipt2326.
Same-PID replacement at the cap is source-confirmed, runtime NOT_RUN at this seed; the original
replacement control was below the cap. It is retained on V1-0689 at receipt2329.

## Verification state

This is the pre-implementation spec seed. Production code is unchanged. Old-source behavioral
RED, focused GREEN, bounded cancellation with unrelated sentinel, independent source review and
frozen dogfood evidence are NOT_RUN. No production delivery or native completion is claimed.
The separate suspected stderr resource risk is V1-0691, runtime NOT_RUN; it is not a prerequisite.

## Limits and rollback

The observer remains bounded sampling, not atomic kernel identity or full process containment.
Witness rows do not promise exhaustive historical identity replay. Aggregate report-bound
refusal remains; no maximum-size fit claim. Original parent state, consumed N32 marker, original
unqualified N32 and provider-validity-incomplete remain unchanged. No new N32 is authorized.
Rollback reverts this isolated repair and clarification; it restores the known cumulative-cap
blocker and does not erase either version's evidence.
