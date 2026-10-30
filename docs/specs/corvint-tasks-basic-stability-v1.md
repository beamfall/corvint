# Corvint Tasks basic stability V1

Owner: Russell Lewis
Date: 2026-10-30
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3181)
Delivery status: experimental
Authoritative inputs: native ticket V1-3181; CAL-V0-007..017, CAL-V0-019 and CAL-V0-069 in `corvint-tasks-agent-leases-v0.md`; PRS-V1-002, PRS-V1-004 and the companion classification of `corvint-tasks` in `corvint-1.0-product-and-release-v1.md`; the V1-3180 inventory `docs/build-log/2026-10-29-core-promotion-inventory.md`.
Native ticket: V1-3181.

## Agent digest
- Claim: One source-built `corvint-tasks` binary runs the agent lifecycle on one non-fixture store with fixed commands, refusal codes and a requalification script.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3181); qualified on darwin/arm64 and linux/amd64. The catalog maturity stays experimental until the owner accepts this profile.
- Exists: `script/qualify-tasks-basic.sh` and `script/qualify-tasks-basic_test.sh`; the CAL-V0-019 suite in `internal/tasks/store`; evidence in `docs/build-log/2026-10-29-tasks-basic-stability.md`.
- Blocked on: owner acceptance of the profile (TBS-V1-460). Mixed binaries (V1-0757), cross-language CEM integration (V1-0596) and release attestation (V1-0232) are separate obligations outside this profile.
- Read next: Requirements; Non-goals.

## User and boundary

An external agent works one ticket from claim to native completion, using the leases contract
(`CAL-V0`), and needs to know which part of that surface it can rely on across builds. This contract
names that part and how it is qualified. It adds no behaviour: every command and code it names is
already owned by `CAL-V0`. A qualified profile is not a Core surface. `corvint-tasks` stays a
companion with its own qualification (`PRS-V1-002`, release classification table).

## Requirements

- `TBS-V1-451`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  The basic profile is exactly these `corvint-tasks` commands on one store:
  - `version`, `init`, `queue status` and `cutover --execution`;
  - `ticket create`, `ticket show` and `plan preview`;
  - `claim`, `renew`, `release`, `submit`, `gate run` and `complete`;
  - `receipt audit`.

  Any other command is outside the profile, even if it ships in the same binary. That includes
  `heartbeat`, `reap`, `dispatch`, `attempt`, `pool`, `service`, `import`, `upgrade` and every
  fixture-queue path.
- `TBS-V1-452`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  Output and refusal compatibility. Each profile command prints one `taskman-command-result/0`
  envelope with an `outcome` and a closed `codes` set. Within a profile qualification, a later
  build keeps each qualified situation's outcome class and code set. The qualified situations are:

  | Situation | Outcome | Codes |
  |---|---|---|
  | A claim before the execution cutover | `REFUSED` | `CUTOVER_MISSING` |
  | A claim whose scope collides with a live attempt | `REFUSED` | `RESOURCE_COLLISION` |
  | A stale or foreign generation on `renew` or `complete` | `REFUSED` | `FENCED` |
  | A submit outside the ticket's touch paths | `REFUSED` | `OUT_OF_SCOPE` |
  | A gate on a dirty worktree | `ERROR` | `DIRTY_WORKTREE` |
  | A gate on a worktree whose tree is not the submitted candidate | `ERROR` | `STALE_TREE` |
  | A completion whose commit is not integrated | `REFUSED` | `COMMIT_NOT_INTEGRATED` and `STALE_TREE` |
  | A completion whose passing gate ran on an older candidate | `REFUSED` | `GATE_STALE` |
  | A failing gate | `OK` | none; `gateResult.state` is `FAILED` and the output is retained |
  | An exact request repeated with the same `--request-id` | as the first | as the first; `replayed` is true |

  A new code may be added only for a new situation. A change to a qualified situation's outcome
  or codes ends the profile's qualification (TBS-V1-460).
- `TBS-V1-453`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  Persisted-state compatibility follows the preceding-reader rule. A build in the profile reads
  every store that the preceding qualified build wrote, without migration. A change to a persisted
  format version that `version` lists is outside the profile until a separate migration contract
  is accepted. That contract must name the migration and its rollback. Rollback inside the profile
  restores the preceding binary (`corvint-update rollback --component tasks`, or the retained
  binary). A rollback leaves the store unchanged, because no profile command rewrites a format.
- `TBS-V1-454`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  The supported tuples are darwin/arm64 and linux/amd64, `go1.27.3`. Each tuple runs one binary,
  built from one source commit, against one store. Windows and every other tuple are UNSUPPORTED
  (`PRS-V1-004`). A tuple counts as qualified only through its own run under TBS-V1-456.
