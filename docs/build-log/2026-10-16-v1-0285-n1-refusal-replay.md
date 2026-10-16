# 2026-10-16 V1-0285 N-1 refusal replay: the release replay reaches all twelve Core verbs

Ticket V1-0285 (panel B6). Base `294542f02c8cef48b50c2babcd17504339019eea` (origin/main). Requirement
CCF-V1-010, accepted under the owner's standing auto-accept of 2026-10-11 (new lane requirement; no
accepted requirement is amended).

## Premise check at the base

Each element of the finding's Fix was re-checked at the base before any change.

- N-1 premise: already fixed (a6f89b4e7, decision 0401). `grep -i "nothing changed"` over the spec
  finds nothing; the scope section and CCF-V1-007 name `v0.8.1` and explain why not `v0.7.0`. Tags
  present: `v0.8.0`, `v0.8.1`, `v1.0.0-rc.1`, `v1.0.0-rc.2` (no later release); decision 0434 names
  `v1.0.0-rc.1`, which the spec cites.
- Frozen enumeration table with values and open/closed status and the add-a-value rule: already
  fixed (CCF-V1-007 (d), 37 rows; `coverage.unexamined[].relation` lists `instruction-routed` and is
  `open`; CCF-V1-006 makes a `closed` addition breaking). The regression test that the table matches
  what the code emits already exists: `observeCoreEnumerations` parses the table from the spec and
  fails when a frozen mode emits a value outside its row or a row is reached by no mode.
- Release check building the previous tag offline: already fixed (`make core-n1-replay`, decision
  0398 / V1-0350) and wired into `.github/workflows/release-gates.yml` (`core-n1-replay-v0.8.1`,
  `core-n1-replay-v1.0.0-rc.1`, V1-1382). No `.github/` change is needed.
- Residual open (build log 2026-10-14-v1-0285-closeout-evidence): the replay reached nine of the
  twelve Core verbs. `cem` and `ocm` success modes are skipped (decision 0525 §6) and `frontier` has
  no success mode (CEM-V1-212). This change closes that residual.

## Decision

Replay each Core verb's pinned CCF-V1-004 refusal under the N-1 binary (`replayCoreRefusalN1`, run by
`TestCoreRefusalsKeepTheFrozenEnvelope` when `CORVINT_CORE_N1_BINARY` is set) and add that test to the
`core-n1-replay` run pattern. The comparison follows CCF-V1-004 and CCF-V1-006: same exit class, empty
stdout, one JSON line; the frontier document exactly; an N-1 code must be this build's code, while a
codeless N-1 refusal may gain one; every N-1 envelope member path keeps its JSON type here.

Rejected:
- Unskipping `cem`/`ocm` success modes for the pre-release tags: observed 2026-10-16 with the skips
  removed, `v0.8.1`, `v1.0.0-rc.1` and `v1.0.0-rc.2` all refuse the `cem/1.0` fixture (`cem status` and
  `cem verify` exit 2 `invalid-arguments` on `--artifacts`; `ocm status` and `ocm verify` exit 1).
- A separate script: the existing make target already builds the tag from `git archive`; a second
  runner would duplicate it.

## Acceptance evidence

Host load averages 48-73 throughout (shared 12-core host).

- Refusal replay with prebuilt N-1 binaries: 12/12 `N-1` subtests PASS against each of `v0.8.1`
  (17.8 s), `v1.0.0-rc.1` (17.2 s) and `v1.0.0-rc.2` (9.8 s), load 59-64.
- Mutations (stand-in N-1 binaries): a renamed code fails 12/12; exit 1 instead of 2 fails 12/12; an
  extra envelope member fails 12/12 (11 by the member rule, `frontier` by its exact-document rule).
- `GOPROXY=off make core-n1-replay CORE_N1_TAG=v0.8.1`: exit 0, 79 s wall including the tag build
  (`ok ... cmd/corvint 50.659s`), load 61. `CORE_N1_TAG=v1.0.0-rc.1`: exit 0, 65 s wall (38.3 s
  test), load 60. The module declares no dependencies at these tags, so the build is offline.

## Retained limits

- Codex review (one round, read-only, static): no findings.
- `cem`, `ocm` and `frontier` success modes still have no N-1 replay; only their refusals replay.
- Only the twelve pinned refusal cases replay; other refusals stay unreplayed.
- The CCF-V1-002 identifier row for `cem status` / `cem verify` still says `verification.spec` =
  `cem/0.2`, while the test pins `stableVerification.spec` = `cem/1.0` (V1-1278). Correcting it amends
  an accepted requirement and is left for the owner.
- `make gate` NOT_RUN here; the orchestrator runs full-gate at closeout. Hosted release-gates run of
  the new pattern NOT_RUN.

## Rollback

Revert this change: `replayCoreRefusalN1` and the helpers it split out, the Makefile run pattern,
CCF-V1-010 and the runbook sentence. No stored state changes.
