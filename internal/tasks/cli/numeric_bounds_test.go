package cli_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCD-V0-159: PR identifiers, ordering and read-only behavior survive wide inputs.
func TestV13179_DeliveryScreenPRIdentityPreserved(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, local := tcdTicket(t, root, "numeric-pr", "")
	numbers := []int64{9223372036854775807, 2147483648, 1, 4294967297}
	var rows []string
	for i, n := range numbers {
		state := "MERGED"
		if i == 1 {
			state = "OPEN"
		}
		rows = append(rows, fmt.Sprintf(`{"number":%d,"state":%q,"title":"lane","body":%q,"labels":[]}`, n, state, "Refs "+local))
	}
	prs := tdsExport(t, "prs.json", "["+strings.Join(rows, ",")+"]")
	intent, state := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intent), fixture.TreeSnapshot(t, state)
	x := tdsRun(t, root, "ticket", "delivery-screen", local, "--prs", prs)
	if got := tdsVerdicts(t, x)[local]; got != "IN_FLIGHT" {
		t.Fatalf("verdict %s: %s", got, x.stdout)
	}
	tickets := field(x.res.Items[0], "tickets").Arr
	hits := field(tickets[0], "prs").Arr
	want := []string{"1", "2147483648", "4294967297", "9223372036854775807"}
	if len(hits) != len(want) {
		t.Fatalf("PR hits: %s", x.stdout)
	}
	for i, hit := range hits {
		if field(hit, "number").Str != want[i] {
			t.Fatalf("PR ordering/identity: %s", x.stdout)
		}
	}
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intent)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, state)) {
		t.Fatal("read changed intent or state")
	}
}

// TCD-V0-163: the public command preserves identity in the envelope and rows.
func TestV13179_BatchDeliveredPRIdentityPreserved(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, local := tcdTicket(t, root, "numeric-batch", "")
	base := revParse(t, root, "HEAD")
	lane := tdsCommit(t, root, "lane", base)
	merge := tdsCommit(t, root, "merge", base, lane)
	for _, n := range []int64{1, 2147483647, 2147483648, 4294967297, 9223372036854775807} {
		number := strconv.FormatInt(n, 10)
		prs := tdsExport(t, "prs.json", fmt.Sprintf(`[{"number":20,"state":"MERGED","title":"lane","body":%q,"headRefOid":%q,"labels":[]},{"number":%d,"state":"MERGED","title":"Batch: #20","body":"","mergeCommit":{"oid":%q},"labels":[]}]`, "Refs "+local, lane, n, merge))
		x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", number, "--prs", prs)
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("batch %s: %s", number, x.stdout)
		}
		item := x.res.Items[0]
		delivered := field(item, "delivered").Arr
		if field(item, "batchPr").Str != number || len(delivered) != 1 || field(delivered[0], "batchPr").Str != number {
			t.Fatalf("batch identity %s: %s", number, x.stdout)
		}
	}
	for _, bad := range []string{"0", "-1", "9223372036854775808"} {
		x := tdsRun(t, root, "ticket", "batch-delivered", "--pr", bad, "--prs", "unused")
		if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("invalid %s: %s", bad, x.stdout)
		}
	}
}
