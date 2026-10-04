# Tasks user service runtime slice: issue 500

Human-owned intent: GitHub issue 500, native V1-0697. On 2026-10-04 the owner accepted the #500
intent as drafted. This replaces the earlier owner-delegated (agent decided) acceptance recorded in
`2026-10-04-tasks-user-service-foundation.md`. The spec header, `docs/specs/README.md` and
`docs/specs/INDEX.json` now say owner-accepted. The limits are unchanged: the profile is optional
and operator-installed, the default product keeps no permanent daemon, and real platform
qualification stays NOT_RUN until disposable evidence exists.

**Owner decision, 2026-10-04: platform qualification is deferred.** The slice ships with real
launchd and systemd qualification marked NOT_RUN. Native ticket V1-0697 stays open until a
disposable Darwin/Linux target exists. The deferral accepts no SERVICE500-004, -005 or -010
platform witness.

The governing contract is `docs/specs/corvint-tasks-user-service-v0.md`, SERVICE500-001..011.

## Delivered

This slice delivers gap items 1-3, 6 and 8 of the issue analysis, plus the managed main.

- **CLI.** `corvint-tasks service install|status|uninstall|stop|resume|run` is in
  `internal/tasks/cli/service.go`. It uses closed flags, and help for each verb is in
  `commandUsage`.
- **Journal and registry.** `internal/tasks/service/store.go` keeps a private 0700 registry under
  `$HOME/.local/state/corvint-tasks/service/<program>`. It holds the manifest, control, pulse, and
  the `operations/` original-request journal, capped at 256 records. The registry is under flock
  and every write is atomic and fsynced. The same request id with different bytes refuses
  REQUEST_ID_CONFLICT. An unfinished operation blocks others until the same id resumes it.
- **Manager interface.** The interface is in `manager.go`. `ExecManager` runs only fixed-path
  launchctl or systemctl, with a minimal environment, a 30s timeout and bounded output. Every
  argv is recomputed from a decoded manifest, never read from the journal.
  - Install, replace and uninstall observe ownership before any effect: unit bytes, the
    registration and systemd drop-ins.
  - A foreign or modified unit refuses.
  - An unreachable manager refuses CAPABILITY_UNAVAILABLE before any effect.
  - Rollback restores only under the same lineage.
  - Unit removal is digest-checked: the old or the new owned bytes, nothing else.
  - Uninstall keeps control, the journal and the workers.
- **Managed main.** `service run` runs the existing dispatcher in-process only while control is
  RUNNING, bound to the installed manifest, and the executable and dispatch-config pins match.
  Otherwise it idles or holds rather than exiting.
- **Liveness.** `service status` is a pure read. `dispatch status` gains an additive `service`
  object only when this program's installed manifest binds the dispatcher state root. Otherwise
  its output is unchanged, and it never calls the manager.
- **Fence pruning.** A 600s healthy reset or a reconciled resume prunes settled decimal-generation
  fences and raises a floor watermark. A later outcome at or below the floor HOLDs and is never
  recharged. Before this, the 128-fence store never shrank, so a long-lived unit could reach a
  permanent capacity HOLD (the open review nit).

## Decisions

- **Stop is an observed-boundary suppression.** The managed main polls control about every 2s and
  cancels the dispatcher; a cancelled roster launches nothing further (a per-launch context check
  in `dispatch.launchRoster`). Without an OpenControlled pre-spawn fence, workers can still start
  for at most one poll plus one roster (each roster ≤ globalCap) after STOPPED is saved. This is
  stated in Traceability and not claimed as SERVICE500-003 closure.
- **ACKNOWLEDGED is ordered after the save.** Stop reads the dispatcher owner and the pulse after
  STOPPED is durable, and the managed main re-reads control after `Open` takes ownership and
  before `Run`. Either the main sees STOPPED and closes without running, or stop sees the owner
  and answers PENDING. A live RUNNING pulse also keeps stop PENDING, and the main publishes its
  pulse at once on a state change.
- **Control requests replay through a bounded ledger.** `requests.json` keeps the most recent 256
  stop/resume (and unjournaled NO_CHANGE install) request ids with their hashes. A delayed retry
  of an older request replays the current control instead of reapplying over a later change. A
  superseded stop answers SUPERSEDED. Control is written before the ledger, and the next request
  first records control's last request, so an interrupted write cannot lose it.
- **Journal headroom.** A NO_CHANGE install takes no journal slot. Installs refuse at 248 records,
  so uninstall always has the last 8. Finished records are never pruned; pruning is remainder.
- **Rollback restores the manifest.** If a commit renamed before its readback failed, rollback
  restores the previous manifest exactly or removes a fresh install's manifest, and refuses a
  manifest that is neither.
- **`--drain`, helpers and `legacyStopFile` refuse UNSUPPORTED.** That is better than a partial
  semantic.
- **Runtime restart debt is NOT_OBSERVED and never charged.** The controller never exits on a
  hold, so the manager keepalive cannot loop.
- **The install request hash binds the service profile bytes only.** Changing the dispatch config
  under the same request id replays the original operation. A new id observes the new pins.
- **Every ROLLED_BACK install answers RESTORED.** That covers the first attempt, the call that
  completes a held rollback, and a replay. The failure that caused the rollback stays in the
  diagnostic.
- **systemd drop-ins are observed system-wide.** Per-unit drop-ins under the user root and the
  `/etc`, `/run`, `/usr/local/lib` and `/usr/lib` `systemd/user` roots block adoption. Type-wide
  drop-ins and generator output are not observed.
- **An invalid profile is refused before the registry is created.**
- **Reinstall after uninstall keeps the retained STOPPED control until `service resume`.**

## Evidence

- **Tests.** The `TestSERVICE500_*` witnesses in `internal/tasks/service/lifecycle_test.go` and
  `internal/tasks/cli/service_test.go` use an in-memory fake launchd and canned systemctl output in
  a temp HOME. No test runs launchctl or systemctl.
- **Platform qualification is NOT_RUN.** That covers SERVICE500-004/005/010: real bootstrap,
  restart, worker adoption and boot/login scope. No disposable Darwin user or Linux
  systemd --user target was admitted, and no service was installed on the owner's host.

Independent review of `cd70ba0a..03107d50` returned CHANGES_REQUIRED with no HIGH findings. Its
four MEDIUM findings (control replay, stop disclosure and ACK ordering, rollback manifest
restore, journal headroom) and the LOW and NIT findings are fixed as recorded above. The
request-id binding to profile bytes was confirmed as intended.

## Remainder

- Gap 4: OpenControlled pre-spawn fence, drain and the legacy stop-file adapter.
- Gap 5: helpers, `run-helper` and runtime logging.
- Gap 7: disposable Darwin/Linux lifecycle qualification (owner-deferred 2026-10-04; V1-0697
  stays open until a disposable target exists).
- Pruning of finished journal records, and observation of type-wide systemd drop-ins.
- Durable runtime restart-debt charging.
- Native closeout.
