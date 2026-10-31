# Core promotion inventory: profiles, retained evidence and first-batch blockers (V1-3180)

A pinned snapshot of what Corvint ships today, how mature each piece is, and what the first
promotion batch still needs. It is an inventory, not a decision. It grants no authority, approves
no release and makes no stability promise. Labels change only through an accepted profile
(`CCF-V1`, `PRS-V1-002`). The file date follows the ticket's declared path; the observations were
taken on 2026-10-30.

## Pins

| What | Value |
|---|---|
| Source | `beamfall/corvint` `origin/main` `93b9f5b08ce841875c9c81d17905908d91346f99`, tree `6b644ac7e348344d9d731a8051b5764b1c0341c7` |
| Core build | `corvint --version`: `Corvint 1.0.0-rc.3.dev (build 7, commit 93b9f5b08ce841875c9c81d17905908d91346f99)` |
| Tasks build | `corvint-tasks version`: `0.0.0-tcp01-unverified+build.7`, `go1.27.3`, slice `TCP-01`, verification `NOT_RUN` |
| Native queue | `queue:corvint:main` at 2026-10-30T19:40:42Z: `headSeq` 14408, `generation` 1004, 3316 tickets (1059 OPEN, 1391 COMPLETED, 866 ARCHIVED, 0 HELD/DRAFT), `canonicalWriter` NATIVE, `fixture` true, `executionCutover` false, `writeBarrier` NONE, an OPERATOR admission barrier, policy sha256 `8f016ff367708ddb741ec5a3435ec7d5b637c203e46d79ed4fae9edf387db05d` |
| Catalog | `docs/CAPABILITIES.md` at the source above |

Historical and current evidence stay separate. The MCP companion receipt is dated 2026-10-20 at
`a7870cac3` in the archived lineage (`docs/specs/mcp-server-2026-07-28-v0.md`, Companion promotion
evidence). It is historical: nothing in this entry re-ran it on the build above. The only
current-build qualification is the V1-3181 Tasks run
(`docs/build-log/2026-10-29-tasks-basic-stability.md`). Every other row below reports its catalog
maturity, not a fresh observation.

## Counted inventory

The counts are taken from the three `capability-gate` tables of `docs/CAPABILITIES.md`, which
`tools/capabilities-check` keeps complete against the code (CAP-V0-001). This entry does not
maintain a second catalog. When the counts drift, the catalog is right and this snapshot is stale.

| Surface | Rows | Core | Experimental | Other |
|---|---:|---:|---:|---|
| `corvint` commands | 52 | 12 | 40 | none |
| Binaries under `cmd/` | 55 | 1 (`corvint`) | 42 | 11 internal; 1 implemented (`corvint-remote-provider`) |
| MCP tools | 39 | 0 | 39 | none |

**Core frozen modes (12).** The commands are `init`, `adopt`, `index`, `query`, `context`, `impact`,
`affected`, `prove`, `cem`, `ocm`, `frontier` and `dogfood` (decision 0332, CCF-V1-001). Only the
modes and identifiers frozen by CCF-V1-002 carry the Core promise.

**Mixed-maturity exceptions inside Core commands (CCF-V1-003).** Each of these is excluded from the
freeze and stays experimental:

- `impact --provider*`, `--repository` and `--range-profile expanded-256`.
- `affected --snapshot`, `--playwright-config`, `--provider*` and `--selection-profile`.
- `prove --cem`, `--attest*`, `--checkpoint` and `--mutate`.
- The `context` lookup subcommands.
- The `cem/0.3` modes.

**Experimental commands (40).** `feature`, `features`, `overview`, `batch`, `depsource`,
`answerability`, `necessity`, `kernel`, `graph`, `test`, `tests`, `test-validity`, `test-plan`,
`runs`, `prove-observe`, `surprise`, `witness`, `breakage`, `migration-ratchet`, `dogfood-ocm`,
`obligations`, `lrf`, `review`, `delta`, `changes`, `step`, `repro`, `docs`, `flows`, `work`,
`lease`, `harness event`, `adapter`, `record`, `migrate-traces`, `eval`, `skill-export`,
`calibrate`, `observations` and `reads`. Some of them are classified as companions (`docs`,
`flows`, `test-validity`, `harness event`) in the release classification table
(`corvint-1.0-product-and-release-v1.md`). A companion classification is not a maturity label.

**Companion and other binaries (54 besides `corvint`).**

- 9 agent-facing experimental: `corvint-mcp`, `corvint-docs-mcp`, `corvint-corpus-mcp`,
  `corvint-test-validity-mcp`, `corvint-lsp`, `corvint-tasks`, `corvint-work-queue`,
  `corvint-intake` and `corvint-pulse`.
- 8 user-facing experimental: `corvint-console`, `corvint-dashboard-snapshot`, `corvint-update`,
  `corvint-web-flows`, `corvint-behavior-falsify`, `corvint-playwright-minimize`,
  `corvint-corpus-http` and `corvint-corpus-republish`.
- 6 test providers, experimental: `corvint-go-test-provider`, `corvint-js-test-provider`,
  `corvint-ruby-test-provider`, `corvint-test-runner`, `corvint-test` and `corvint-tests-accept`.
- 16 analyzer plugins, experimental. Each is admitted only through its own profile.
- 3 experimental rows classed internal: `corvint-corpus-parity`, `corvint-cem-candidate` and
  `corvint-cem-experiments`.
- 11 internal rows: the release, readiness, toolchain-receipt and postmerge tools. They are not
  public surfaces and carry no promise.
