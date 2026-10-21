# 2026-10-20 V1-0191: corvint-mcp companion profile accepted and qualified by the official-schema receipt

Human-owned intent: `docs/specs/mcp-server-2026-07-28-v0.md`, section "Companion promotion evidence
(V1-0191)". The intent is accepted for the companion profile under the owner's 1.0 feature-cut
delegation (2026-10-20). The 1.0 classification table in
`docs/specs/corvint-1.0-product-and-release-v1.md` already lists `corvint-mcp` as a required 1.0
companion and is unchanged.

BASE: `2b0cfce94ae2cea248c3ccab4b301af99af572d6`. Receipt commit:
`a7870cac3`, which changes only documentation over the base, so the server code is the base's.

## Finding

The ticket asks for four things. Two were already delivered on the base, and two depended on the
pinned official schema, which the repository does not vendor.

- Read-only task-context and `cem report` tools (criterion 2) exist as `corvint.context` and
  `corvint.cem.report` behind `--tool-profile task-review` (`MCPV0-024..026`, decision 0374). They
  use the `MCPV0-008` envelope, the terminator-collision refusal and the 393,216-byte bridge budget
  (`internal/mcp/bridge/bridge.go` `maxResultBytes`). Their no-mutation tests are
  `TestContextToolReturnsBoundPacketWithoutWrites` and
  `TestCEMReportToolPreviewsCLIReportWithoutPublishing` in process, and
  `TestContextAndCEMReportAreBoundReadOnlyAndFramed` over the compiled process. The compiled test
  compares a digest of the whole root, `.git` included, and requires that no `cem-review.md` was
  written. No code change was needed.
- The three official-schema tests (`TestServerTrafficMatchesOfficialSchema`,
  `TestTaskReviewTrafficMatchesOfficialSchema`, `TestAFUV1034FlowsOfficialSchema`) skip unless
  `CORVINT_MCP_OFFICIAL_SCHEMA` names a copy with SHA-256
  `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`. The first lane run had no
  copy. The owner then approved a download of the schema pinned in
  `conformance/mcp-2026-07-28/README.md` (commit `271ecc9accafdd9b83a3c869fa67c22953b2af80`) to a
  scratch directory outside the repository. Its SHA-256 matches the pin. The earlier pass on
  2026-09-23 (`docs/BUILD-LOG.md`) predates the flows, follow-ref, reason-class and bound changes,
  so the receipt below replaces it for this server.

## Decision

- The spec's intent status moves from proposed to accepted for the companion profile. Its delivery
  status moves from experimental to `implemented (stdio companion profile, official-schema receipt
  2026-10-20)`, naming the three receipt items in "Companion promotion evidence". `ROADMAP.md`'s
  disposition row follows.
- The inline "proposed" words on `MCPV0-031..033` and `MCPV0-036..037` are removed. Those
  requirements now inherit the accepted header, like the rest of the profile.
- `mcp-server-unavailable` leaves `globalDegradations` in `integrations/compatibility.json` and the
  `degradations` of both adapter manifests (`integrations/claude-code/plugins/corvint/compatibility.json`,
  `integrations/codex/plugins/corvint/compatibility.json`), citing this receipt.
  `integrations/README.md`, `ROADMAP.md` and the Codex and Gemini CLI READMEs say the same.
- The receipt covers the stdio server only and no host tuple. No adapter row changes status,
  lifecycle conformance or host-version evidence. Every row stays `FALLBACK`. The Claude Code
  manifest keeps `surfaces.mcp` `UNAVAILABLE`, because neither plugin ships an MCP declaration.
- Removing the code changes shipped manifest content, so `AHI-020` requires version bumps. Claude
  Code goes from 0.3.2 to 0.3.3 (`plugin.json`, `marketplace.json`, `compatibility.json`
  `adapter.version` and `lastValidation.adapterVersion`, and the matrix row). Codex goes from 0.2.6
  to 0.2.7 (`plugin.json`, `compatibility.json` and the matrix row). This follows the precedent in
  `aa4dd403c` and `0cddfecd2`. `lastValidation` keeps its 2026-10-09 `STATIC_ONLY` date and status,
  so no host requalification is claimed.

## Evidence

Host: darwin, 12 cores, Go `go1.27.3` (the repository pin is `go1.27.2`; the host has only 1.27.3).
Load average was 30 to 84 during the runs, so the timings are inflated and are not budgets.

Lane run at `2b0cfce94`, with no schema:

- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -v ./conformance/mcp-2026-07-28`:
  `ok` in 22.6 s. 35 top-level PASS, 0 FAIL, 3 SKIP. The skips are the three official-schema tests.
- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./cmd/corvint-mcp`: `ok` in 13.5 s.
- `heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/mcp/bridge`: `ok` in
  33.6 s.

Official-schema receipt at `a7870cac3`, schema SHA-256
`ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`, load average about 40:

- `heavy.sh env CORVINT_MCP_OFFICIAL_SCHEMA=<scratch>/schema.json GOTOOLCHAIN=local go test -count=1 -timeout 30m -v -run OfficialSchema ./conformance/mcp-2026-07-28`:
  `ok` in 2.6 s. `TestAFUV1034FlowsOfficialSchema` PASS in 1.11 s,
  `TestServerTrafficMatchesOfficialSchema` PASS in 0.27 s, and
  `TestTaskReviewTrafficMatchesOfficialSchema` PASS in 0.42 s. None skipped.
- The same command without `-run`: `ok` in 17.2 s, with 38 top-level PASS, 0 SKIP and 0 FAIL.
- The orchestrator ran the three tests independently at the same commit (go1.27.3, darwin): all
  pass in 1.51 s, 0.41 s and 0.63 s, with 0 skips and the package `ok` in 4.3 s.

Not run:

- the `-race` variant;
- the official Streamable HTTP conformance runner, which has no stdio target and stays `NOT_RUN`
  as the conformance README records;
- fuzz and cross-build promotion;
- host lifecycle requalification for the bumped adapter versions.

The spec's `NOT_OBSERVED` items also remain: a notification-cancellation-won call, an emitted
progress notification and non-Unix cleanup.

## Rollback

Revert the commits for this entry:

- restore `mcp-server-unavailable` to the three compatibility files and restore the prior adapter
  versions;
- restore the README, `ROADMAP.md` and spec wording, with the spec header back to `Intent status:
  proposed` / `Delivery status: experimental`;
- restore the inline "proposed" markers;
- regenerate the README/INDEX projection with `make spec-requirements`.

No server code or state changed.
