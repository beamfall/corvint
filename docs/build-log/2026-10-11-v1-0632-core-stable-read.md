# V1-0632 S1: Core status and verify read cem/1.0 through the stable verifier

Date: 2026-10-11. Base: `10efc02d16e6069d73426e6d6abd860e71ae83ab` (`origin/main`).
Branch: `claude/cem1-consumers-s1`. Scope: the first consumer-migration slice of
`CEM-V1-007`, with proposed requirements `CEM-V1-017` to `CEM-V1-019` in
`docs/specs/cem-stable-v1.md`. No decisions, ticket-store writes, PRs or producer changes.

## Finding

The S0/S0E foundation added the stable wire (`wire.StableSpec = "cem/1.0"`), the
stable verifier (`verify.Stable`) and the hidden `cem verify-stable` command. No
consumer outside `internal/cem` read `cem/1.0`. Core `cem status`, `cem verify` and
`cem report` sent an exact `cem/1.0` map to `wire.ParseMap`. That parser refused it as
`unsupported-spec` with the 0.x text ``spec must be "cem/0.1", "cem/0.2", or
"cem/0.3"``. The refusal did not say where the map could be read. The MCP report
preview projected the same refusal as `cem-map-invalid`.

The public S0E lifecycle blockers in
`protocol/cem-1.0/stable/repository-envelope-packet/REQUIRED-REPAIRS.json` are
RB01 to RB07. RB01 to RB06 are repaired natively. On this host (Darwin arm64,
Go 1.27.1), `TestStableS0EPublicCases` gives 54 PASS and 2 NOT_RUN; the two NOT_RUN
cases need Go 1.24.13. `TestStableS0ELinkedControl` passes. RB07 is the remaining
host qualification (Linux amd64, portable standalone Linux, cross-device topology),
tracked under V1-0650 and V1-0668. It needs hosts, not code, so it does not block
this slice.

The consumer stage plan reconstructs nine slices from the S0E `MIGRATION.json`:
- S1: Core read.
- S2: report, markdown and MCP.
- S3: CEM producer and the dogfood flow.
- S4: companion and portable verify.
- S5 to S8: OCM, frontier, TCQ, obligations, prove, local completion and remaining consumers.
- S9: the end-to-end Tasks-to-runner-to-producer chain and the migration dispositions.

That plan is a working note outside the repository. Each later slice needs its own
build log.

## Change

- `wire.ParseMap` keeps every precedence step. Only an exact `cem/1.0` string spec
  now gets an `unsupported-spec` message that names the stable read. Every other spec,
  including the experimental candidate, keeps the old text.
- `wire.DeclaresStable` selects the reader only. It is true for a bounded JSON object
  whose `spec` is exactly `cem/1.0`.
- `workflow.Read` reads the map bytes, then parses them.
  - When the 0.x parser refuses an exact stable root, `readStable` handles status and verify.
  - It refuses report and report preview with `unsupported-spec`.
  - It refuses `--patch` and policy ceilings with `invalid-arguments`.
  - It calls `verify.Stable` on the session root with no Core `openRepository`, because
    the stable admission forbids any configuration pre-read.
  - It embeds the complete result as `stableVerification`.
  - Status adds `map` and `state`, which is `accepted`, `rejected` or `unsupported`.
    The read never produces 0.x verification, counts, worklist, policy issues or
    `ready-for-ci`.
- A 0.x map that is given `--artifacts` refuses with `invalid-arguments` at stage 3,
  after its map parses.
- `cem status` and `cem verify` accept `--artifacts`. `RunWithOCM` exits with the
  verifier's own code: 0 for ACCEPT, 1 for REJECT, 2 for a refusal.
- The read is hidden: `cmd/corvint` help and receipts are unchanged.

## Evidence

