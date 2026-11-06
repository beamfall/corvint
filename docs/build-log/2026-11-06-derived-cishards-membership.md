# Derived cishards membership (V1-4802)

V1-4802 asked that adding or deleting a Go package stop forcing a hand edit of `.github/cishards`
data, so deletion PRs no longer touch `.github` and run FULL CI. Investigation at base
`670559b9ec3ffbcd59aa6c6d02029ec0cb206a93` found the runtime already derives membership: the
helper reads the `go list ./...` universe on standard input, `PlanFromParallel` prices an
unpriced package at the whole-table median, and `SplitPackages` and `SelectFromParallel` skip
entries outside the universe. No checked-in shard list exists; `walltime_test.go` is a frozen
replay fixture independent of the live universe. The ticket premise was therefore a missing
contract and missing tests, not missing code.

Decision: amend AFP-V0-022 (V1-4802 paragraph) rather than add a new ID, because the
`affected-plan-v0.md` Requirements section is at the 256-obligation OCM bound and AFP-V0-022
already owns the complete-universe partition. The median stays computed over the whole table,
stale entries included, so the default weight is a function of the table alone and Select still
equals Plan. Drift stays advisory (AFP-V0-040); the new repository test logs it and never fails.

## Evidence

- `TestAFPV0022MembershipDerivedFromUniverse` and `TestAFPV0022RepositoryUniverseNeedsNoTableEdit`
  in `.github/cishards/slices_test.go` pass. The live universe has seven unpriced packages and no
  stale cost entry; every plan at sixteen shards stays complete.
- Falsifier: changing `PlanFromParallel` to drop an unpriced package fails both tests
  (`.../x/added: whole=0` and `internal/analyzerenvelope: whole=0`); the mutation was reverted.
- Scratch worktree at the base, `corvint affected --base 670559b9...`:
  - deleting `tools/plan-delta` (priced in `package-costs.json`): dirty paths are the two deleted
    files, no `.github` path; `scope=UNKNOWN unknown=1 LANGUAGE_FRONTIER=1`
    (`shell:foreign-invocation-unmodelled`);
  - plain edit of `tools/plan-delta/main.go` (control): the same single unknown;
  - removing the `tools/plan-delta` cost entry instead: `unknown=58`, including
    `UNOWNED_DIRTY_PATH .github/cishards/package-costs.json`.

  A deletion is planned exactly like an ordinary edit; the remaining UNKNOWN comes from the
  repository-wide shell frontier, tracked by V1-4392 and V1-4687, not from shard data.

Limits: hosted CI runs the cishards tests in an isolated module copy where the repository test
skips; the synthetic test is the CI evidence there. Rollback reverts the two tests and the
AFP-V0-022 paragraph; no runtime behavior changed.
