# OpenCode v2 task workbench

Owner request on 2026-09-29: implement all six proposed OpenCode/Corvint additions—task
focus, acceptance-to-proof, qualification doctor, next actions, session economics and
multi-session map. AHI-036 through AHI-041 record this as an experimental, read-only
terminal slice. It builds on the existing AHI-033–035 context, change and Tasks views;
it does not add task mutation, test execution, a transcript scan or execution authority.

The change starts from public `origin/main` at
`723e268ab23f0ed929b6ff19c2ef4084f9ebf242` in an isolated worktree. The primary
checkout has unrelated local work and was not edited. The initialized native Tasks
store is private to that primary checkout; this worktree's `receipt audit` returned
`UNINITIALIZED`. The stock UI witness creates its own initialized fixture queue and
must not be mistaken for primary-queue execution or completion evidence.

Pre-change `corvint query` and path `impact` receipts are retained under the worktree's
private Git directory. Query surfaced the previous OpenCode Tasks build log and disclosed
two omitted ranked results and seven withheld test-path candidates. Path impact included
the OpenCode adapter, inspector and governing AHI spec, with one ranked result omitted.
`corvint affected --base 723e268a…` on the dirty implementation reported scope
`UNKNOWN`, 144 selected units and 23 unknown rows, including the non-Go language
frontier. This is test-selection advice, not proof of coverage.

The workbench binds ticket detail to the Tasks head receipt, retains only an explicit
session focus, and refuses stale joins. Criteria remain `UNOBSERVED` because neither
the ticket record nor the existing CEM/OCM supplies a criterion-specific proof link.
Ticket-level references, declared paths, changed paths, check plans and observed check
states are displayed separately. The doctor reads exact package/host qualification;
the latest changed package remains `UNQUALIFIED` until an exact tuple campaign passes.
Cost since focus uses a volatile TUI baseline and may include other work after focus;
savings and cost per verified criterion remain `NOT_OBSERVED`. Session-family ownership
is explicit; inaccessible cross-worktree bindings are `UNOBSERVED`.

The existing stock OpenCode 2.0.18 UI witness was extended to focus a ticket and inspect
all five workbench tabs. Its first two attempts missed focus after an asynchronous Tasks
refresh; the third refused an intermittently empty host version probe. A subsequent run
captured every new tab but exposed an unset host identity in the interruption witness.
The qualifier now waits for a settled Tasks page, retries only empty successful version
output at most three times, and carries the parsed host identity into that witness.
Nonempty unsupported versions still fail closed. The final stock UI witness passed in
both dark and light themes, including pointer ticket focus, all five workbench tabs and
interruption cleanup with no surviving descendants:
`/private/tmp/opencode-v2-native6/report.json` and
`/private/tmp/opencode-v2-native6/light/report.json`. This is UI evidence only; it does
not promote an exact adapter/host qualification tuple.

Verification before final binding: 33 focused OpenCode JavaScript tests passed. The
OpenCode TSX bundle parsed. `TestHostAdapterJavaScriptHosts` passed with the added
workbench RPC race case. The focused Go qualifier suite passed outside the sandbox;
inside the sandbox its descendant snapshot was unavailable. Requirements generation,
definition and traceability checks passed on the staged spec. Independent review found
race, attribution, retired-session and inaccessible-peer errors; these were repaired,
and its focused re-review found no remaining defect.
The repository-wide `make gate` remains `NOT_RUN` under the owner’s scoped-work
preference. Exact-package native qualification and any usefulness or savings claim
remain open unless separately evidenced.

Rollback restores the previous OpenCode package/configuration. Its workbench focus is
volatile, so rollback leaves Tasks records, CEMs and verification receipts untouched.
