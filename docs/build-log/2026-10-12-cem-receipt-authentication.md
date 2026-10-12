# 2026-10-12 CEM runner receipt authentication (V1-1264)

Branch `claude/v1-1264-receipt-authentication`, stacked on the V1-1263 branch at adb5714fe.
Decision 0523 P1, requirement `CEM-V1-084` (proposed) in `docs/specs/cem-stable-v1.md`. The
ticket text names CEM-V1-064; that ID is the accepted bundle export requirement, so this
requirement takes the next free ID, CEM-V1-084.

## Decisions

- Wire: a receipt's `authentication` is the sealed legacy string `"NOT_OBSERVED"`,
  `{"method":"none"}` or `{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}`. The legacy
  string stays valid and re-encodes unchanged, so every sealed S0E map, the repository-envelope
  packet and `RESULT.schema.json` keep their bytes. The method selects the key set, so the order is
  missing-field, unknown-field, invalid-field.
- Graph: E is a new `receipt-attestation` artifact role, required only for a dsse receipt; a
  repeated E is `duplicate-reference` and an attestation no receipt names is
  `artifact-role-closure`. A map that ships a key file as an attestation is therefore refused at
  the references stage: a map never supplies its own trust.
- Envelope: one DSSE envelope (`application/vnd.in-toto+json`, exactly one signature) over a
  closed in-toto Statement v1 with predicate type
  `https://corvint-context.dev/attestation/cem-receipt/v1`, an empty predicate and the ordered
  subjects `runner-receipt`, `runner-plan`, `target-tree` (gitTree). K is the hex SHA-256 of the
  raw Ed25519 key, so the DSSE keyid and the receipt keyid are the same value.
- Checks run after both artifact passes: shape, keyid, subjects (with or without trust), then the
  signature only for a trusted K. A failure is REJECT at stage `artifacts`, exit 1, with the code in
  `issueCodes` and `authentication` FAILED. A bad signature under an untrusted key is never
  checked, so it stays UNTRUSTED_KEY and can never reach VERIFIED.
- Trust: only `--trusted-keys FILE` (absolute, regular, at most 64 KiB, closed
  `corvint-trusted-keys/1`, 1..64 keys) can raise the axis. A bad file is `invalid-arguments`
  before the map is read, never a silent fallback to NOT_OBSERVED. The native CLI and both portable
  commands take the flag. The console pane trusts no key and says so.
- The public case's keys are TEST-ONLY and derived from public strings
  (`test-only-keys.json`); `TestCEMV1084ReceiptAuthenticationFixtures` re-derives every key,
  envelope and key file from them.

## Timed walkthrough

Binaries `corvint` and `cem01-go` built from the lane worktree with `GOTOOLCHAIN=local`
(go1.27.2, darwin/arm64; build cache warm, 1 s). The repository was built from
`receipt-authentication/repository.recipe.json` with Git plumbing in under 1 s (base `467cbb73c`,
target `1f181cd08`). Each row is one process, wall seconds measured around it. The operator was
an agent, so human reading and decision time is NOT_OBSERVED.

| Map | `--trusted-keys` | Command | Seconds | Exit | Result |
| --- | --- | --- | --- | --- | --- |
| `stable.json` | `trusted-keys.json` | `corvint cem verify-stable` | 0.66 | 0 | ACCEPT, authentication VERIFIED, executionAtCommit NOT_OBSERVED |
| `stable.json` | `trusted-keys.json` | `cem01-go verify-stable-canonical` | 0.77 | 0 | same |
| `stable.json` | `trusted-keys.json` | `cem01-go verify-stable` | 0.24 | 0 | same |
| `stable.json` | `other-trusted-keys.json` | `corvint cem verify-stable` | 0.25 | 0 | ACCEPT, UNTRUSTED_KEY |
| `stable.json` | `other-trusted-keys.json` | `cem01-go verify-stable-canonical` | 0.50 | 0 | ACCEPT, UNTRUSTED_KEY |
| `stable.json` | `other-trusted-keys.json` | `cem01-go verify-stable` | 0.21 | 0 | ACCEPT, UNTRUSTED_KEY |
| `stable.json` | none | `corvint cem verify-stable` | 0.24 | 0 | ACCEPT, NOT_OBSERVED |
| `stable.json` | none | `cem01-go verify-stable-canonical` | 0.52 | 0 | ACCEPT, NOT_OBSERVED |
| `stable.json` | none | `cem01-go verify-stable` | 0.19 | 0 | ACCEPT, NOT_OBSERVED |
| attestation over the base tree | `trusted-keys.json` | `corvint cem verify-stable` | under 1 | 1 | REJECT artifacts, `attestation-subject-mismatch`, FAILED |
| attestation over the base tree | `trusted-keys.json` | `cem01-go verify-stable-canonical` | under 1 | 1 | same |
| key file as attestation (self-carried) | `trusted-keys.json` | `corvint cem verify-stable` | under 1 | 2 | REJECT references, `artifact-role-closure` |
| key file as attestation (self-carried) | `trusted-keys.json` | `cem01-go verify-stable-canonical` | under 1 | 2 | same |
| `stable.json` | relative `trusted-keys.json` | `corvint cem verify-stable` | under 1 | 2 | REJECT arguments, `invalid-arguments` |

Both falsifiers from decision 0523 P1 hold: a self-carried key stays below VERIFIED (it is refused
outright), and a signature over another gitTree is refused even by a trusted key. The full
EXPECTED.json (one accept, five axis cases, thirteen refusals) passes on the native verifier and
on both portable verifiers in the `TestCEMV1084*` tests.

## Limits

- No producer signs receipts. Signed production is an operator step (FPK-V0-036), and
  `internal/cemcandidate/stable.go` still assembles receipts with the legacy authentication form.
- `RESULT.schema.json` still pins `authentication` to `NOT_OBSERVED` for both the receipt
  reference and the axis, and its issue-code enum lacks the four attestation codes. A result with
  VERIFIED, UNTRUSTED_KEY or FAILED does not validate against it until the additive regeneration
  that V1-1265 owns. The sealed S0E packet is unchanged.
- The keys are TEST-ONLY; no key distribution, rotation, revocation or transparency log exists,
  and stranger time is NOT_OBSERVED.
