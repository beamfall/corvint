# Basic Tasks external-agent profile: contract and qualification (V1-3181)

V1-3181 names the part of `corvint-tasks` that an external agent can rely on across builds, and
qualifies it from current source. The contract is
[TBS-V1](../specs/corvint-tasks-basic-stability-v1.md) (`TBS-V1-451` to `TBS-V1-460`, a new prefix,
because the leases spec is at its obligation limit). The contract adds no behaviour: every command and
code it names is owned by `CAL-V0-007..017`, `CAL-V0-019` and `CAL-V0-069`. The ticket depends on
the V1-3180 inventory ([2026-10-29-core-promotion-inventory.md](2026-10-29-core-promotion-inventory.md)).
The file date follows the ticket's declared path; the runs were taken on 2026-10-30.

## Decision

- The profile is the single-binary lifecycle on one store (TBS-V1-451). Its outcomes and code sets
  are frozen per qualified situation (TBS-V1-452). Its persisted state follows the preceding-reader
  rule, with no in-place migration (TBS-V1-453).
- Dispatch, services, pools, mixed binaries, scale, Beamfall takeover, release promotion and
  cross-language CEM integration are excluded (TBS-V1-455).
- Qualification is a script, `script/qualify-tasks-basic.sh OUT_DIR`, not a new command. It reuses
  the maintained `internal/tasks/cli/testdata/external-agents` templates to build a disposable
  **non-fixture** queue whose required gate is a real command (`test -f a/PASS`). It runs the
  CAL-V0-019 suite and records the execution cutover from that run.
- The catalog maturity of `corvint-tasks` stays **experimental**. TBS-V1-460 allows a change only
  after the owner accepts this exact profile. Nothing here touches the live store: `queue:corvint:main`
  still reads `fixture: true` with no execution cutover.

## Qualification runs

Both runs used commit `82427862b7ed54b1a9f25dc72254a0f4991da3e7` (tree
`7a3158528062cd90e5072852db82417a67167cc5`, clean) and `go1.27.3`. Each built its own binary and ran
its own suite, and each ended `QUALIFIED cases=30`.

| Tuple | Host | Binary sha256 | Suite run sha256 (elapsed) | `results.jsonl` sha256 |
|---|---|---|---|---|
| darwin/arm64 | the Mac, via `script/heavy.sh` (local slot) | `d578fc8b49553d7acd91620d8eb61a54c23df8c871162a9361b83534bb59772e` | `acc22d8a286a106d005302310f327a44d7763c1fb49185735c8eaf8c129ce09b` (43.4 s) | `104346be53b1619cc32e90fd057b38fa8b2ea7e945ff8e1cc10fc81f27fa0532` |
| linux/amd64 | WSL lane host, a clean clone of the same commit | `5a1891fa4879e98d28b0dc5c75c875c2e17c8c929a120e6d4939c89435f0d209` | `14867cff0d75035e5a3432772daec9571674563a0579f5a29795571633fe72b0` (16.0 s) | `68641149c050d3d2b3e915f5e79bf415bad57a44e58891a2c830e3ce7cd3a617` |

Both suite runs passed all five CAL-V0-019 tests and had no failure event:
`ConcurrentCollidingClaimsAdmitOne`, `RacingLeaseVerbsLeaveOneConsistentHead`,
`FencedGenerationCannotMoveOrComplete`, `FaultAtEveryArtifactIsAllOrNothing` and
`KilledWriterRecovers`.

The binary's `version` reports `0.0.0-tcp01-unverified+build.0`, verification `NOT_RUN`. That is the
source-build label. The binary's identity is its sha256.

The 30 cases, in order, with the observed outcome and codes, were the same on both tuples:

