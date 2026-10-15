# Receipt authentication public case (CEM-V1-084)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

This case reuses the neutral-criterion repository and runner artifacts and signs
its one runner receipt with a DSSE Ed25519 envelope over an in-toto Statement
v1 (see the CEM-V1-084 section of `../ALGORITHMS.md`).

The keys are TEST-ONLY. `test-only-keys.json` records how each seed is derived
from a public string, so anyone can sign with them. Never trust these keys
outside this case.

- `repository.recipe.json`, `artifacts/runner-plan.json`,
  `artifacts/runner-receipt.json`: byte-identical to `../neutral-criterion/`.
  Build the repository the same way; `EXPECTED.json` records the commits and
  their trees.
- `stable.json`: the neutral map with the receipt's `authentication` set to
  `dsse-ed25519` (keyid of signer A, envelope digest of
  `artifacts/receipt-attestation.json`) and that envelope appended as a
  `receipt-attestation` artifact.
- `artifacts/receipt-attestation.json`: signer A over the receipt, plan and
  target tree.
- `artifacts/attestation-*.json`: refusal inputs: signed over the base tree
  (`other-tree`), signed by signer B (`other-key`), signer A's keyid with a
  signature over another statement (`bad-signature`), and a wrong
  `payloadType` (`malformed`).
- `trusted-keys.json` names signer A; `other-trusted-keys.json` names signer B.
  Both are `corvint-trusted-keys/1` files a consumer passes with
  `--trusted-keys`; their presence in this directory trusts nothing.
- `EXPECTED.json`: the full result, except `runtime`, under
  `trusted-keys.json`; five axis cases (no file, another key, method none,
  the legacy string, a bad signature under an untrusted key); and thirteen
  refusals. Each case replaces whole top-level map members with `set` and
  passes `trustedKeys` (a file in this directory) when named.

Run with either consumer (absolute paths):

```sh
corvint cem verify-stable --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE \
  --trusted-keys CASE/trusted-keys.json
cem01-go verify-stable-canonical --repository R --map CASE/stable.json \
  --expected-base BASE --target TARGET --artifacts CASE \
  --trusted-keys CASE/trusted-keys.json
```

Both consumers must return the same result. The tests named `TestCEMV1084*` in
`internal/cem/verify` and `interop/cem01-go` check this, and
`TestCEMV1084ReceiptAuthenticationFixtures` re-derives every key, envelope and
key file from the recorded seeds.