- `TBS-V1-455`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  The profile excludes:
  - dispatch, supervision, services, pools and continuous runs;
  - mixed binaries or non-lease writers on one store (V1-0757);
  - any scale, throughput or latency claim;
  - Beamfall roadmap takeover (`PRS-V1-014`);
  - release promotion and attestation (V1-0232);
  - cross-language CEM integration (V1-0596).

  Qualifying it never initializes, migrates or cuts over a live store.
- `TBS-V1-456`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  `script/qualify-tasks-basic.sh OUT_DIR` does the following:
  - builds `corvint-tasks` from the checkout and runs the CAL-V0-019 suite;
  - creates a disposable non-fixture queue in `OUT_DIR/repo`, from the maintained external-agent
    templates, whose required gate is a real command;
  - runs the lifecycle on that queue: ticket create, plan preview, claim, renew, submit, gate run,
    Git integration, native complete, readback and receipt audit, plus a release and re-claim;
  - retains `identity.json` (source commit, tree and dirty flag, Go version, binary sha256 and
    version, suite sha256 and tests, host), the suite run, and one `results.jsonl` line per case
    with its argv, exit, outcome, codes, expectation and verdict.

  It refuses an `OUT_DIR` that exists and is not an empty directory (exit 2).
- `TBS-V1-457`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  The run includes each situation of TBS-V1-452:
  - a stale or fenced generation (`renew` and `complete`);
  - a scope collision;
  - a wrong candidate: out of scope, and a worktree tree that differs from the candidate;
  - a dirty candidate;
  - a failing gate and a stale gate;
  - an unintegrated completion;
  - exact-request replay (`claim` and `complete`).

  Lease expiry, reaping, racing writers and crash recovery are qualified by the CAL-V0-019 suite
  the same run executes.
- `TBS-V1-458`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  Reachability comes before any refusal case counts:
  - The store must read back `fixture: false`.
  - A claim before the cutover must refuse `CUTOVER_MISSING`, which proves the execution gate is
    live.

  The test-only `QUALIFY_TASKS_BASIC_TAMPER=CASE` replaces one case's expected codes with an
  impossible value. `script/qualify-tasks-basic_test.sh` proves three things with it:
  - exactly that case fails;
  - the run exits 1;
  - the reachability cases still pass.
- `TBS-V1-459`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  No failed or skipped case counts as passed. Any mismatch prints `FAIL case=...` and ends the run
  `FAILED` with exit 1. A suite run that lacks a passing suite test, or that has a failure, stops the
  run before any lifecycle case. The last line is `QUALIFIED` only when all three hold:
  - the binary was built by the run;
  - the suite was run by it;
  - the source tree was clean.

  A supplied binary (`--tasks-binary`), a supplied suite run (`--suite-run`) or a dirty source
  prints `LIFECYCLE_PASSED` with those inputs named.
- `TBS-V1-460`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3181)
  Qualification is per source commit and tuple. Any change under `cmd/corvint-tasks/` or
  `internal/tasks/` needs a new run before the profile is claimed for it. The catalog maturity of
  `corvint-tasks` changes only for this exact profile, after the owner accepts it with its retained
  evidence. That acceptance gives no Core promise and no release approval.

## Non-goals

Everything in TBS-V1-455. No new command, code or format. No full-binary, fleet or release claim.
A passing run proves the named situations on the named tuple and nothing broader.

## Failure modes

- A qualified code changes or disappears in a later build: the matching case fails, and the run
  is `FAILED`.
- The templates drift so that the non-fixture queue cannot be built: the script refuses before
  `init` and names the template.
- The CAL-V0-019 suite list changes: the script reads `QualificationSuite` from the source, so a
  renamed test is still required. If the list cannot be read, the run refuses.
- The host lacks a supported toolchain: the build or suite fails, and the run ends `FAILED`.

## Rollback

Delete the two scripts and this spec. No product code or persisted state depends on them.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| TBS-V1-451 to TBS-V1-455 | This contract | `docs/build-log/2026-10-29-tasks-basic-stability.md` |
| TBS-V1-456, TBS-V1-457 | `script/qualify-tasks-basic.sh` | Retained `QUALIFIED cases=30` runs on darwin/arm64 and linux/amd64 (build-log entry) |
| TBS-V1-458, TBS-V1-459 | `script/qualify-tasks-basic.sh`, `QUALIFY_TASKS_BASIC_TAMPER` | `script/qualify-tasks-basic_test.sh` |
| TBS-V1-460 | Catalog row unchanged (experimental) | `docs/CAPABILITIES.md` Tasks section |
