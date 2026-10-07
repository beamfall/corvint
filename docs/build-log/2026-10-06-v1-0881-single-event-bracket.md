# 2026-10-06: V1-0881 one repository bracket per dogfood event

## Intent

Ticket V1-0881 (no GitHub issue). A dogfood event (`corvint dogfood event`, behind every
`adapter claude-code` and Codex hook) probed the repository twice, once before its reads and once
after, and the index loader took its own identity-and-status reads between them. On the primary's
roughly 2,500-path dirty worktree under load each probe cost about 400 ms against an effective
adapter budget of about 1.5 s, so the hooks degraded to `dogfood-event-deadline`. Owner goals
(2026-10-06): large projects without slowdowns; never burn CPU unless essential.

## Change

- `docs/specs/local-completion-policy-v0.md` adds `LCP-V0-016` (proposed, not accepted): one
  `GPK-V0-007` bracket per event, no second probe, no loader reads, no `ls-tree`; drift still
  refused as `dogfood-event-repository-drift`, never retried. Refusal-table citations re-pinned.
  `docs/specs/go-production-kernel-migration-v0.md` adds `GPK-V0-076` (proposed, not accepted) for
  the kernel primitive. `docs/specs/README.md` rows name both; `INDEX.json` status strings are
  unchanged, so are the specs' intent and delivery headers.
- `internal/gokernel/repository.go`: `ProbeRepositoryAround(ctx, root, read)` takes one
  observation, runs `read` against it as `Observation`, takes the closing observation and returns
  `ErrRepositoryDrift` when the identity or dirty set moved. `Observation.Repository()` is the
  opening observation in `Repository`'s shape with an empty profile; a nil dirty set digests as
  `[]`, exactly as the status parse yields a clean tree, so the two sides compare equal.
- `internal/contextindex/snapshot.go`: `LoadSnapshotObserved(root, observation)` hits the full
  snapshot the observed tree names and applies the observed dirty set and status digest without a
  Git process. A miss still builds in memory, under the same deadline forecast (`AHI-031`).
- `cmd/corvint/local_completion_event.go`: `localEventRead` runs the policy evaluation, envelope
  and context packet inside the bracket; `localEventContext` loads through
  `loadSnapshotObserved`. The context-drift, policy-drift and expected-target checks are unchanged.
  `qualified_lifecycle.go` threads the observation through its packet function.

## Decisions

- **No retry.** `GPK-V0-058`'s harness path retries once on drift; the dogfood surface has always
  refused drift (`dogfood-event-repository-drift`), and a retry would double the cost the ticket
  removes on exactly the worktrees that drift. The refusal keeps the fixed native envelope.
- **No profile read.** No event envelope emits `ProfileID`; the profile is a function of the tree,
  so an equal closing tree already implies an equal profile. `before != after` keeps its meaning.
- **Loader stays zero-spawn.** `LoadSnapshotObserved` trusts the bracket's observation; drift
  between the observation and the snapshot read is the closing observation's job. The loader's
  own `LoadSnapshot` and the harness `LoadEventSnapshotObserved` are untouched.
- Found while measuring, not fixed here: the `status --porcelain -z --untracked-files=all` scan
  with `core.fsmonitor=false` re-lstats every file on each observation (0.25-0.5 s at 50k files,
  so two per event still bound the hook at about 1 s); every snapshot hit decodes the whole
  59 MB gob; a tree change rebuilds the full snapshot (V1-0328); the 128 MiB aggregate source
  bound refuses kubernetes outright.

## Evidence

Measured on Darwin, 1-minute load average 15-21 (other lanes running), `GOMAXPROCS=3`, each median of 5.
Before: `TD/bin/corvint` built from `76f7f2ac`; after: the same tree plus this change.

