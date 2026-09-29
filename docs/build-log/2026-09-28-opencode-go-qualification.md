# OpenCode Go qualification and startup context repair

The owner requested landing the OpenCode UI integration. The release coordinator assigned
V1-0459/V1-0460 closeout to this lane after GOC-V0-008 found an interpreter dependency in the
maintained qualification tests. AHI-032/033/034 retain the native qualification and UI contracts;
GOC-V0-008 requires their producer, campaigns and safety checks to run in Go. PR 333's original
0.7.1 UI checks passed; its automatic merge was paused for this release repair. Native ticket
completion remains with the coordinator after integration and the full release gate.

The four qualification scripts are replaced by `tools/qualify-opencode` and
`internal/opencodequalification`. The Go producer writes the exact record read by Node's
`corvint_status`. Architecture normalization, strict booleans/numbers, case completeness, actual
focused test execution, frozen package/collector/executable identities, exact latency quantiles,
recall/byte comparison, atomic invalidation and interruption remain required. Collector binding
includes the Go entrypoint, implementation, embedded host observers, build inputs and running
binary. The existing host observers and scripted loopback provider retain native behavior; the
Darwin/Linux PTY port retains current-frame assertions, both themes, pointer/keyboard navigation,
source selection, narrow views and proof inspection. Other platforms remain unsupported.

The direct installed-plugin witness exposed a real first-prompt race: `onPrompt` reserved a
session call, then its `runLimited` path counted that same request again. An overlapping
session-start request reached the two-call limit and suppressed prompt context. Removing the
outer duplicate reservation preserves the runner's cancellation and concurrency bound. A
reproduction against stock OpenCode failed before the fix and passed afterward. The focused
host suite now also holds startup open while awaiting the first prompt's receipt.

Independent review found two producer gaps. Invalid executable admission now follows previous
record backup and INCOMPLETE publication. Missing/shebang-host regressions require FAIL instead
of stale PASS. The interruption witness now checks observed PID/start identities passively before
its supervising runner rescues survivors. A deliberately broken detached-child cleanup must fail;
the maintained gate runner must pass the same witness. Rescue still runs to avoid leaking the
negative-control child. This is sampled observed-descendant evidence, not universal OS containment.

The earliest end-to-end proof is Go producer to actual Node consumer, including image/package
mutation refusals. Focused Go producer/cleanup tests, the actual adapter host/interrupt suite,
Go vet, Linux/unsupported-platform compile checks, contract/version checks and the native campaign
pass before freezing source. Native PTY campaigns passed both themes before the final frozen run.
The final six selected checks bind the committed source and CEM; their logs and qualification
record are retained as local evidence rather than rewriting historical results. Repository-wide
`make gate` is NOT_RUN in this lane because the release coordinator owns that gate.

Corvint pre-change query, path impact and affected-plan receipts are retained under
`/tmp/corvint-opencode-go/evidence`; non-Go omissions and affected-plan unknowns remain explicit.
The CEM/OCM workflow retains unassessed requirements separately from actual native behavior.
Ranking, learning, external model outcomes and authority promotion are outside this repair.
Existing historical qualification reports remain unchanged. Rollback selects the retained prior
package/configuration; removing the plugin entry disables it without migrating repositories,
traces or receipts. The shipped package version advances to 0.7.2 with the source change.
