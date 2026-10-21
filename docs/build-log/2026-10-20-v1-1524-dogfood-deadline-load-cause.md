# V1-1524: dogfood-event-deadline under ~1,270 dirty `.taskman/` paths is a load cause, not a dirty-set cause

Date: 2026-10-21. Ticket: V1-1524 (P1). Requirement: `AHI-055` (amended). Base: origin/main 90ab9f33e
(Corvint 1.0.0-rc.3 build 697). Host: macOS, 12 logical CPUs (8 performance + 4 efficiency).

## Reproduction

Two plain local clones of the base: `repro1524` with 1,266 dirty paths, all under `.taskman/` (507
modified tracked ticket projections, 759 untracked projection copies), and `clean1524` with none.
Both ran `corvint index` first, so every event is a snapshot hit (177 MB snapshot). Events were driven
through the real adapter (`corvint adapter claude-code user-prompt|stop`, `CLAUDE_PROJECT_DIR` set to
the clone). Per-phase timings came from a scratch program (not committed) that runs the event's own
read: `gokernel.ProbeRepositoryAround` (opening probe), `localcompletion.EvaluateObserved`,
`contextindex.LoadSnapshotObserved`, `contextindex.DogfoodPromptContext`, closing probe. CPU load was
generated with foreground `xargs -P N` busy loops for 35-40 s.

| Condition | Phase (ms): open probe / eval / snapshot load / compile / close probe / total | Adapter wall (ms) |
| --- | --- | --- |
| idle (load 8-10), 1,266 dirty | 108-123 / 0 / 187-197 / 1 / 106-125 / 417-434 | user-prompt 454-614, stop 241-424 |
| idle, clean | 167-193 / 0 / 190-199 / 1 / 178-195 / 546-579 | user-prompt 512-534, stop 265-337 |
| 45 busy loops (load avg rising to 23), 1,266 dirty | 209-308 / 0 / 291-578 / 1-5 / 151-193 / 711-1066 | user-prompt 944-991, stop 443-637 |
| 60 busy loops (load avg 63), 1,266 dirty | 184-451 / 0 / 451-741 / 1-4 / 168-538 / 1073-1554 | user-prompt 1304-1432 (all ok), stop 623-747 |
| 60 busy loops (load avg 63), clean | 637-769 / 0 / 282-604 / 1-3 / 546-796 / 1641-2055 | user-prompt 1548-1615: 3 of 3 `dogfood-event-deadline` |

Process start (`corvint --version`) under 45-way load: 40-48 ms.

## Finding

- The deadline expiry reproduces at load ~60 on this 12-CPU host, and it reproduces on the **clean**
  clone, not the dirty one. The 1,266 store-written `.taskman/` projection paths add no measurable time;
  the dirty clone's Git status reads were faster than the fresh clean clone's.
- Under load the snapshot decode (177 MB) and the two Git status/identity reads dominate, each
  stretching 2-4x; process start and the local-completion evaluation do not matter.
- V1-1496's own measurement agrees: 4,820 dirty paths took 0.37 s for `git status` at load 64.

## Decision

Keep failing open (`LCP-V0-008`, `AHI-021`). No change to the deadline, the probe or the snapshot. The
AHI-055 notice no longer guesses "a very large set of uncommitted or staged changes, or heavy machine
load" and no longer suggests committing or stashing: it names this reproduction's measured cause
(load), says the store's `.taskman/` files were measured not to matter, and gives an operator
remediation (waiting for load to drop may help if load caused the timeout, with no promised recovery
because this event was not measured; do not commit, stash or discard `.taskman/` files) that never
touches store-owned files. It still says the notice did not measure the current event, as AHI-055
forbids claiming an unobserved cause.

Live per-event measurement in the notice (phase that expired, dirty count, load average) stays with
V1-1496. Reducing the snapshot decode under load is a separate performance question for the index
format, not this lane.

## Evidence

- `cmd/corvint/host_adapter.go` `deadlineRemediation`; `assertDeadlineRemediation` in
  `cmd/corvint/host_adapter_test.go` pins the new lines through `TestClaudeAdapterDogfoodEventDeadlineCarriesNoNotice`,
  `TestClaudeAdapterStopDeadlineFailsOpenVisibly`, `TestCodexAdapterDeadlineNamesRemediation` and
  `TestClaudeCompactionDeadlineNamesRemediation`.
- Rollback: revert the text change; no state or wire format changes.
