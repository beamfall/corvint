// SPDX-License-Identifier: Apache-2.0

package flowreports

// Help documents the three reports; `corvint flows --help` prints it (AFL-V1-001, AFL-V1-007).
const Help = `
Flow reports that join flows with test results run in corvint (PROD-DOCS-011, AFL-V1):
  corvint [--root PATH] flows status --flows DIR [--receipt FILE]... [--evidence FILE]... [--provider FILE] [--no-store] [--base SHA]
  corvint [--root PATH] flows impact --flows DIR --base SHA [--no-store]
  corvint [--root PATH] flows gaps --flows DIR [--evidence FILE]... [--no-store] [--registry FILE] [--provider FILE]

Required: --flows DIR, the committed application-flow-intent/1 directory read at HEAD, and for
impact --base SHA. A .corvint/flows.json of an unsupported or invalid profile refuses the whole
report (FLOWS_SCHEMA_UNSUPPORTED or FLOWS_DECLARATION_INVALID, exit 2, nothing on stdout).
status writes application-flow-status/0: per flow and variation, the attached tests and the last
recorded result of each (passed, failed, flaky, timedOut, skipped, interrupted or not-run) or
NOT_OBSERVED. Results come only from two writers: corvint test receipts (corvint-test-run/0) and
flows ingest documents (test-run-evidence/0). By default status reads the private run evidence
store at <git-dir>/corvint/evidence, which both append to, and the default receipt
<git-dir>/corvint/test-run.json when the store does not already hold it; each --receipt FILE and
--evidence FILE adds to them and --no-store reads only the named files. result_sources lists every
source, read, absent, in-store or not-read. The newest record naming a test gives its result;
stale with its source_commit when it is bound to another commit or tree. A receipt names a test
file or package, never one title, so its outcome is shown with granularity file. --provider FILE
adds the inventory state and the unassigned count of E2E tests no variation links; without it
unassigned is null. --base SHA limits the report to exactly the flows and variations flows impact
names and carries its impact_graph. At most 8192 test rows, then truncated with full totals.
impact writes application-flow-impact/1: the flows, variations and test keys reached from
base..HEAD changes, with the path of each hop, and last_results per flow when the store holds a
record. gaps writes application-flow-gaps/1; any gap makes a flow incomplete; --provider FILE also
lists the unlinked E2E inventory tests and the inventory's state (AFU-V1-150).
A malformed, oversized or other-profile --receipt or --evidence file refuses the whole report
(invalid-arguments, exit 2). The reports write nothing. corvint.flows_status,
corvint.flows_impact and corvint.flows_gaps on corvint mcp --tool-profile flows return the same
reports.
Example: corvint flows status --flows flows --receipt .git/corvint/test-run.json
Example: corvint flows impact --flows flows --base "$(git merge-base HEAD origin/main)"
`
