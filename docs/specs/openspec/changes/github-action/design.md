# Design: `github-action`

## Decisions

- One plan artifact with a digest; shards filter by runner; a final job verifies that all shards
  used the same plan (owner decision R22, 2026-11-09). GHA-V1-018 fixes the artifact schema,
  GHA-V1-005 the shard rule and GHA-V1-010 the result job, which uses the existing
  `corvint test verify-shards` (`cmd/corvint/test.go`, integ 9ec3a29d0).
- Narrowing on `pull_request` and `push`, with the `push` base taken from `before`; fork pull
  requests always run everything (owner decision R23, 2026-11-09). The `pull_request` diff is
  `HEAD^1..HEAD` of the tested merge commit (GHA-V1-017, settled without the owner).
- Trust: actions pinned by SHA; `contents: read` plus `id-token: write` only for attestation; never
  check out pull request code under `pull_request_target` (owner decision R24, 2026-11-09).
- Kill switch grammar `full:<runner>` everywhere; an unknown value exits 2 in CI (owner decision
  R13, 2026-11-09). NMS-V1-008 owns the grammar.
- One digest per binary and platform until each provider binary is folded into `corvint` after G23
  (owner decision R14, 2026-11-09).
- Licence Apache-2.0 for `beamfall/corvint-action` (owner decision R25, 2026-11-09).
- Docs review, the docs drift step and the PR comment are advisory by default; an opt-in makes the
  drift step blocking (owner decision R4, 2026-11-09).
- Only Corvint configuration files under `.corvint/` force a full run; the flow file selects linked
  tests; the private ledgers add nothing (owner decision R31, 2026-11-09; NCM-V1-006).
- The receipt read by the ticket-gate step sits at the default location fixed by PROD-TEST-020
  (owner decision R7, 2026-11-09).
- The self-shadow on Corvint's own CI starts from the M5a qualification candidate (owner decision
  R2, 2026-11-09) and passes `--accept-stale-binary`.
- GHA-V1-040 to GHA-V1-043 stay in this change; change `acceptance-record` records their final home
  (owner decision R1, 2026-11-09). Decided 2026-11-09 under R42, D6: the four requirements stay
  here with their IDs unchanged; `release-process` cites them and does not restate them.
- Base on a force-push: an unfetchable `before` is an unresolvable base and runs every test; a
  fetchable one narrows normally (decided 2026-11-09 under R42, D1; GHA-V1-003).
- Cut-over and required statuses: `go-product` is the one context the action's result job
  replaces; `doc-gates`, `ci-control-plane` and `greptile-gate` stay; `ci-control-plane.yml` is
  trimmed, not deleted (decided 2026-11-09 under R42, D15; GHA-V1-031). The live ruleset is
  NOT_VERIFIED.

- Adopter gaps from review ci-agents F16 are each recorded once. Requirements, all preview:
  `always-run` (GHA-V1-027), `working-directory` (GHA-V1-028), the `merge_group` trigger in
  starters (GHA-V1-025), and `actionlint`, `zizmor` and Dependabot on the action repository
  (GHA-V1-029). Non-goals at 1.0: a durations feed, JUnit or annotations, a flaky-test policy, a
  `token` input and Scorecard. Decided 2026-11-09 under R42, N3: the split stands as written.
- Index cache (GHA-V1-026): key is binary digest plus commit, saved only by default-branch `push`
  runs, index digest checked on restore (review ci-agents F15).
- `github-split-check` keeps `.github` changes in their own pull requests until the cut-over.
  `source-removal` removes it in B11's script removal PR, after the cut-over PR merged (RMV-V1-032).

## Failure modes

- Binary download fails or a checksum does not match: the job fails with a typed message and runs
  nothing. It never runs an unverified binary and never falls back to a different version.
- Base cannot be resolved (shallow clone that cannot be deepened, all-zero `before`, base not an
  ancestor, unknown event): full run with reason `BASE_UNRESOLVED`.
- The binary refuses, times out or returns UNKNOWN or FULL for any runner: that runner runs fully
  with the binary's reason code; if the plan as a whole fails, every runner runs fully.
- The E2E inventory is stale: all E2E tests run (owner answer 4).
- A change touches a Corvint configuration file under `.corvint/` (NCM-V1-006), the narrowing map,
  the provider declaration file, the workflow that calls the action, or the action pin: full run
  (G15b, G16, R31).
- A pull request comes from a fork: full run with reason `fork-pull-request` (R23).
- The previous push's run failed or was cancelled: its commits drop out of the next `push` diff
  (GHA-V1-003 with `before`); the scheduled full run (GHA-V1-013) is the backstop.
- `CORVINT_SELECTION` holds an unknown value: the plan step exits 2 naming the value and the result
  job fails (R13).
- A shard reads a plan whose digest differs from the published one: it runs nothing and fails; the
  result job fails (GHA-V1-005).
- The plan selects nothing: the run job is skipped through `if:`; the result job passes only because
  the plan lists no expected shard (GHA-V1-010).
- The artifact read by the comment workflow is hostile: text is escaped, never evaluated, and only
  the workflow's own marked comment changes (GHA-V1-021).
- The self-shadow uses a released binary on Corvint's own repository without
  `--accept-stale-binary`: every plan widens to full through `self-binary-stale` and matches
  trivially; GHA-V1-030 requires the flag.
