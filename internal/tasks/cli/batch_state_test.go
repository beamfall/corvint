// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV01103_MissingStateNamesTheField covers V1-4267: a batch record
// without `state` is refused MALFORMED naming the field and the full field
// list, never "message withheld"; a present non-MERGED state still refuses
// PREREQUISITE_UNSATISFIED naming it (TCD-V0-1103).
func TestTCDV01103_MissingStateNamesTheField(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	m := strings.Repeat("a", 40)
	runs := tdsExport(t, "runs.json", `[{"databaseId":5,"headSha":"`+m+`","conclusion":"success","url":"https://github.com/o/r/actions/runs/5"}]`)
	fields := "number,state,title,body,headRefName,headRefOid,mergeCommit,labels,mergedAt"
	for _, rec := range []string{
		`{"number":30,"title":"Batch x","body":"","mergeCommit":{"oid":"` + m + `"},"labels":[]}`,
		`{"number":30,"state":"","title":"Batch x","body":"","mergeCommit":{"oid":"` + m + `"},"labels":[]}`,
	} {
		prs := tdsExport(t, "prs.json", "["+rec+"]")
		for _, args := range [][]string{
			{"ticket", "batch-delivered", "--pr", "30", "--prs", prs},
			{"ticket", "batch-close", "--pr", "30", "--prs", prs, "--runs", runs},
			{"ticket", "batch-close", "--pr", "30", "--prs", prs, "--runs", runs, "--manual"},
		} {
			x := tdsRun(t, root, args...)
			msg := strings.Join(x.res.Warnings, " ")
			if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(msg, "`state` field") ||
				!strings.Contains(msg, "gh pr list --state merged --json "+fields) || strings.Contains(msg, "withheld") {
				t.Fatalf("%v with %s: exit %d %s", args[:2], rec, x.code, x.stdout)
			}
		}
	}
	prs := tdsExport(t, "prs.json", `[{"number":30,"state":"OPEN","title":"Batch x","body":"","labels":[]}]`)
	x := tdsRun(t, root, "ticket", "batch-delivered", "--pr", "30", "--prs", prs)
	if !hasCode(x.res, wire.CodePrerequisiteUnsatisfied) || !strings.Contains(strings.Join(x.res.Warnings, " "), "PR #30 is OPEN, not merged") {
		t.Fatalf("OPEN batch: %s", x.stdout)
	}
}
