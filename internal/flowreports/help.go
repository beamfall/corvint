// SPDX-License-Identifier: Apache-2.0

package flowreports

// Help documents the three reports; `corvint flows --help` prints it (AFL-V1-001, AFL-V1-007).
const Help = `
Flow reports that join flows with test results run in corvint (PROD-DOCS-011, AFL-V1):
  corvint [--root PATH] flows status --flows DIR [--evidence FILE]... [--provider FILE] [--no-store] [--base SHA]
  corvint [--root PATH] flows impact --flows DIR --base SHA [--no-store]
  corvint [--root PATH] flows gaps --flows DIR [--evidence FILE]... [--no-store] [--registry FILE] [--provider FILE]

Required: --flows DIR, the committed application-flow-intent/1 directory read at HEAD, and for
impact --base SHA. A .corvint/flows.json of an unsupported or invalid profile refuses the whole
report (FLOWS_SCHEMA_UNSUPPORTED or FLOWS_DECLARATION_INVALID, exit 2, nothing on stdout).
status writes application-flow-status/0: per flow and variation, the attached tests (the targets
of its reviewed test links; a stale, unreviewed or inferred link attaches none) and the last
recorded result of each (passed, failed, flaky, timedOut, skipped, interrupted or not-run) or
NOT_OBSERVED. Results come only from two writers: the runs corvint test records and the
test-run-evidence/0 documents flows ingest writes. By default status reads the private run
evidence store at <git-dir>/corvint/evidence, to which corvint test appends every run and flows
ingest --record appends its document; each --evidence FILE adds to it and --no-store reads only
the named files. result_sources lists every source, read or not-read, with its record count. The
newest record naming a test gives its result; stale with its source_commit when it is bound to
another commit or tree. A corvint test run names a test file or package, never one title, so its
outcome is shown with granularity file, with its slice when the run covered part of the file. A
run names no Playwright project, so for a variation of several projects it stands for none:
NOT_OBSERVED with reason project-not-recorded. --provider FILE
adds the inventory state and the unassigned count of E2E tests no variation links; without it
unassigned is null. --base SHA limits the report to exactly the flows and variations flows impact
names and carries its impact_graph. At most 8192 test rows, then truncated with full totals.
impact writes application-flow-impact/1: the flows, variations and test keys reached from
base..HEAD changes, with the path of each hop, and last_results per flow when the store holds a
record. A changed path with no owning unit in the impact graph has unknown reach, linked or not:
it is listed in reach_unknown, and every flow is named with all its variations and reason
reach-unknown, never an empty or narrowed list; status --base then keeps every flow. A link target
with no owning unit (no adapter for its language, or a missing file) does the same for every
changed path, and graph.unknown names it as LINK_TARGET_UNOWNED. The flow declaration and the
.json files directly in the intents directory are flow inputs and are left out. gaps writes
application-flow-gaps/1; any gap makes a flow incomplete; --provider FILE also
lists the unlinked E2E inventory tests and the inventory's state (AFU-V1-150).
A malformed, oversized or other-profile --evidence file refuses the whole report
(invalid-arguments, exit 2), and so does a stored record that is not a document this build reads or
whose tests the store cannot name (run-evidence-corrupt); a record edited after it was written that
still parses is skipped and the tests it held show reason run-evidence-corrupt. The store and the named files are
bounded separately, 8192 records each. The reports write nothing. corvint.flows_status,
corvint.flows_impact and corvint.flows_gaps on corvint mcp --tool-profile flows return the same
reports.
Example: corvint flows status --flows flows --evidence runs/runs.jsonl
Example: corvint flows impact --flows flows --base "$(git merge-base HEAD origin/main)"
`
