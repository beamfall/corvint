# Line evidence: additive integration and bounded runner arguments

Ticket: V1-5697. Accepted change: G14, PR #235, 2026-11-18.
Base: `9f4d5d89ab0648425db40e617f3d890598f9ff79`.
Contract: [Line Evidence V1](../specs/line-evidence-v1.md), LEV-V1-001 to LEV-V1-015.
State: experimental; Ruby collector proof remains blocked. No runner is admitted to tier `line`.

## Decisions

The accepted writing verb is `corvint line-evidence refresh`. It requires a clean committed tree,
rechecks HEAD and file OIDs after collecting, then atomically replaces only the successful instance.
The private `linecov-index/4` file binds config and lockfile blobs, command and runtime identity
inside the existing runner-identity field. Canonical IDs replace collector-local names. Measured
durations are zeroed in this derived selection index so identical runs produce identical bytes.
Standalone collector tools remain available; their removal was not authorized by this lane.

The shared reader only adds tests and advisory ranks to the file plan. It never authorizes removal.
Fresh evidence remains `file` with `line-evidence-unqualified`; no FIH records exist. The opt-in
`e2e-line-experimental` profile is labelled experimental and keeps that qualification barrier.
Stale identity/config/ancestry selects the whole instance, including whole Go package execution.
Watch execution retains its planning index override. The explicit legacy Go `--line-index`
qualification contract is unchanged, as required by design decision 8.

Pytest 8.2 introduced argument files; older or unreadable versions retain the full fallback. The
version probe and private argument-file write occur only at execution, never while planning.
Python still runs whole files. Source:
[pytest usage](https://docs.pytest.org/en/stable/how-to/usage.html#read-arguments-from-file).
RSpec's options-file parser discards file/directory arguments, so it cannot safely carry a selected
file list. Source:
[RSpec configuration options](https://raw.githubusercontent.com/rspec/rspec-core/v3.13.0/lib/rspec/core/configuration_options.rb).
Unsupported long name lists widen to files with `argv-limit`; if even the file argv exceeds the
bound, the invocation uses its full suite command and retains `argv-limit`.

## Observed proof and limits

Raw logs and installed test-only JavaScript dependencies are outside the repository under
`/home/russell/lanes/lev-evidence/`. The native reader package run passed in 9.534 seconds, including
live Vitest 3.2.4 (5.33 seconds) and Jest 30.2.0 (4.13 seconds) collector fixtures. Each collected a
committed source, then used a one-line working-tree edit to find covering canonical tests at tier
`file`, reason `line-evidence-unqualified`. RSpec and Minitest native fixtures were explicitly
skipped: Ruby is absent. The named `corvint-rspec-qual:2026-10-20` container image is also absent.
Synthetic reader fixtures passed for all five collectable runners; they do not qualify collectors.

`TestLEVV1001RefreshAndConsumers` refreshed real Go tests twice and compared the complete index bytes.
It checked staged and untracked changes through affected, test and MCP, Vitest watch acceptance,
and unchanged store bytes after consumer reads. The focused CLI run passed in 2.993 seconds and
also included stale-instance execution, pytest file tier and additive Playwright selection/rank.

| Log | SHA-256 |
| --- | --- |
| `lineevidence-native-whole.log` | `3139158ab272d8f4043ed26c12907415d9366b74e25d0f0c089493dc3a4284b2` |
| `cmd-focused.log` | `e65cfc9a206bc0bb0dc414278db9bc5bd165f1e340297b1f01df1e1b98f5fe41` |

The first npm install failed because its default cache under `/home/russell/.npm` is read-only.
Retrying with `/home/russell/.cache/corvint-lev-npm` succeeded. The initial native fixture attempt
therefore lacked executable dependencies; the retained successful native run supersedes it.
A focused CLI compilation initially used the wrong kill-switch import path; correcting it restored
the focused run. No existing assertions or committed goldens were weakened or replaced.

Task 10 (Action) and task 11 (FIH admission) are outside this lane. Ruby live proof is NOT_RUN.
Fault-injected misses and net wall-clock benefit are NOT_OBSERVED; neither line-level nor browser
per-test narrowing is promoted. Final package/gate results belong to the lane handback. Independent
review is supplied by the orchestrator. The retired CEM/dogfood workflow is not used.
