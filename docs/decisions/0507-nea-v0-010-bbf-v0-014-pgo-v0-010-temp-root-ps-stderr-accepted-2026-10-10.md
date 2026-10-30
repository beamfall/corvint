# Decision 0507 — Acceptance temp root and bounded ps stderr (NEA-V0-010, BBF-V0-014, PGO-V0-010) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #760 proposed `NEA-V0-010` in `docs/specs/new-e2e-test-acceptance-v0.md` and `BBF-V0-014` in `docs/specs/browser-behavior-falsification-v0.md` (V1-0686: an operator-only `accept --temp-root` per-run TMPDIR carried to workers, providers and control hooks) and `PGO-V0-010` in `docs/specs/process-group-owner-v0.md` (V1-0691: POSIX descendant snapshots bound `ps` stdout to 4 MiB and stderr to 16 KiB). The owner first accepted all three on 2026-10-10 ("acceptance is given"). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `NEA-V0-010`, `BBF-V0-014` and `PGO-V0-010` as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0507".

## Limits

The evidence is the `TestNEAV0010*`, `TestBBFV0014*` and `TestDescendantSnapshot*` tests named in the PR build logs. Linux and unsandboxed production `ps`, live qualification and `make gate` are `NOT_RUN`. The `--temp-root` scope is cooperative output placement, not filesystem containment. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the three status texts to proposed. To withdraw the behavior, also revert the `--temp-root` changes in `internal/testacceptance` and the bounded capture in `internal/procgroup/descendants_posix.go`, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
