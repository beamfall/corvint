# Release-gate lane rel-gates-w1 — V1-3177, V1-3055, V1-2555

Base `3555875f94e9fd717ccb9f84d0aabf638adbe45d`. Three release-gate tickets: the N-1 replay
route, the red nightly full gate, and the local rc.3 candidate rebuild.

## V1-3177: N-1 replay against corvint-history

Decision: no code change. The ticket asks the release gate to acquire the N-1 predecessor from the
archived `corvint-history` repository and refuse when it cannot. The owner decision of 2026-10-30
(1.0.0-rc.3 is the first release; the pre-1.0 releases are gone) superseded that route, and
`9dc17baf8`, on this base, delivered it: `.github/workflows/release-gates.yml` records
`install-lifecycle-n1` as `NOT_APPLICABLE FIRST_RELEASE` with a workflow notice, a step-summary
line and a gate record, and `docs/RELEASE-RUNBOOK.md` documents the route. The row is visible, not
silently skipped. Turning it back into a refusal would contradict the owner decision and the
CCF-V1-007 amendment, so project authority wins over the stale acceptance text.

- AC1, AC3, AC4, AC5 (history acquisition, refusal, replay pin): superseded by the owner route.
  Nightly run 41202020938 at `780f9e970` shows `core-n1-replay` and `linux-core` green.
- AC2 (the lifecycle N-1 row against a published predecessor archive): BLOCKED externally.
  `beamfall/corvint` has no published release and `corvint-history` is unreachable for replay.
  It unblocks when the owner publishes rc.3 and a later candidate runs the gate.

## V1-3055: nightly full-gate failures

Nightly runs 40988175187 (`9477995d7`), 41059652620 (`21fea70cb`) and 41202020938 (`780f9e970`)
failed only in the `full-gate` rows on macOS and Ubuntu. Root causes:

| Failure | Cause | Outcome |
| --- | --- | --- |
| `cross-vet`, `license-headers-check` | fixed on base | ALREADY-DELIVERED by `b05325e24` |
| `no-python-runtime-dependency-test` | `script/qualify-tasks-basic.sh` called `python3` (GOC-V0-008 forbids it) | fixed: identity and verdict built with `jq` |
| `qualify-tasks-basic-test`, Ubuntu exit 2 | the profile1 wrapper ran under dash, which exits 2 on `set -o pipefail` even behind `\|\| :` | fixed: wrapper has a bash shebang |
| `cmd/corvint` Git spawn counts of 0 | the in-process `corvint mcp` server's `gitstatus.Pin()` hid the counting shims | ALREADY-DELIVERED by `5983f6985`; the six spawn-count tests pass at this head |
| `batch-precheck-test` case 10, macOS | macOS `TMPDIR` ends in `/`, so the test's path held `//` while the script resolved it with `cd && pwd` | fixed: the test resolves `$tmp` the same way |
| `TestATRV0027_StartingChildIsNeverLost`, macOS | `SUPERVISOR_LOST` after the 60 s attach wait on a loaded runner; passes `-count=5` locally and on Ubuntu | flake: ticket proposal, no retry |

Flake proposal (TestATRV0027): run 41202020938 failed at 59.57 s; the starting child (pid 98209,
started 15:05:02Z, ceiling 15:15:02Z) never committed before the `--wait 60` attach expired. The
test's fixed wait does not scale with runner load; the proposal is to bound the wait by the child's
ceiling or report the starting state instead of `SUPERVISOR_LOST`.

Wanted after merge: `gh workflow run release-gates.yml --ref <batch head>` to confirm the full-gate
rows on both operating systems.

## V1-2555: local rc.3 candidate

The candidate was rebuilt at lane head `5f29067224b0e99c80d133293ffd782d2a6c3cde` (base plus this
lane's two script fixes; the archiver refuses a revision other than the checked-out HEAD) with
`GOTOOLCHAIN=local` go1.27.3, following runbook step 5, into a private 0700 directory outside the
repository. Nothing was published, tagged or uploaded.

- `release-artifact-v0 archive`: `verdict=PASS`, 5 targets, toolchain `go1.27.3`, tree
  `90906da995f37ab1af70f53f2de9263f3b9c83e6`, manifest sha256
  `17409147f4bce43e7de62174331e08177d6f520cc65f1f0eb8ebaa20468d511d`; `shasum -a 256 -c
  SHA256SUMS` OK for all five archives.
- `make companion-release-gate`: rc=0, 33 `[ok]` checks, no failures; the build states that
  browser/UI qualification is a separate step and is not claimed.
- `script/public-release-check`: NOT_RUN. Core qualification needs a protected Go authority bundle
  (`CORVINT_GO_AUTHORITY_BUNDLE`, decision 0382), which ASD-V0-007 forbids inferring.
