# Decision 0321 — A repository adopts the work queue through `corvint work init`

Date: 2026-09-19. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#20; owner review of the intent is welcome and changes only the digest line).

## Decision

Beamfall/corvint#20 reports that `corvint work observe` returns `SOURCE_UNQUALIFIED` for every
repository, so `propose-wave` always abstains. The adoption path is delivered inside the existing
WQO-V0 contract, with no new wire profile, policy field, or store root:

- **`corvint work init --repository NAME`** (WQO-V0-047) writes the policy, an empty
  `corvint-worklist/0` worklist, and a two-line adapter script under `.corvint/`. It refuses when
  any of them exists and never stages or commits. The operator reviews and commits them.
- **`corvint work adapter`** (WQO-V0-048) is the producer. It is the one the self-dogfood adapter
  already runs, moved into `internal/worklistadapter` with a second closed mapping,
  `repository-worklist-v0` (`.corvint/worklist.json`, route `agent`). Verification work such as
  suite batches, failure-classification repairs, test-validity receipts, and cleanup/retry is
  expressed as ordinary tickets whose `touchPaths` drive WQO-V0-043 clashes.
- **Store scope by exact reproduction** (WQO-V0-046). WQO-V0-017 withholds complete store scope
  because a policy declares no external store roots. For `repository-worklist-v0` the observer
  recomputes the mapping from the qualified committed tree and requires byte-identical snapshot,
  details, and checkpoint documents. When they match, the committed tree is the whole store, so the
  existing complete pre/post manifests are complete scope and the observation can be `VALIDATED_AT`.
  Containment, executable identity, OS mutation enforcement, and network stay explicit unknowns.
- **Self-dogfood unchanged.** Corvint's own `decision-0046-v0` mapping does not qualify. Its
  existing observations, final-check fixtures, and the WQO-V0-017 paragraph stay exactly as they
  were. That keeps the blast radius to adopted repositories.
- **Companion name.** The Tasks companion source is `github.com/Beamfall/corvint-tasks` and its
  binary is `corvint-tasks`; the release gate's default checkout moves from `corvint-taskman`. The
  historical names in earlier decisions are not rewritten.

## Alternatives weighed

- *A store-root field in the policy*: this widens `work-queue-policy/0` and needs an
  enumeration of arbitrary external stores. That is the scan WQO-V0-017 refuses.
- *Trust the adapter's own completeness claim*: an adapter is repository-owned code; its output
  cannot establish its own scope. Reproduction by Corvint can.
- *Qualify both closed mappings*: equally sound, but it changes the self-dogfood contract and
  retires the incomplete-scope fixtures that the WQO-V0-021/025/032 final-check witnesses rely on.
  It can be proposed separately.
- *Ship the adapter only in the Tasks companion*: the observer runs the adapter under the fixed
  VPO-V0-022 `PATH`, and one binary serving both sides removes a version-skew failure.

## Rollback

Remove `work init` and `work adapter` from `cmd/corvint`, and remove the mapping check in
`workMappingReproduced`. Observations return to `UNKNOWN/SOURCE_UNQUALIFIED`. Committed
`.corvint/` files in adopting repositories are inert without the verbs.
