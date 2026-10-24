# skh-a: site knowledge stage 5 — candidate flows from test step sequences

Date: 2026-10-24. Lane: cloud skh-a (V1-1902). Spec: `docs/specs/site-knowledge-howto-v0.md`.
Platform for every result below: linux/cloud. Dogfood targets: NOT_PRODUCED (cloud lane).

## Owner authority

New requirements `SKH-V0-120`..`126` come from the lane's reserved block `SKH-V0-100`..`139`.
They are accepted under the owner's standing auto-accept (2026-10-11).

## Decisions

- **Clustering key (SKH-V0-050, 121).** A candidate is the set of tests with the same start and
  end screen, read from their first and last `visit`. A variation is the set of tests with the
  same action-segment sequence. A page-object or helper run is one `call` segment, so tests that
  reuse a method share a segment even when the method's inner steps are long. The rule is
  deterministic and lexical, with no similarity threshold. Tests that diverge at a step are
  separate variations and are reported as conflicts. They are never merged by guessing.
- **Trust joins without importing the corpus into appmap (SKH-V0-052, 123, 126).** The inference
  takes plain `TestTrust` values: a test anchor span plus the labelled trust level, verdict,
  selection and gap. The CLI fills them from a built corpus artifact opened with
  `doccorpus.Open`, so the DCP-V1-090 selection is the build's own, not recomputed. A corpus
  built at another revision is refused: its line spans would name other source.
- **Import through the existing AFU-V1 path (SKH-V0-051, 125).** `--emit request` prints a
  `corvint-behavior-adapter-request/1`. `flows import --format behavior-adapter-request` already
  reads that format and writes proposed intents only, so no new import format is added. The
  adapter needs envelope inputs (migration, discovery, candidates, tests). The emitted ones are
  inline, have no committed file, and carry the reason "generated, not committed, nothing ran".
  The discovery input lists no executions, so it claims no run. Before printing, the producer
  runs the request through `doccorpus.BuildBehaviorAdapter`, as `flows export --emit request`
  does.
- **What cannot be emitted is named (SKH-V0-125).** The adapter requires a non-empty outcome
  value. A valueless state matcher uses its DCP-V1-084 canonical value; any other assertion
  without a literal expected value is not emitted, and the candidate's `emit` is
  `no-emittable-outcome`. Projects are `unknown`, because Playwright projects are not read here.
  Blocked and incomplete candidates are reported but never emitted.

## Evidence (linux/cloud)

- `TestSKHV0_050_054_FlowInferenceTrustAndImport` in `internal/appmap`: ok. It covers a
  three-test candidate with two variations, where a `run+reviewed` partial source with support 1
  is primary over two unreviewed sources with support 2. It also covers:
  - a `run+reviewed` defect blocking its two-test candidate;
  - an `unrelated` test excluded;
  - a no-visit test left unclustered;
  - an opaque-step candidate that is incomplete and single-source;
  - worktree, index, refs and ignored files unchanged;
  - `appflows.Import` of the emitted request writing one proposed intent whose variation actions
    equal the candidate's.
- `TestSKHV0_126_InferCLIAndImport` and `TestSKHV0_126_TrustFromJoins` in `cmd/corvint`, and
  `TestSKHV0013CorpusMCPMapKnowledge` in `cmd/corvint-corpus-mcp`: ok.
- Real-adopter candidate flows: NOT_RUN. The first adopter is not reachable from the cloud
  session.

## Limits

- The end screen comes only from the last `visit`. A click that navigates, with no later visit,
  leaves the start screen as the end screen.
- No fixture builds a full corpus artifact with test knowledge. The join is tested at the
  conversion boundary.
