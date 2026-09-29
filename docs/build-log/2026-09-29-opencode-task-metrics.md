# OpenCode task-manager metrics

Owner intent on 2026-09-29: show Corvint Tasks (`.taskman`) metrics in both the OpenCode sidebar
and `/corvint` cockpit. AHI-035 freezes the read-only slice: status totals, a bounded open-ticket
page, selected blocker and acceptance detail, explicit refresh, and honest unavailable states.
The change was first built from public `origin/main` at
`624e9aecbf71993d26c57b2e20837282469cb02c`. Before publication, main advanced through
OpenCode 2.0.19 qualification. The task source was integrated in a fresh isolated clone from
`a15d281dc922293d0a9a853175c0d56b7840c8af`; the primary checkout's concurrent queue
changes remain untouched.

The client calls only `queue status`, `ticket search --status OPEN`, and `ticket show` through the
existing fixed-argv owned-process runner. Summary, page and detail must share one Tasks head
receipt. Draft, open, completed, held and archived counts remain distinct from queue blockers,
intent checks, gate observations and native completion. The sidebar and panel hold only one
bounded, volatile observation per session. The CLI's native intent and receipt store remains the
authority; these views neither mutate it nor grant execution authority. Rollback removes the
OpenCode Tasks read/UI path and leaves `.taskman` and its receipts untouched.

Corvint orientation returned `OUT_OF_SCOPE` below its relevance floor in the mixed primary
worktree; no result was promoted to authority. `corvint affected --base 624e9aec...` returned
`scope: UNKNOWN` with 143 selected units and explicit language-frontier unknowns. The tracked Go
qualifier path had no ranked `impact` result. AHI-035 and the native Tasks read contract supplied
the governing context. A live read of the primary store returned 486 total, 302 open, 32 bounded
rows, and one active attempt at its observed receipt; a fresh clone without a private Tasks
journal correctly refused `UNINITIALIZED`. The seeded test store exercises a one-ticket positive
path. No queue mutation was performed in the primary checkout.
The final-base `affected` plan also reported `scope: UNKNOWN`, retaining its language-frontier
unknowns.

Independent review found three correctness gaps and they were repaired: DRAFT was omitted from
the total, a replaced snapshot could retain a stale ticket detail, and an older cancelled detail
read could overwrite a newer one. A fourth finding exposed malformed blocker/record values that
could appear as zero blockers; the projection now rejects them. Focused regression tests cover
counts, page omissions, receipt races, malformed data, fixed commands, refusal, hostile text,
and cancelling a Tasks read with a descendant process. The native stock OpenCode 2.0.18 terminal
run captured Tasks sidebar, page, detail and refresh frames in both dark and light themes under
`/private/tmp/corvint-opencode-task-witness-run3/`. Its final pre-existing interruption observer
reported `descendant snapshot unavailable`, so the whole inspector campaign is not a PASS.
The focused interruption witness passed independently outside the sandbox. The Tasks frames are
UI evidence only and do not promote package qualification or execution authority.
On the final base, the child-start assertion missed its 500 ms deadline while a Go host test was
compiling in parallel; the quiet rerun passed. The test now permits up to three seconds for child
startup while still checking that cancellation retires the process group.
An exact-commit repeat captured the Tasks page and detail again at
`/private/tmp/corvint-opencode-task-witness-final/`; its terminal cleanup observer reported
`terminal cleanup incomplete` after the UI capture. The observed PIDs were gone when checked
after the command. The complete stock-host campaign therefore remains `NOT_PASS`.
Final-base independent review found that the sidebar omitted the draft, held and archived counts
required by AHI-035; the sidebar now shows those counts alongside completed and open.
On the final `a15d281d...` base, stock OpenCode again captured sidebar, Tasks page, detail,
refresh and cockpit frames at `/private/tmp/corvint-opencode-task-witness-v2/`. Its later
qualification version probe received empty stdout and refused the host, despite a direct
`/opt/homebrew/bin/opencode --version` reading `opencode v2.0.18`. The campaign remains
`NOT_PASS`; the focused `TestGateInterruptionWitness` passed on this base.

Focused JS tests, the OpenCode TSX bundle parse, `TestHostAdapterJavaScriptHosts`, focused Go
qualification tests, and generated requirement checks passed before the final binding. The
repository-wide `make gate` is `NOT_RUN` under the owner's scoped-work preference. Native package
qualification for the changed 0.7.4 source remains `UNQUALIFIED` until its exact installed host
tuple passes the complete campaign. Billed tokens and cache savings are `NOT_OBSERVED`.