| # | Case | Outcome | Codes |
|---:|---|---|---|
| 1 | `init-nonfixture` (reads back `fixture: false`) | OK | |
| 2-3 | `create-1`, `create-2` | OK | |
| 4 | `claim-before-cutover` | REFUSED | `CUTOVER_MISSING` |
| 5 | `cutover-execution` (from the suite run) | OK | |
| 6 | `plan-preview` | OK | |
| 7 | `claim` | OK | |
| 8 | `claim-replay` (`replayed: true`) | OK | |
| 9 | `claim-scope-collision` | REFUSED | `RESOURCE_COLLISION` |
| 10 | `renew` | OK | |
| 11 | `renew-fenced-generation` | REFUSED | `FENCED` |
| 12 | `submit-out-of-scope` | REFUSED | `OUT_OF_SCOPE` |
| 13 | `submit-failing-candidate` | OK | |
| 14 | `gate-dirty-worktree` | ERROR | `DIRTY_WORKTREE` |
| 15 | `gate-fails` (`gateResult.state` FAILED) | OK | |
| 16 | `gate-wrong-candidate` | ERROR | `STALE_TREE` |
| 17-18 | `submit-passing-candidate`, `gate-passes` | OK | |
| 19 | `complete-unintegrated` | REFUSED | `COMMIT_NOT_INTEGRATED`, `STALE_TREE` |
| 20 | `submit-final-candidate`, then `git merge --ff-only` | OK | |
| 21 | `complete-stale-gate` | REFUSED | `GATE_STALE` |
| 22 | `gate-final` | OK | |
| 23 | `complete-fenced-generation` | REFUSED | `FENCED` |
| 24 | `complete` (outcome COMPLETED) | OK | |
| 25 | `complete-replay` (`replayed: true`) | OK | |
| 26 | `readback-completed` (status COMPLETED) | OK | |
| 27-29 | `claim-after-completion`, `release`, `reclaim-after-release` | OK | |
| 30 | `receipt-audit` | OK | |

`script/qualify-tasks-basic_test.sh` proves the script's own refusals and its fault injection
(TBS-V1-458, TBS-V1-459). It passed on darwin/arm64 in about 8 s. It checks five things:

- A missing or non-empty `OUT_DIR` exits 2.
- A suite run that lacks `KilledWriterRecovers` fails before any lifecycle case.
- A run over a supplied binary and suite ends `LIFECYCLE_PASSED ... binary=SUPPLIED suite=SUPPLIED`,
  never `QUALIFIED`.
- `QUALIFY_TASKS_BASIC_TAMPER=claim-replay` fails exactly that case with exit 1.
- In the tampered run, the two reachability cases (`init-nonfixture`, `claim-before-cutover`) still
  pass.

## Review fixes

An independent review of `93b9f5b08..31d354914` found four gaps, all fixed in the same lane:

- A case's verdict left out the exit status, so a refusal that exited 0 passed. Each case now also
  requires exit 0 on `OK` and 1 otherwise (TBS-V1-452).
- The envelope profile was not checked. Each case now requires `taskman-command-result/0`.
- `QUALIFIED` checked only input provenance and a clean source. It now also requires a supported
  tuple and `go1.27.3`; otherwise the last line is `LIFECYCLE_PASSED ... tuple=OS/ARCH/GO
  supportedTuple=false` (TBS-V1-454, TBS-V1-459).
- The shell test runs `go build`, so it is an unbounded unit. It is now registered per
  `docs/SHELL-TEST-REGISTRATION.md`: the `qualify-tasks-basic-test` target, `GATE_STEPS`,
  `shell-test` and `units` in `.corvint/unbounded-readers.json`. Its reads cannot be declared in
  `.corvint/test-read-scopes.json`, because a `go build` reads the whole program.

The shell test now also runs the lifecycle over two wrapped binaries: one that always exits 0 and one
that rewrites the profile to `/1`. Each must fail `claim-before-cutover` and end with exit 1.

## Tuples

The supported tuples are darwin/arm64 and linux/amd64. Both are qualified above, so neither is
unavailable. Windows is UNSUPPORTED under PRS-V1-004 and was not run.

## Limits

- The linux/amd64 run used a clone transported as a Git bundle. The repository has no remote
  qualification helper for this.
- Qualification is per commit and tuple (TBS-V1-460). A later change under `cmd/corvint-tasks/` or
  `internal/tasks/` needs a new run.
- No scale, latency or multi-binary claim is made.
- Owner acceptance of the profile is pending. The maturity label is unchanged.

## Requirement IDs used

`TBS-V1-451` to `TBS-V1-460`.
