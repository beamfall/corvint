# Linked reads survive content replacement proofs

Self-review repair for the P1 finding in `internal/testevidence/capture.go`.
Governing intent: `RSP-V0-007` lexical/resolved path preservation and bounded
write proofs; `RSP-V0-010` immutable-content freshness.

The failure class is treating proof about resolved file contents as proof about
every dependency of a read. A successful exclusive create or rename replacement
does not prove that the lexical symbolic link cannot change the read's outcome.
The suppression allowlist now admits only valid direct-path blobs (lexical equals
resolved), matching repository and resolved write path, successful earlier writes
in the same process tree, and either create with exclusive-create/own-failed-lookup
proof or replace with tree-created-file rename proof. Every other form retains
the read. This API cannot retain a separate link edge, so it keeps the whole
linked read. The existing unknown-write-kind default remains fail closed.

`TestRSPV0007WriteProofRetainsLinkRetargeting` combines captured linked reads with
all three admitted proof variants, verifies the unchanged record is fresh, then
changes only the link's immutable identity and requires freshness refusal.
`TestRSPV0007UnknownWriteProofRetainsRead` supplies every positive proof flag with
an unknown write kind on a direct path and requires retention. Existing positive
direct-path tests and all existing assertions and goldens remain unchanged.

Verification: `corvint affected --base 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`
selected `internal/testevidence` with `DIRECT_SOURCE_CHANGE`; its additive result
was `FALLBACK / IMPORT_NOT_ALREADY_IN_PACKAGE`. The installed binary reported
`self-binary-stale`, retained as a limitation. The full package passed through
`script/heavy.sh`, `GOTOOLCHAIN=local go test -count=1 -timeout 30m
./internal/testevidence` (0.132 seconds reported by Go). No Git identity variables
were exported for tests. The first wrapper admission required an explicit whole
package reason; the retry supplied the owner's whole-package mandate.

The staged citation repin found no moved citation. Static spec requirements and
line citations passed; the spec-check target's auxiliary Go test recipes were
disabled to keep this repair within its authorized package scope. Independent
review and repository-wide checks remain host-owned; qualification is not claimed.
No wire shape, diagnostic, exit code or core-freeze output changed. Rollback is
reverting this repair commit; that also restores the reported suppression defect.

Raw evidence is external under `/home/russell/lanes/rspt10-evidence/`:

| Artifact | SHA-256 |
|---|---|
| `self-review-affected.json` | `522fb70e14af524368da92f9200d337bd034ebde28ebeffb82c55703e2a64593` |
| `self-review-testevidence.log` | `55aae5ae006ed7b8ab6a070f1cfcd53be399258295fe9a02d73d0c4edf399905` |
| `self-review-spec-citations.log` | `dd0afbd2633ff9bf949007de988c6f97467684ed03cd3caf55330950900dcd88` |