| Repository | Hook | Before wall s / CPU s / spawns | After wall s / CPU s / spawns |
|---|---|---|---|
| r5k clean | session-start / user-prompt / stop | 0.38 / 0.60 / 13; 0.40 / 0.60 / 13; 0.30 / 0.48 / 10 | 0.36 / 0.41 / 4; 0.36 / 0.39 / 4; 0.27 / 0.34 / 4 |
| r5k 2,500 dirty | session-start / user-prompt / stop | 0.47 / 0.69 / 13; 0.68 / 0.77 / 13; 0.39 / 0.55 / 10 | 0.34 / 0.41 / 4; 0.37 / 0.41 / 4; 0.35 / 0.40 / 4 |
| r5k 5,001 dirty (every file) | session-start / user-prompt / stop | 0.58 / 0.79 / 13; 0.74 / 0.79 / 13; 0.63 / 0.62 / 10 | 0.30 / 0.34 / 4; 0.29 / 0.34 / 4; 0.23 / 0.31 / 4 |
| r50k clean | session-start / user-prompt / stop | `dogfood-event-deadline` at 1.51 s / 2.4-2.6 / 6-10 | 1.24 / 1.63 / 4; 1.28 / 1.72 / 4; 1.08 / 1.44 / 4 |
| r50k 2,500 dirty | session-start / user-prompt / stop | deadline at 1.51 s / 2.3-2.6 / 7-9 | 1.13 / 1.61 / 4; 1.10 / 1.61 / 4; 0.97 / 1.46 / 4 |
| r50k 10,000 dirty | session-start / user-prompt / stop | deadline at 1.51 s / 2.5-3.2 / 6-9 | 0.97 / 1.59 / 4; 1.03 / 1.58 / 4; 1.01 / 1.36 / 4 |
| r200k any | all three | deadline at 1.52 s, 1-2 spawns | unchanged: the first status scan alone exceeds the budget |

r5k and r50k are generated repositories of 5,000 and 50,000 tracked files with a private
`.git/corvint` store each; r200k has 200,000. Peak RSS is unchanged (about 45 MiB at 5k and
200 MiB at 50k for index-backed events, the whole snapshot decoded). The 50k hooks now answer
inside the adapter budget; what remains is two `git status --untracked-files=all` scans.

Tests (`TMPDIR=TD/tmp GOMAXPROCS=3 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`):
`./internal/gokernel/` PASS (`TestProbeAroundSpawnsFourGitProcessesAndRefusesDrift`, three
subtests); `./cmd/corvint/` PASS (`TestDogfoodEventSpawnsOneBracket`, five cases at four spawns;
`TestDogfoodEventRefusesRepositoryDriftInsideTheBracket`; the existing
`TestHarnessSharedObservationSpawnCount` unchanged); `./internal/contextindex/` PASS after re-pinning
the `IDX-SNAP-V0-017` input-audit digest for the new loader (consumer-only: `analyzerSchemaID`
stays `corvint-analyzer/105`, no fact or encoding changed, as commit 43f3e40d did).
`go vet` clean. The frozen evaluations are not touched by this change (no ranking input moved);
`dogfood-change` binding is left to the integrating session.

## Rollback

Revert the commit. The two-probe path keeps no persistent state, and no snapshot encoding,
envelope byte or refusal code changes.

## Addendum 2026-10-06: status-scan alternatives measured, not shipped (V1-0416 round 2)

The coordinator asked for the gain of two ways to cut the remaining scan cost, each with the
contract change it would need. `core.fsmonitor` stays forced off: a daemon-reported state is not
evidence Corvint can pin. Measured on r50k and r200k (`TD/logs/ucache-probe.txt`), load 8-10.

| Option | r50k scan | r200k scan | Gain | Contract change needed |
|---|---|---|---|---|
| Baseline `status --porcelain=v1 -z --untracked-files=all` under `gitRaw`'s forced `-c core.untrackedCache=false` (`internal/gokernel/repository.go:117-125`) | 0.21-0.27 s | 1.2-1.8 s | - | - |
| Read an existing `core.untrackedCache` read-only (`--no-optional-locks`, cache already written by `update-index --untracked-cache`; `--test-untracked-cache` rc 0 on this host) | 0.24-0.25 s | 1.4-2.1 s | none measurable | `GPK-V0-007` pins the exact argv including `-c core.untrackedCache=false`; honouring the cache means trusting index extension state the bracket did not observe |
| Identity-only closing observation (`rev-parse` pair, 0.04-0.05 s) when the event's reads cannot touch the worktree | saves one scan: 0.2-0.3 s | saves one scan: 1.2-2.1 s | about half of the per-event floor | weakens drift detection: a dirty-set change inside the window would no longer refuse; `GPK-V0-007`/`GPK-V0-076` would need a "reads that do not consult the worktree" class and the dogfood surface a per-event declaration |

Neither is shipped. The untracked cache gives nothing on this host because `git status` still
stats every tracked entry; only the identity-only closing would move r200k hooks, and it buys
the saving by not observing what the bracket exists to observe.