- Failing before: the new CLI tests and `TestParseMapStableRefusalNamesStableRead`
  were copied into a detached `origin/main` worktree at `8f032d969`. Its
  `internal/cem` is identical to the base. There they fail:
  - `TestCoreStableReadMatchesVerifyStable`
  - `TestCoreStableReadKeepsVerifierExit`
  - `TestCoreStableReadExplicitRefusals`
  - `TestCoreStableReadRefusesArtifactsFor0x`
  - `TestParseMapStableRefusalNamesStableRead`

  The two unchanged-behaviour guards pass there, as intended:
  `TestCoreStableRead0xRefusalsUnchanged` and `TestParseMapOtherSpecRefusalUnchanged`.
  `TestDeclaresStable` cannot compile on the base.
- After the change, with Go 1.27.1 on Darwin arm64, all of these pass. Each Core read
  equals `cem verify-stable` on all 21 fields and the exit code, against the public
  sealed positive materialized from the S0E packet:
  - The positive is ACCEPT with sidecar EXACT, exit 0.
  - With `--artifacts` omitted, the result is `artifacts-required`, exit 2.
  - With one artifact byte flipped, the result is REJECT `artifact-digest-mismatch`, exit 1.
- Under Go 1.27.2, the same tests pass. The Core-equals-`verify-stable` comparison
  still runs. The ACCEPT and REJECT outcome assertions log NOT_RUN, because the
  public case names Go 1.27.1.
- Focused packages, all `ok`, with `-p 1 -count=1`:
  - Go 1.27.1:
    - `go vet` of `internal/cem/...`, `internal/mcp/bridge` and `cmd/corvint`.
    - `go test` of `internal/cem/...`, `internal/mcp/bridge`,
      `conformance/cli-parity-v0`, `conformance/ocm-v0`, `conformance/frontier-v0`
      and `internal/dogfoodflow`.
  - Go 1.27.2:
    - `go vet` of the touched packages, `internal/mcp/bridge` and `cmd/corvint`.
    - `go test` of `internal/cem/wire`, `internal/cem/workflow` and `internal/cem/cli`.
    - The 19 `cmd/corvint` tests matching
      `TestCEM|TestRootHelp|TestHelp|TestStable`. They include
      `TestCEMSeamsDependOnlyOnStdlibAndGit`, `TestCEMHelpSurfaces` and
      `TestCEMEndToEndAndGoldenEnvelope`.
- Doc gates pass with `rc=0`: `spec-requirements`, `requirement-definitions`,
  `traceability-tests`, `decision-numbers`, `line-citations`, `unbounded-readers`,
  `use-case-receipts` (18 current) and `go-format`.
- `corvint affected --base 10efc02d1` selected 146 packages through the shared `wire`
  and `workflow` imports and left 22 language-frontier unknowns. Those unknowns include
  `go:build-constraint-variants` and `go:nested-module-frontier`.

## Limits

- NOT_RUN:
  - The other affected packages, which import `wire` only for 0.x parsing.
  - `cmd/corvint` beyond focused CEM tests.
  - `make gate`.
  - The `interop/cem01-go` module, which is untouched.
  - Linux and Go 1.24.13 runs of the new tests. The tests compare against
    `verify-stable`, so they hold on any runtime, but ACCEPT and REJECT are asserted
    only on the Go release named by the public case.
  - Independent review.
  - The dogfood CEM bind and seal, which are left to the integrating lane.
- The MCP projection of the stable refusal is still `cem-map-invalid` until S2.
  Report and markdown have no stable rendering yet.
- The host default `go` is Homebrew Go 1.27.2, and the repository pin is Go 1.27.1.
  The broad run used the cached Go 1.27.1 toolchain, and the touched packages were
  also run under Go 1.27.2. Under Go 1.27.2 the sealed positive refuses at stage
  `runtime`, which is correct behaviour, not a regression. That refusal is why its
  outcome assertions are tied to the case's `goVersion`.
- Help visibility, Core production of `cem/1.0`, and the OCM, frontier, TCQ and
  obligation dispositions remain owner decisions. Their defaults are hidden, A01 only
  and explicit refusal.
- Rollback: revert this change. No map, packet, golden or 0.x output byte changes.
