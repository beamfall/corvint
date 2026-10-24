# skh-a: site knowledge stage 3 completed — Rails setup, setup projects and the condition query

Date: 2026-10-24. Lane: cloud skh-a (V1-1900). Spec: `docs/specs/site-knowledge-howto-v0.md`.
Platform for every result below: linux/cloud. Dogfood targets: NOT_PRODUCED (cloud lane).

## Owner authority

New requirements `SKH-V0-100`..`105` come from the lane's reserved block `SKH-V0-100`..`139`.
They are accepted under the owner's standing auto-accept (2026-10-11). Collision check:
`docs/specs/REQUIREMENTS.tsv` at origin/main holds no `SKH-V0-1xx` ID.

## Decisions

- **Reuse V1-1891, not a second Rails reader (SKH-V0-032, 100).** The AMAP-V0-114
  setup-writer index already read factories, traits, YAML fixtures and setup endpoints, including
  the model writes of their handlers. The setup report loads the same application roots, with the
  same read predicate and shared read budget as `branches`, and converts each writer into a typed
  record. Seeds needed one change: the endpoint's per-line write scan is now
  `setupWriterIndex.lineWrites`, and its per-model accumulator is `writeSet`. Endpoints and
  `db/seeds*.rb` share both. Seeds are kept out of the index's writer list, so the branch query's
  output is unchanged.
- **Fixed in the shared scan.** `FactoryBot.create(...)` matched the model-write pattern as a
  write to a `FactoryBot` model. It is now skipped there and read through its factory. This also
  removes a bogus `FactoryBot` writer from any setup endpoint that calls FactoryBot in the branch
  query.
- **V1-1883 reuse is partial.** The seed-parity normaliser lives in `internal/testsource`, which
  imports `internal/appmap`, so appmap cannot call it. Records reuse only its TSA-V0-102 name
  rule (lower snake case). Moving the normaliser below appmap is a follow-up.
- **TSA-V0 setup-select is not joined.** It needs a context index that this report does not build.
  Setup projects are read from a literal `projects` list in `playwright.config.*`. A `testMatch`
  is evaluated only when it is a flag-free or `i` regular expression, or a glob without `**`.
  Anything else is UNKNOWN with a reason; nothing is guessed.
- **Condition query (SKH-V0-031, 103).** The query is lexical only (`SKH-V0-004`). It matches,
  in order: a literal target (a `storageState` read cannot produce its own state), then
  `Entity.attribute`, then entity words. Flow preconditions in the map are answered by the same
  function, so "preconditions link to the records that produce them" uses the code under test.
- **Profile `/1` (SKH-V0-105).** `not_read` changes from a list of names to `{source, reason}`
  objects, so the change is not additive. The profile moves to `site-knowledge-setup/1`. No
  command reads either version back, so there is nothing to keep readable. The `/0` producer is
  gone, not kept alongside.

## Evidence (linux/cloud)

- `TestSKHV0_100_105_RailsSetupProjectsAndConditions`, `TestSKHV0_030_031_SetupRecordsAndTestLinks`
  and `TestAMAPV011*` in `internal/appmap`: ok.
- `TestSKHV0_103_SetupConditionCLI` and `TestSKHV0_013_PageObjectsAndStepsCLI` in `cmd/corvint`:
  ok.
- `cmd/corvint-corpus-mcp`: ok.
- Real-adopter sample: NOT_RUN. The first adopter is not reachable from the cloud session.

## Limits

- Ruby `before`, `let` and `let!` bodies are not followed when Ruby tests are linked.
- Setup endpoint parameters are named only as `*` when the handler writes request attributes.
- Cleanup is reported only where it is read: `after-use`, `after-each`, `after-all` and
  `teardown:NAME`.
