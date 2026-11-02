// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTLFV01102_BulkProgressRetryReplaysOverAPendingReceipt: a bulk
// progress batch with "current" entries, retried while another writer's
// receipt is pending, replays the completed entries instead of refusing
// them REQUEST_ID_CONFLICT. A refused entry whose ticket moved on since
// replays its recorded refusal rather than conflicting, as the single-target
// form does (TLF-V0-1102, V1-4290).
func TestTLFV01102_BulkProgressRetryReplaysOverAPendingReceipt(t *testing.T) {
	root, _ := ergStore(t)
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	aID, _ := progressTicket(t, root, "alpha", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["a/"]}`)
	bID, _ := progressTicket(t, root, "bravo", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["b/"]}`)
	cID, _ := progressTicket(t, root, "charlie", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["c/"]}`)
	dup := progressPayload("IN_PROGRESS", "", "", "", "")
	if x := atm(t, root, nil, "ticket", "progress", "--target", bID, "--expected-revision", "current", "--request-id", "pre-b", "--issued-at", "2026-11-01T11:00:00Z", "--payload", dup); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pre-mark: %s", x.stdout)
	}
	batch := `[{"target":"` + aID + `","expectedRevision":"current","payload":{"phase":"IN_REVIEW","pr":"20"}},` +
		`{"target":"` + bID + `","expectedRevision":"current","payload":` + dup + `}]`
	args := []string{"ticket", "progress", "--batch", "--request-id", "bulk", "--payload", batch, "--issued-at", "2026-11-01T12:00:00Z"}
	outcome := func(x run, i int) (string, []string) {
		t.Helper()
		if len(x.res.Items) != 1 || len(field(x.res.Items[0], "entries").Arr) <= i {
			t.Fatalf("batch result has no entry %d: %s", i, x.stdout)
		}
		e := field(x.res.Items[0], "entries").Arr[i]
		return field(e, "outcome").Str, strs(field(e, "codes"))
	}
	first := tdsRun(t, root, args...)
	if o, _ := outcome(first, 0); o != "COMPLETED" {
		t.Fatalf("first batch: %s", first.stdout)
	}
	if o, codes := outcome(first, 1); o == "COMPLETED" || strings.Join(codes, ",") == wire.CodeRequestIDConflict {
		t.Fatalf("duplicate mark was not refused: %s", first.stdout)
	}
	// b moves on after its refused entry.
	if x := atm(t, root, nil, "ticket", "progress", "--target", bID, "--expected-revision", "current", "--request-id", "later-b", "--issued-at", "2026-11-01T12:30:00Z", "--payload", progressPayload("BLOCKED", "", "", "", "owner")); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("later mark: %s", x.stdout)
	}
	holdPending(t, root, func() {
		if x := atm(t, root, nil, "ticket", "progress", "--target", cID, "--expected-revision", "current", "--request-id", "held-c", "--issued-at", "2026-11-01T13:00:00Z", "--payload", dup); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("held write: %s", x.stdout)
		}
	})
	retry := tdsRun(t, root, args...)
	if o, codes := outcome(retry, 0); o != "COMPLETED" || !field(field(retry.res.Items[0], "entries").Arr[0], "replayed").Bool || len(codes) != 0 {
		t.Fatalf("completed entry did not replay over a pending receipt: %s", retry.stdout)
	}
	if _, codes := outcome(retry, 1); strings.Contains(strings.Join(codes, ","), wire.CodeRequestIDConflict) {
		t.Fatalf("refused entry conflicts on retry: %s", retry.stdout)
	}
}
