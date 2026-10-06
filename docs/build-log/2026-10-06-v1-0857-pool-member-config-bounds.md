# 2026-10-06: V1-0857 pool health/cleanup bound and pinned external cwd

## Intent

Issue beamfall/corvint#628 (ticket V1-0857). Pool `memberConfig` health and cleanup commands were
limited to `timeoutSeconds` 1..300, and every command ran in the queue checkout (`cwd:"REPOSITORY"`).
An operator whose database reset takes several minutes, and whose reset tooling lives in a separate
checkout, had to keep an external sweeper. Requested: a larger bound for health and cleanup, and a
`cwd` that names a pinned external checkout that is verified before anything runs.

## Change

- `PSR-V0-011`: health/cleanup `timeoutSeconds` accepts 1..3600 (`intent.MaxPoolCommandSeconds`).
  CAL-V0-033 and PSR-V0-008 are amended. The other limits are unchanged, including safeReuse 1..900,
  the sweep total 1..1800 and the dispatcher poolSweep 1..1800.
- `PSR-V0-012`: `cwd` may be `{"kind":"PINNED_REPOSITORY","path":ABS,"revision":OID}` for health,
  cleanup, and (as optional `safeReuse.cwd`) reset+verify. `store.poolCommandDir` verifies, with
  bounded `sweepGit` probes, that the path is a real (non-alias) directory, a worktree top level
  (else MISSING_EVIDENCE), that `HEAD^{commit}` is the revision (else STALE_TREE), and that porcelain
  status with untracked=all, excluding `.taskman`, is empty (else DIRTY_WORKTREE).
- The check runs at three points:
  - In the writer observer at claim preparation, cleanup and sweep admission. A refusal records
    nothing and runs nothing.
  - Again just before launch. A failure runs nothing, records a failed observation and the member
    stays quarantined.
  - After exit. Drift records SOURCE_CHANGED.
- Docs: `docs/TASKS-EXTERNAL-AGENTS.md`, CAL policy shape.

## Decisions

- **Shape.** The pin follows the `configRef` style: a closed object with an absolute path and a
  full object id. Validation is host-independent (clean POSIX path, not `/`; lowercase 40/64 hex).
- **Clean-input rule.** The pin uses REPOSITORY's PSR-V0-006 rule, untracked=all, rather than a
  tracked-only rule. The stricter rule mirrors the existing contract. Gitignored files stay allowed.
- **Queue repository still checked.** The queue-repository clean-input rule still applies. A relative
  env file stays relative to the queue repository.
- **Binding.** The pin is bound through the member definition digest, so occupied definitions stay
  immutable.
- **Timeout clock.** The timeout unit is the package variable `poolCommandSecond`. Tests shorten it
  through `SetPoolCommandSecondForTest`, so the 3600 bound is enforced without waiting.
- **Long-command interplay (recorded in PSR-V0-011).**
  - The writer lock is not held during execution.
  - Claim-time health runs before the claim commits, so no lease, heartbeat or retry debt exists for
    it. Cleanup acts only on a quarantined allocation.
  - No TTL reaps PREPARING or CLEANING.
  - Inside a sweep, cleanup is still cut at the member's 900 s deadline or the 1800 s total.
  - Dispatcher role `idleSeconds`/`wallSeconds` must exceed the health bound. This is the
    operator's responsibility; a worker killed mid-health leaves PREPARING for explicit recovery.

## Evidence

- `GOMAXPROCS=2 go test -p 1 -count=1 -run 'PSRV001[12]'` on `internal/tasks/intent` and
  `internal/tasks/store`: PASS.
  - `TestPSRV0011PoolCommandBound`
  - `TestPSRV0012PinnedCwdShape`
  - `TestPSRV0011ExtendedBoundEnforced`: `sleep 30` under bound 3600 is killed in about 1.3 s with
    the injected clock and records `TIMEOUT;` quarantine.
  - `TestPSRV0012PinnedExternalCwd`: covers pinned, mismatched revision, dirty tracked, dirty
    untracked, alias, subdirectory and non-worktree.
  - `TestPSRV0012PinnedSweep`: the pinned sweep frees the member after cleanup and reset run in the
    external worktree. A mismatched pin refuses admission without running a phase.
- Pool/policy/sweep regression subset on both packages; `go vet` for darwin, linux and windows on
  both packages.
- `make spec-requirements-check requirement-definitions-check line-citations-check
  traceability-tests-check`: PASS.

## NOT_RUN

`make gate`, `go test ./...`, full `cmd/corvint` suites, the Linux runtime, the dogfood CEM loop and
hosted CI.

## Residual risk

- A reset longer than 900 s still cannot complete inside `pool sweep`. Raising the sweep bounds needs
  its own requirement.
- Detached processes and external services started from a pinned checkout remain outside
  process-group qualification.

## Rollback

1. Free or clean every member whose occupied definition uses a pinned cwd or a bound above 300.
2. Publish policy with `cwd:"REPOSITORY"` and a bound of 1..300.
3. Revert the source.

Older decoders reject the new shapes.