- 1 implemented: `corvint-remote-provider`. Its network consent keeps it outside the default local
  product.

**MCP surfaces (39 tools).** All 39 are experimental.

- `corvint-mcp` has 12. `corvint.query`, `corvint.impact`, `corvint.status` and `corvint.help`
  are in every profile. `corvint.context` and `corvint.cem.report` come with task-review and
  task-review-lsp. Five `corvint.flows.*` tools come with `flows`, and `corvint.graph.imports`
  comes with `import-graph`.
- `corvint-corpus-mcp` has 24.
- `corvint-docs-mcp` has 2.
- `corvint-test-validity-mcp` has 1.

**Unsupported.** Windows hosts are UNSUPPORTED for 1.0 (PRS-V1-004). The supported tuples are
darwin/arm64 and linux/amd64. Every refusal a row names in the catalog stays that row's unsupported
boundary.

## First-batch rows

### Basic MCP companion profile (V1-0191)

| Field | Value |
|---|---|
| Boundary | `corvint-mcp` over stdio, default and task-review profiles. Companion only (decision 0373); every host adapter stays `FALLBACK`. Not Core, and no new transport. |
| Requirements | `MCPV0-*` in `mcp-server-2026-07-28-v0.md` (intent: accepted companion profile; delivery: implemented, stdio). Catalog row: experimental. |
| Support tuple | darwin/arm64 and linux/amd64 under PRS-V1-004. The retained receipt does not name its tuple: `NOT_OBSERVED`. |
| Evidence | Historical only: on 2026-10-20 at `a7870cac3` the official-schema receipt passed (schema sha256 `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`), 35 conformance tests passed and the task-review read-only evidence passed. Current-build conformance: `NOT_RUN`. |
| Unmet gates | A current exact-source conformance and official-schema rerun. The ticket's historical failed `full-gate` (2026-10-21, `unsupported Go version` in `tools/corvint-pr-tests`) and its retry state must clear through native recovery, never manual completion. Fuzz, complete race, cross-build and Streamable HTTP stay outside the receipt (`NOT_RUN`). |
| Reused ticket | V1-0191 (OPEN, P2, milestone v1-0). |

### Basic Tasks external-agent profile (V1-3181)

| Field | Value |
|---|---|
| Boundary | `corvint-tasks` single-binary external-agent lifecycle on one native store: create, plan preview, claim, renew, release, submit, command gate, complete, readback and receipt audit. Dispatch, services, pools, mixed binaries, scale, Beamfall takeover and release promotion are excluded. |
| Requirements | CAL-V0-007..017, CAL-V0-019 and CAL-V0-069. The profile contract is the new `corvint-tasks-basic-stability-v1.md` (TBS-V1). |
| Support tuple | darwin/arm64 and linux/amd64, `go1.27.3`, one source-built binary per store. |
| Evidence | Before this batch: the CAL-V0-019 suite is defined (`internal/tasks/transaction/qualification.go`), and there are fixture-queue tests. There was no current-build lifecycle qualification on a non-fixture store, so it was `NOT_OBSERVED`. V1-3181 delivers that run. |
| Unmet gates | Owner acceptance of the qualified profile before any label change. The live store `queue:corvint:main` is still `fixture: true` with no execution cutover, and this batch does not change it. |
| Reused ticket | V1-3181 (OPEN, P1). It depends on this ticket. |

**Duplicate search.** Before anything new was filed, the queue was searched for the
first-batch work. Every gap this inventory names already has a native ticket: V1-0191, V1-3177,
V1-3181, V1-0596, V1-0757, V1-0232, V1-2685, V1-3182, V1-0921, V1-0607, V1-3185 and V1-3186. This
entry files no new ticket.

## Dependency order

1. V1-3180, this inventory.
2. V1-3177, the release-baseline repair. The first-release N-1 path was decided on 2026-10-30
   (`2026-10-30-n1-first-release.md`). The ticket stays OPEN until a passing Release gates run is
   observed for rc.3.
3. In parallel after step 1: V1-0191, MCP companion requalification, and V1-3181, the basic Tasks
   profile.
4. Owner acceptance of each qualified profile, per profile.
5. Separate obligations that this batch does not satisfy:
   - V1-0596: CEM 1.0 cross-language Tasks integration.
   - V1-0757: mixed binaries and non-lease writers.
   - V1-0232: release attestations against the policy gate argv.
   - V1-2685: discoverability.
6. v1.0 blockers for day-to-day Tasks use. They do not block the basic profile's qualification:
   - V1-3182: named plans.
   - V1-0921: precise search.
   - V1-0607: hook context delivery.
   - V1-3185: broad path reservations.
   - V1-3186: receipt-audit latency.

## Acceptance decisions still needed

- **Release baseline.** Accept V1-3177's repaired first-release baseline once a passing Release
  gates run exists. Until then, no release qualification exists for rc.3.
- **Supported companion or Core.** The release table already classifies `corvint-tasks` as a
  companion and `corvint-mcp` as a required 1.0 companion. Still open:
  - Does a qualified companion profile get a stability label of its own? PRS-V1-002 reserves the
    1.0 stability promise for Core.
  - Or does it stay experimental with a qualification record?
  Neither profile moves into Core without a CCF-V1 amendment.
- **Per-profile acceptance.** For each profile, the owner names the exact profile and its evidence
  before any catalog maturity label changes.

## Limits

This is a snapshot at the pinned source. A later change to `docs/CAPABILITIES.md` makes the counts
stale, and no check keeps them current, by design. Anything not re-run here is `NOT_OBSERVED` or
`NOT_RUN`, and nothing in it carries authority.