- The event is `pull_request_target`: the action refuses with a typed message (GHA-V1-008).
- A run shard is cancelled, fails or never starts: the result job fails.
- The runner OS is Windows: the action refuses with a typed message (answer 16).
- The job summary or PR comment would exceed its size bound: stable order, a truncation marker and
  totals; the full plan stays in the artifact.
- A restored index cache does not match the commit and the Corvint binary digest, or its bytes do
  not hash to the digest recorded at save time: it is discarded and rebuilt; a cache never narrows
  by itself. A pull request run never saves a cache entry (GHA-V1-026).

## Acceptance evidence

- Action repository tests on Linux and macOS for each scenario above, including a tampered archive,
  an all-zero `before`, a base-branch commit after the pull request opened (GHA-V1-017), a fork
  pull request, `pull_request_target`, a Windows runner, the kill switch
  global, per runner and with an unknown value, a cancelled shard, a crashing plan command, a plan
  digest mismatch, an empty plan, a failed plan job, a skipped run job, and a hostile artifact for the
  comment workflow.
- A conformance test in `beamfall/corvint` that the plan command's selections equal
  `corvint affected --base SHA` on the same inputs, bound to the GHA-V1-018 schema (GHA-V1-004).
- Each starter workflow run once in its runner's qualification repository (G12a), with the run URL
  retained. Preview items may stay NOT_RUN at 1.0 with that label.
- The shadow record on Corvint's own CI: 30 pull request numbers, each with both verdicts and an
  empty would-be-miss list, in one build-log entry (GHA-V1-030).
- One release candidate published through GHA-V1-040 and GHA-V1-041, with the approval event and
  `SHA256SUMS` retained, and the kill switch exercised on it (G10).

## Rollback

- Adopters: set `CORVINT_SELECTION=full` (immediate), then pin the previous `v1.x.y`. Released tags
  are never deleted; a bad action release is withdrawn by moving `v1` back through a new release and
  marking the bad release in its notes (G10 rollback order).
- Corvint's own CI: revert the cut-over pull request; the removed scripts return from history. Until
  cut-over nothing is removed, so rollback is deleting the shadow jobs.
- Release workflow: disable the workflow; release-gates remains as it is today.
- The 35 requirement IDs are retired, not reused, if the change is withdrawn.

## Contradictions and open questions for the owner

1. Platforms: `tools/release-binary/binaries.json:5-9` builds `darwin/amd64` and `windows/amd64` as
   well; G15a names three platforms and answer 16 makes native Windows unsupported. The release
   change must drop the two extra platforms or the owner must amend G15a. Settled without the
   owner: the release targets are `linux/amd64`, `linux/arm64` and `darwin/arm64` only; drop
   `darwin/amd64` and `windows/amd64`.
2. G27 says gate-ledger and gate-receipt are replaced at cut-over, but both are also the local
   `make gate` machinery (`Makefile:135-155`), and decisions 0224 and 0352 and the specs
   `gate-ledger-v0.md` and `go-archive-gate-v0.md` govern them. The superseding decision for G8/G27
   must state what records a gate after they go (proposed: the release gates of G8 plus the action's
   plan artifact).
3. Attestation and "only the publish job has write permission" (G15a): owner decision R24
   (2026-11-09) allows `id-token: write` for attestation only, and GHA-V1-041 counts only
   `contents: write` as write permission. Still open: whether `attestations: write` is also needed
   (NOT_VERIFIED), and whether the build job or the publish job produces the attestation (D17).
4. Resolved by owner decisions R23 and R24 (2026-11-09): fork pull requests run everything, and the
   action refuses `pull_request_target`. Checked: Corvint's own `ci-control-plane.yml` uses
   `pull_request_target` without checking out head code (integ 9ec3a29d0), which fits the
   documentation rule; the action's own refusal does not affect that workflow.
5. GHA-V1-016 applies the G15a approval pattern to the action repository; G17 does not say so.
6. Resolved by owner decision R25 (2026-11-09): Apache-2.0 (GHA-V1-015).
7. PROD-TEST-012 leaves "the length of the period" to the owner; G18's 30 pull requests is used here
   as that period for Corvint's own CI. Confirm it also serves as the PROD-TEST-012 acceptance, and
   whether seeded faults or replayed failures are required as miss evidence (D18).
8. `push` base: R23 keeps `before`. Whether to use the last commit with a successful run instead,
   which needs `actions: read`, is open (NEW-CI-1).
9. Whether the action repository also ships a reusable workflow (NEW-CI-2).
10. The requirement and non-goal split for the review ci-agents F16 adopter gaps (GHA-V1-027 to
    029 and five non-goals in `proposal.md`) was made without the owner. Decided 2026-11-09 under
    R42, N3: the split stands and no requirement changes.
11. The seven-repository acceptance of PROD-INT-018 and GHA-V1-025 runs the action on each
    qualification repository. That needs a fork or mirror of each one in an organisation where
    Corvint may add workflows (review ci-agents F17). Decided 2026-11-09 under R42, N4 (low
    confidence): seven public mirrors in the `beamfall` organisation, `corvint-qual-go`,
    `-vitest`, `-jest`, `-playwright`, `-pytest`, `-rspec` and `-minitest`, each a pushed copy of
    the pinned commit and not a fork, by 2026-11-27. Creating the repositories and setting them
    public is the owner's act. The acceptance stays NOT_RUN until they exist. The
    `proof-repositories` manifest records each mirror URL.
