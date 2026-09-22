# Compatibility trial v0 comparator baseline

Status: `SPECIFIED_NOT_FROZEN`

Authority: CTR-V0-005/009 and the coordinator delegation in decision 0052. This document freezes
the comparator's contract and records missing artefacts explicitly. It does not claim a baseline
run exists.

## Comparator contract

The primary comparator is a capable agent operating a straightforward differential harness over the
same pinned snapshots as the Corvint arm. The harness supplies `git diff`, `git bisect run`, `go test`,
and an exact byte differ. It has no Corvint access and no weaker resource or tool allowance than the
Corvint arm. The baseline is operator-run and is not part of the shipped Corvint or replay binary.

An independent reviewer who is not the Corvint-arm operator improves the comparator on the ten pilot
cases before the freeze. Arm order is randomized from a recorded seed; separate operators prevent
cross-arm learning. Each case has one attempt and a 45-minute human-active cap per arm. Both arms
receive identical revisions, accepted intent, fixtures, seeds, access, build availability, and total
budget.

## Required pins

| Artefact | Path | sha256 | State |
|---|---|---|---|
| differential harness executable/script | `NOT_PRODUCED` | `NOT_PRODUCED` | missing |
| frozen prompt | `NOT_PRODUCED` | `NOT_PRODUCED` | missing |
| model identity and dated release-build record | `NOT_PRODUCED` | `NOT_PRODUCED` | missing |
| tool allowlist | `NOT_PRODUCED` | `NOT_PRODUCED` | missing |
| budget/operator-proficiency/tuning configuration | `NOT_PRODUCED` | `NOT_PRODUCED` | missing |

Placeholder digests are forbidden. The comparator is specified but not frozen until every row has a
real path and a recomputed lowercase sha256. The first scored run is therefore `NOT_RUN`.

## Replay old side

The replay `old` executable is distinct from the comparator arm. For each case it is built from the
candidate commit's parent with `go1.27.0`, `GOTOOLCHAIN=local`, and `-trimpath`; old and new build
identities must match. Its executable digest belongs in the future CTR-V0-001 descriptor. Gold is
accepted before execution and is never derived from a run of `new`.
