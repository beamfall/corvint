# 2026-10-20 V1-0191: corvint-mcp companion profile accepted; official-schema receipt NOT_RUN

Human-owned intent: `docs/specs/mcp-server-2026-07-28-v0.md`, section "Companion promotion evidence
(V1-0191)". The intent is accepted for the companion profile under the owner's 1.0 feature-cut
delegation (2026-10-20). The 1.0 classification table in
`docs/specs/corvint-1.0-product-and-release-v1.md` already lists `corvint-mcp` as a required 1.0
companion and is unchanged.

BASE: `2b0cfce94ae2cea248c3ccab4b301af99af572d6`.

## Finding

The ticket asks for four things. Two were already delivered on the base, one needs an input this
lane cannot fetch, and one depends on that input.

- Read-only task-context and `cem report` tools (criterion 2) exist as `corvint.context` and
  `corvint.cem.report` behind `--tool-profile task-review` (`MCPV0-024..026`, decision 0374). They
  use the `MCPV0-008` envelope, the terminator-collision refusal and the 393,216-byte bridge budget
  (`internal/mcp/bridge/bridge.go` `maxResultBytes`). Their no-mutation tests are
  `TestContextToolReturnsBoundPacketWithoutWrites` and
  `TestCEMReportToolPreviewsCLIReportWithoutPublishing` in process, and
  `TestContextAndCEMReportAreBoundReadOnlyAndFramed` over the compiled process. The compiled test
  compares a digest of the whole root, `.git` included, and requires that no `cem-review.md` was
  written. No code change was needed.
- The pinned official schema (criterion 1) is not vendored. The three official-schema tests
  (`TestServerTrafficMatchesOfficialSchema`, `TestTaskReviewTrafficMatchesOfficialSchema`,
  `TestAFUV1034FlowsOfficialSchema`) skip unless `CORVINT_MCP_OFFICIAL_SCHEMA` names a copy with
  SHA-256 `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`. No such copy existed
  under `/tmp`, `/private/tmp` or `~/Downloads`. Fetching it from the URL pinned in
  `conformance/mcp-2026-07-28/README.md` was outside this lane's authority, so the official-schema
  execution is `NOT_RUN` at this base. Its last pass, on 2026-09-23 (`docs/BUILD-LOG.md`), predates
  the flows, follow-ref, reason-class and bound changes, so it is not a receipt for this server.

## Decision

- The spec's intent status moves from proposed to accepted for the companion profile. Delivery
  status stays experimental. The spec names the three receipt items that promote it, and records
  item 2 as `NOT_RUN`.
- The inline "proposed" words on `MCPV0-031..033` and `MCPV0-036..037` are removed. Those
  requirements now inherit the accepted header, like the rest of the profile.
- `mcp-server-unavailable` stays in `integrations/compatibility.json` and both adapter manifests.
  The ticket allows removal only with the conformance receipt cited, and there is no current one. No
  adapter row changes. The stdio receipt would cover no host tuple anyway.

## Evidence

Host: darwin, 12 cores, Go `go1.27.3` (the repository pin is `go1.27.2`; the host has only 1.27.3).
Load average was 30 to 84 during the runs, so the timings are inflated and are not budgets.

- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -v ./conformance/mcp-2026-07-28`:
  `ok` in 22.6 s. 35 top-level PASS, 0 FAIL, 3 SKIP. The skips are the three official-schema tests
  above, each reporting `CORVINT_MCP_OFFICIAL_SCHEMA unset: official-schema validation NOT_RUN`.
- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./cmd/corvint-mcp`: `ok` in 13.5 s.
- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/mcp/bridge`: `ok` in
  33.6 s.

Not run: the `-race` variant, official-schema execution (missing input above), and the official
Streamable HTTP conformance runner (no stdio target; it stays `NOT_RUN` as the README records).

## What closes the rest

An operator fetches the pinned schema, then runs the following at the candidate commit:

```console
CORVINT_MCP_OFFICIAL_SCHEMA=PATH GOTOOLCHAIN=local go test -count=1 -run 'OfficialSchema' -v ./conformance/mcp-2026-07-28
```

The three tests must pass, not skip. A new build-log receipt then records the run with the
schema's SHA-256. The change that cites that receipt removes `mcp-server-unavailable` from
`integrations/compatibility.json` and both adapter manifests, updates `integrations/README.md` and
`ROADMAP.md`, and moves delivery status off experimental.

## Rollback

Restore the spec header to `Intent status: proposed`, the inline "proposed" markers, and the
README/INDEX projection (`make spec-requirements`), and delete this entry. No code, state or
compatibility row changed.
