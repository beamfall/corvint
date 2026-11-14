<!-- SPDX-License-Identifier: Apache-2.0 -->
# Nightly release-gates triage (#192): lazy plan budget under host load

Date: 2026-11-13. Branch `claude/lazy-plan-budget-contention`, based on origin/main `814647cf0`.
Ticket V1-5003. Scheduled `release-gates` run 43444722071 at `6c17e3888` failed three ways:

- `cross-vet` (both gate jobs): `cmd/corvint/inventory_readers_unix_test.go` used
  `sessionDeclaration` from a `darwin || linux` file under a `unix` constraint, so the BSD and
  Solaris vets did not compile. Already fixed on main by `6990e22bb`.
- Hostile regression matrix (`linux-core`): rows still named the deleted `./internal/trace`
  package (`2695ec5fa`). Already fixed on main by `9e6e322ca`.
- `TestLazyPlanBudget` (both gate jobs): open+plan process CPU p95 110.6 ms (ubuntu-24.04, p50
  93.3 ms) and 121.9 ms (macos-15, p50 94.6 ms) against the 100 ms GLS-V0-003 budget during the
  parallel `go test ./...` of `make -k gate`. This change addresses that one.

## Decision

Measuring process CPU (`4e723128b`) removed scheduler stalls but not contention itself: cache and
memory-bandwidth sharing and GC workers on busy cores still inflate the CPU a plan costs. On a quiet
4-core linux/amd64 lane host the same test measures p50 69 ms, p95 83 ms. GLS-V0-003 is a
reference-host bound, so the test now reads the 1-minute load average before and after the timed
runs and, when the p95 is over budget and either reading reached the core count, reports
`NOT_JUDGED` (a skip that names the measurements) instead of failing. That is PRB-V0-028's
contention threshold, which `internal/perfbudget` applies to every perf run; the reader is
test-local because `internal/perfbudget` imports `internal/linecov`. An unobserved load average is
not contention, so hosts that cannot report one are still judged, as is every quiet host.
`TestPlanBudgetContended` pins the decision and that Linux and Darwin observe the load.

Limit: the nightly gate runs the package under load, so it will usually report `NOT_JUDGED` there
rather than enforce the budget; enforcement comes from a quiet run of the test or the
reference-host measurement. `TestPlanTimeBudgetWithSegments` has not failed (p95 24 ms) and is
unchanged.
