## 2026-09-27 CAL-V0-022/026: explicit pack scope derivation and lock measurement (S8 remainder)

The owner selected explicit pack opt-in for S8 and conservative default claims in this session.
The default gob snapshot key hashes the executable, so `corvint-tasks` cannot reuse one written
by `corvint`. A two-binary probe observed that miss, then loaded the same tree successfully
through the existing analyzer-schema-keyed pack with `CORVINT_SNAPSHOT_FORMAT=pack`.
This change keeps that format opt-in and experimental; it does not promote the index format.

CAL-V0-022 uses the existing context loader and TaskContext in process. It never builds an index,
changes ranking, invokes another Corvint executable, or writes outside task state. The base tree,
title, body, analyzer schema and returned packet are committed to the derivation digest.
Default format, absent/stale/dirty snapshots, unsupported context, omitted or withheld paths,
missing critical evidence, governance refusals and budget shortages all abstain to
`WHOLE_REPOSITORY`. A successful scope contains bounded, validated, pinned nongovernance paths.

Named claims keep DECLARED, REQUESTED, DERIVED, WHOLE_REPOSITORY precedence. `claim --next` uses
the existing conservative priority plan to select one ticket, derives that ticket only, and
binds the facts to its ID. The model independently selects again and checks the final scope's
collisions. A plan blocked by its conservative resources stays blocked; the deriver does not
skip a higher-priority ticket to improve parallelism.

CAL-V0-026 remains NOT_MET: each lease still inventories, audits and decodes the ticket store,
and there is no verified audit cache. The opt-in `TestCALV0026_LockHoldMeasurement` imports 3,000
synthetic tickets and records 20 claim, renew and release samples, with command duration and
actual successful flock-acquisition-to-release duration. A context-owned observer records no
store state and has no effect on admission. The measurement report retains the failed structural
clause regardless of latency. Host load and measured source revision accompany the run.

Independent review found the general snapshot loader could fall back to an executable-specific
gob after a pack miss. The dedicated pack-only loader now refuses that fallback, with missing
and corrupt pack regressions. The general loaders retain their existing fallback behavior.
CLI tests cover named and next claims, declared/requested precedence, conservative default
and unselective queries, and unchanged index files.

Measurement: pending execution on frozen source.

Verification: focused scope/claim/lock regressions, the required documentation checks and one
independent review. Repository-wide `make gate` is NOT_RUN under the owner's scoped-work rule.
Corvint query/affected and CEM/OCM bind this change; retrieval/ranking evaluations and optional
providers are not applicable because the existing ranking implementation is unchanged.

Rollback: revert S8. Claims without declared/requested paths revert to whole-repository scopes;
stored DERIVED attempts retain the existing scope codec and enforcement. Removing the observer
only removes timing evidence. No index format or store schema migration is introduced.
