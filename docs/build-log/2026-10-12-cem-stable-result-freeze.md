# 2026-10-12 CEM stable result freeze: `citation` and `verdictSha256` (V1-1265)

Branch `claude/v1-1265-freeze`, based on 773a52e20. Decision 0523 P1 and decision 0525 §3. The
requirements are `CEM-V1-092` (citation only on ACCEPT) and `CEM-V1-066` (verdict digest) in
`docs/specs/cem-stable-v1.md`. This entry records the result-shape lane; the map-schema lane
(`2026-10-12-cem-stable-map-provenance.md`) and the `cem/v2` lane (`2026-10-12-cem-v2-statement.md`)
landed in the same integration branch.

## Decisions

- Both members landed through `tools/cem-stable-regen` (CEM-V1-091), never by hand-editing the
  sealed packet: `citation` after `artifactChecks` (21 to 22 keys), then `verdictSha256` after
  `repositoryEnvelope` (22 to 23 keys). `make cem-stable-regen-check` reports 56 cases x 23 keys.
- `citation` is the `cem-citation-support/0` block and is non-null only when the outcome is ACCEPT.
  Every refusal, including a drift REJECT, carries `null`, in both consumers. This resolves the
  nine-case native/portable disagreement recorded in V1-1288: the portable consumer no longer
  computes a citation it cannot stand behind on a refused map.
- `verdictSha256` is the lowercase hex SHA-256 of the canonical JSON of the result without
  `runtime`, `repositoryEnvelope` and `verdictSha256`. It is recomputed at encoding on every
  outcome; a value held in the struct is never emitted. Re-verification on another admitted Go
  tuple or under another repository-envelope label yields the same digest. A non-integer number in
  the result is an encoding error, not rounded.
- The two public cases (`neutral-criterion`, `receipt-authentication`) keep `EXPECTED.json`
  outside the tool's scope, so their `accept.resultExceptRuntime` members were added by a scratch
  script and their `protocol/cem-1.0/stable/manifest.json` entries re-bound by hand. Both regen runs
  also re-bound the two stale `REPOSITORY-ENVELOPE.*` manifest entries recorded in V1-1287.
- Cases 21 and 22 (go1.24.13 only) and ten envelope cells (the four sha256-repository cells and six
  refusals) have no host-run value; their members came from `--values` and `--cell-values` override
  files whose digests were recomputed independently from the sealed expectations.
- Integration: the producer-only comparison in `TestCEMV1094ProducerOnlyEmptyVerifyAccept` and its
  portable twin now exclude `verdictSha256` as well as `mapSha256`, and assert the digest moved,
  because `provenance` and `role` change the map bytes and the verdict digest covers `mapSha256`.

## Evidence

- Native: `TestCEMV1092CitationOnlyOnAccept`, `TestCEMV1092SealedCasesCarryCitationOnlyOnAccept`,
  `TestCEMV1092RefuseAndRejectClearCitation`, `TestCEMV1066SealedVerdictDigests`,
  `TestCEMV1066VerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066EveryOutcomeCarriesVerdictDigest`
  (`internal/cem/verify`); `TestStableCanonicalFullResults` compares all 23 keys.
- Portable: `TestCEMV1092PortableCitationOnlyOnAccept`, `TestCEMV1066PortableSealedVerdictDigests`,
  `TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict`,
  `TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest` (`interop/cem01-go`).
- `TestRegenerateRealPacketCopy` derives the key count and README expectation instead of
  hardcoding them.

## Gaps

- No consumer reads `verdictSha256` yet; decision 0525 step 8 and V1-1283 consume it.
- Cases 21 and 22 were not run on this host on either consumer.
- The citation regeneration left a double blank line in the packet README pointer paragraph; it is
  cosmetic and bound by the packet manifest, so it waits for the next regen run.
- Independent review (Codex, 2026-10-12) reproduced the pre-existing V1-1289 refusal-code divergence
  (`reference-coherence` native, `unresolved-reference` portable) as two different verdict digests for
  the same map; the digests are correct over their differing results, and V1-1289 now carries a
  know-how note that its fix must add a digest-parity case.
