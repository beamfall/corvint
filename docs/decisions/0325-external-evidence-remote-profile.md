# Decision 0325 — Accept a separately built HTTPS evidence adapter

Date: 2026-09-20. Status: accepted.

## Authority and decision

The owner's explicit profile approval, recorded verbatim in decision 0324, accepts
`docs/specs/external-evidence-provider-remote-v0.md`. This closes 0318's separate-profile gap;
0318's NO-GO on the default local path remains binding.

The optional `corvint-remote-provider` is built separately and passed explicitly as a provider
command. It performs one HTTPS GET only with `--allow-network`, ordinary TLS validation plus
an operator-pinned SPKI digest, no redirect/proxy/retry, bounded headers/body/deadline and private
file credentials. No networking package is imported by Core. Its stdout remains an untrusted
record, checked through the identical file decoder and immutable Git reference validation.
An explicitly selected bounded PEM CA file can replace system roots; hostname verification and
SPKI pinning remain mandatory. Literal and JSON-escaped credential reflection is refused.

## Rollback

Remove the separately built adapter and stop invoking it. The default binary, index format and
provider records require no migration; manual fetch-to-file remains available.
