package cli

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func packetField(v wire.Value, k string) wire.Value {
	x, _ := v.Obj.Get(k)
	return x
}

// TestERGV0020_TouchedObligations: only entries whose matched test paths or
// reviewed example cite a changed path are listed, with their test
// identities and cited lines (ERG-V0-020).
func TestERGV0020_TouchedObligations(t *testing.T) {
	l := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{
		{ID: "AC-1", State: ticket.ObligationWitnessed, Evidence: &ticket.ObligationEvidence{Matches: []ticket.ObligationMatch{{Path: "e2e/login.spec.ts", TestID: "t2"}, {Path: "e2e/login.spec.ts", TestID: "t1"}}}},
		{ID: "AC-2", State: ticket.ObligationOpen, Evidence: &ticket.ObligationEvidence{Matches: []ticket.ObligationMatch{{Path: "e2e/other.spec.ts", TestID: "t3"}}}},
		{ID: "AC-3", State: ticket.ObligationWitnessed, Review: &ticket.ObligationReviewRecord{Review: ticket.ObligationReview{Subject: ticket.ObligationReviewSubject{Path: "src/a_test.go", StartLine: 4, EndLine: 9}}}},
		{ID: "AC-4", State: ticket.ObligationOpen},
	}}
	got := touchedObligations(l, []string{"e2e/login.spec.ts", "src/a_test.go", "src/a.go"})
	if len(got) != 2 || got[0].ID != "AC-1" || strings.Join(got[0].TestIDs, ",") != "t1,t2" || strings.Join(got[0].Paths, ",") != "e2e/login.spec.ts" ||
		got[1].ID != "AC-3" || got[1].Lines != "src/a_test.go:4-9" {
		t.Fatalf("touched obligations: %+v", got)
	}
}

// TestERGV0019_SkeletonAndRanges: hunks are named by their commit line range
// (or base range for a deletion); a skeleton over the cap is refused
// LIMIT_EXCEEDED; a checkpoint that does not fit is named, with its commit
// binding NOT_OBSERVED (ERG-V0-019, 021).
func TestERGV0019_SkeletonAndRanges(t *testing.T) {
	h := splitHunks("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,2 +1,3 @@\n a\n+b\n c\n@@ -10,2 +11,0 @@\n-d\n-e\n")
	if len(h) != 2 || h[0].Range != "lines 1-3" || h[1].Range != "base lines 10-11" || !strings.HasPrefix(h[1].Text, "@@ -10,2") {
		t.Fatalf("hunks %+v", h)
	}
	p := &reviewPacket{Ticket: "T-1", Commit: strings.Repeat("c", 40), Base: strings.Repeat("b", 40), BaseKind: PacketBasePassed, BaseSource: "PASSED_ANCESTOR",
		Scope: PacketScopeTests, ObligationsState: "NONE", ReturnGate: "g", ReturnEvent: strings.Repeat("e", 64),
		Checkpoint: &dispatch.HandoffNote{Worker: "w-1", Ticket: "T-1", Role: "review", At: "2026-10-18T00:00:00Z", Messages: []string{strings.Repeat("m", 6000)}}, CheckpointSource: "note.json"}
	v, err := p.build(4096)
	if err != nil {
		t.Fatal(err)
	}
	cp := packetField(v, "checkpoint")
	om := packetField(v, "omitted").Arr
	if packetField(cp, "text").Kind != wire.KindNull || packetField(cp, "commitBinding").Str != string(ticket.NotObserved) || len(om) != 1 || packetField(om[0], "section").Str != "checkpoint" {
		t.Fatalf("checkpoint under the cap: %s", wire.Encode(v))
	}
	for i := 0; i < 200; i++ {
		p.Items = append(p.Items, packetItem{Code: "CODE", Text: "x", Paths: []string{"src/a_test.go"}})
	}
	if _, err := p.build(4096); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("an oversized skeleton was not refused: %v", err)
	}
}

// TestERGV0019_MetadataOnlyChangeKeepsHunk: a mode-only change and a new
// empty file have no content hunk, yet each keeps a "metadata" hunk naming
// the change, ahead of any content hunk (Codex #1189 finding 3).
func TestERGV0019_MetadataOnlyChangeKeepsHunk(t *testing.T) {
	for name, tc := range map[string]struct{ diff, meta string }{
		"mode only":      {"diff --git a/x b/x\nold mode 100755\nnew mode 100644\n", "old mode 100755\nnew mode 100644\n"},
		"empty new file": {"diff --git a/x b/x\nnew file mode 100644\nindex 0000000..e69de29\n", "new file mode 100644\n"},
		"mode and lines": {"diff --git a/x b/x\nold mode 100644\nnew mode 100755\nindex 1..2\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", "old mode 100644\nnew mode 100755\n"},
		"deleted empty":  {"diff --git a/x b/x\ndeleted file mode 100644\nindex e69de29..0000000\n", "deleted file mode 100644\n"},
	} {
		h := splitHunks(tc.diff)
		if len(h) == 0 || h[0].Range != "metadata" || h[0].Text != tc.meta {
			t.Fatalf("%s: hunks %+v", name, h)
		}
	}
	if h := splitHunks("diff --git a/x b/x\nindex 1..2\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"); len(h) != 1 || h[0].Range != "lines 1-1" {
		t.Fatalf("content-only diff gained a metadata hunk: %+v", h)
	}
}

// TestERGV0017_ReturnRecencyIsReceiptOrder: two RETURNs on different gates
// with the same recordedAt second resolve to the one later in the journal,
// whatever the gates' name order (Codex #1189 finding 4).
func TestERGV0017_ReturnRecencyIsReceiptOrder(t *testing.T) {
	ret := "RETURN"
	event := func(seq uint64, code string) *snapshot.ExternalReviewEvent {
		return &snapshot.ExternalReviewEvent{RecordedAt: wire.Timestamp("2026-10-18T00:00:00Z"), ReceiptSeq: wire.SizeOf(seq),
			Request: snapshot.ExternalReviewRequest{Verdict: &ret, Reasons: []snapshot.ExternalReviewReason{{Code: code, Text: code}}}}
	}
	for _, order := range [][]string{{"a", "z"}, {"z", "a"}} {
		p := &reviewPacket{}
		var newest uint64
		seqs := map[string]uint64{"a": 10, "z": 11}
		for _, gate := range order {
			p.noteEvent(gate, event(seqs[gate], "FROM_"+gate), []byte(gate), map[string]string{}, &newest)
		}
		if p.ReturnGate != "z" || len(p.Items) != 1 || p.Items[0].Code != "FROM_z" {
			t.Fatalf("order %v: return gate %s items %+v", order, p.ReturnGate, p.Items)
		}
	}
}
