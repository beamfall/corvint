# External Evidence Provider Remote V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted (decision 0321)
Delivery status: not-started
Authoritative inputs: issue 11 and the owner's direction to resolve it; decision 0318;
`docs/specs/external-evidence-provider-transports-v0.md`.

## Agent digest
- Claim: A separately built `corvint-remote-provider` fetches one TLS-pinned HTTPS record only with explicit network consent.
- Status: accepted (decision 0321)/not-started; acceptance requires `TestRemoteTransportConformance`.
- Exists: `cmd/corvint-remote-provider`, `internal/remoteprovider`.
- Blocked on: no delivery prerequisite; public Internet availability is not a correctness witness.
- Read next: Requirements; Acceptance; Rollback.

## User and boundary

An operator can supply a remote evidence record through the existing `--provider-command` transport.
The adapter is separately built, never imported by Core, never part of the default install, and
owns no index, authority or database. Core's immutable index contract and strict record decoder
remain the verifier. Fetching manually into a file remains the simpler baseline.

## Requirements

- `EEP-REMOTE-001`: The separately built command MUST require `--allow-network --config FILE` on
  each invocation. Configuration MUST be one bounded JSON object containing `url`, `spkiSha256`
  and optional `credentialFile`; HTTPS URLs MUST have no userinfo, query or fragment. No other
  invocation or environment variable enables networking. Core MUST not import this adapter.
- `EEP-REMOTE-002`: Fetch MUST be a single GET, with normal certificate/hostname validation and
  exact SHA-256 server SPKI pinning. TLS 1.2 or later is required. Redirects, proxies, retries,
  compression and connection reuse MUST be disabled. All work shares a ten-second deadline;
  response headers are at most 64 KiB and the complete body at most 1 MiB. Only status 200 succeeds.
- `EEP-REMOTE-003`: Optional bearer credentials MUST be read only from a regular, non-symlink,
  operator-owned file with no group/other permission bits and at most 4096 bytes. Credentials
  MUST be a nonempty printable ASCII token without whitespace; no environment or argv credential
  is accepted. Errors MUST be fixed local text; credential bytes MUST never enter stdout, stderr,
  receipts or records. A response containing the credential MUST be refused before publication.
- `EEP-REMOTE-004`: Complete successful body bytes MUST be written unchanged and decoded only
  by the existing file/command decoder. Every network, TLS, HTTP, timeout, oversize or input error
  MUST exit nonzero with zero stdout, causing Core's closed command-provider row. No partial record,
  retries, cache, persistent process, repository write or authority promotion is permitted.

## Acceptance

`TestRemoteTransportConformance` serves all existing EEP-V0/V1/V2 and selection fixture bytes over
test HTTPS and compares full resulting sections. `TestRemoteFailures` covers timeout, unavailable,
oversize, malformed records, TLS mismatch, redirect, HTTP failure, secret reflection and credential
permissions. `TestRemoteCLI` verifies explicit opt-in and zero stdout on failures. Tests use local
TLS fixtures; no network service or account is a prerequisite for the default product or gate.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `EEP-REMOTE-001` | `cmd/corvint-remote-provider`, `internal/remoteprovider` | `TestRemoteCLI` |
| `EEP-REMOTE-002` | `internal/remoteprovider` | `TestRemoteFailures` |
| `EEP-REMOTE-003` | `internal/remoteprovider` | `TestRemoteFailures` |
| `EEP-REMOTE-004` | `internal/remoteprovider` | `TestRemoteTransportConformance`, `TestRemoteFailures` |

## Rollback

Stop invoking and remove the optional binary, its command/package and decision 0321. Existing files,
records and immutable index formats are unchanged. Decision 0318's default-path NO-GO remains active.
