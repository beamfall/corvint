# 2026-10-19 V1-1897: site knowledge "show me how to do X" spec

Docs-only lane for V1-1897. It adds `docs/specs/site-knowledge-howto-v0.md` (prefix `SKH-V0`),
which records the owner's intent of 2026-10-19 verbatim and stages it into eight slices: page
objects (V1-1898), step IR (V1-1899), data setup (V1-1900), navigation edges (V1-1901), flow
inference (V1-1902), the `corvint how` task query (V1-1903), generator (V1-1904) and proof
pipeline (V1-1905).

## Acceptance

- The intent is owner-written and accepted; the spec quotes it verbatim, including its spelling.
- `SKH-V0-001`..`009`, `010`..`014`, `020`..`023`, `030`..`034`, `040`..`043`, `050`..`054`,
  `060`..`064`, `070`..`074` and `080`..`089` are accepted under the owner's standing auto-accept
  of lane requirements (2026-10-11); V1-1897. Each marker says `standing`, so
  `make spec-acceptance-audit` lists them until the owner reads them back.
- Delivery is `not-started`. No code changes in this lane.

## Decisions recorded in the spec

- **Confidently** has six conditions, all on the exact candidate bytes: a live pass with repeats,
  positively established TSA-V0 validity, a complete NEA-V0 assessment, an allowlisted
  discrimination verdict with every control killed, behavioural witnesses for every line's
  elements, and an outcome assertion equal to the oracle accepted for the request before the run.
  The first five alone read `EXECUTION_VERIFIED`, never `PROVEN`. Anything else abstains with a
  named gap.
- **No synthesis.** Because CCE-V0 compilation was killed (V1-1677: 0 of 10 changes useful,
  `docs/build-log/2026-10-18-cce-spike-v1-1677.md`), the generator only copies mined statements
  and assertions with parameter-slot substitution.
- **Preregistered measure.** N owner-written requests (N proposed 20), frozen with a SHA-256
  before the first run. Fewer than 40% answered by a passing, discriminating test kills stages 7
  and 8. 0 unanchored statements and byte-identical reruns are hard gates.
- **Constraints.** Browser and Node stay in companions (invariant 7); lexical matching only, no
  embeddings; mined steps and flows enter only as proposed AFU-V1 intents (invariant 8);
  `storageState` and typed values pass `internal/secretscreen` and PWP-V0 `/2` redaction.
- **Assumed first adopter** (owner must confirm): Rails + AngularJS ui-router, Playwright E2E,
  RSpec and legacy Jasmine, because AMAP already resolves `ui-router-states/0` and
  `rails-routes/0`. A React adopter would make V1-1712 a prerequisite.
- Known gaps cited: V1-1712 (JSX `<Routes>` gives 0 screens), V1-1236 (inherited and helper
  page-object methods), V1-1191 (link-only crawl).

## Collision checks

- `SKH-V0` is not used in `docs/specs/INDEX.json` at origin/main `6ceee260`, and no open pull
  request (#1165, #1166, #1176, #1178, #1179, #1184) adds an `SKH-V0` ID or this spec path.
- `DCP-V1-086`..`092` (V1-1868, PR #1165) merged at origin/main `bda19b02` during this lane; the
  spec cites them as merged contract after the one main merge.
- No decision number is taken.

## Independent review (Codex, one round)

Codex reviewed PR #1188 and raised 11 findings, all fixed in the spec:

1. Discrimination failed open. `SKH-V0-085` allowlists `DISCRIMINATES` and `NEW_REACH` and maps
   every other `CCE-V0-009` verdict, or none, to a blocking code.
2. An empty finding list was read as validity. `SKH-V0-086` needs one located, blob-bound example
   with explicit `ELIGIBLE` hygiene; abstentions and `uncertain` files block.
3. NEA-V0 axes were missing. `SKH-V0-082` now requires the complete NEA-V0-004/006 assessment and
   keeps blocked axes in the receipt.
4. Proof did not show the test demonstrates X. `SKH-V0-087` binds each request to an accepted
   oracle before the run, and `EXECUTION_VERIFIED` is split from `PROVEN`.
5. The `test.fixme` guard deadlocked proof. `SKH-V0-088` defines the companion-only transform and
   binds the same candidate bytes through every check to the final artifact.
6. Trust ranking ignored defects. `SKH-V0-052` honours the DCP-V1-090 verdict and selection, and
   the new `SKH-V0-054` adds the fixtures.
7. Generation exceeded proof support. `SKH-V0-070` restricts V0 to Playwright (orchestrator
   decision under delegated forks, revisable by the owner).
8. The closed IR lacked `opaque`. `SKH-V0-020` adds it with `reason` and span.
9. Flow inference had no write boundary. `SKH-V0-051` emits only an import document,
   and `SKH-V0-054` adds the no-mutation read test and the import round trip.
10. Selector and method verification had no witness. `SKH-V0-089` defines the behavioural
    witness and its negative fixtures.
11. Rollback assumed a chain. The spec records the stage dependency graph, reverts dependents
    first, and `SKH-V0-007` makes readers refuse newer record versions.

## Owner questions

The spec's "Owner questions" section lists them: confirm the adopter and local run approval,
confirm N and the 40% threshold, and name the Ruby page-object form. The Playwright-only V0
generator is decided by the orchestrator (finding 7) and stays revisable by the owner.
