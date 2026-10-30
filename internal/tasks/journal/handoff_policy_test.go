package journal

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func handoffSelector(raw []byte) HandoffPolicySelector {
	return HandoffPolicySelector{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", OriginalPolicySha256: wire.Sum(raw)}
}

// This read test observes policy compatibility without inventing the missing
// generation provenance. Store tests below exercise genuine claim afterimages.
func TestCALV0044_HistoryStreamsEveryAfterimageWithoutCheckpoint(t *testing.T) {
	for _, restored := range []bool{false, true} {
		repo, r := setup(t)
		original := read(t, filepath.Join(repo.IntentDir, "policy.json"))
		v, _ := wire.Parse(original)
		v.Obj.Set("policyVersion", str("2"))
		if restored {
			v.Obj.Set("cemRequired", wire.Bool(true))
		}
		appendReceipt(t, repo, snapshot.StagePolicyUpdate, map[string][]byte{"intent/policy.json": wire.EncodeFile(v)}, "", true, true, false)
		v.Obj.Set("policyVersion", str("3"))
		v.Obj.Set("cemRequired", wire.Bool(false))
		appendReceipt(t, repo, snapshot.StagePolicyUpdate, map[string][]byte{"intent/policy.json": wire.EncodeFile(v)}, "", true, true, false)
		r.Checkpoint = checkpointed(t, repo, r)
		counter := &countingSource{Native: r.Source.(Native), reads: map[string]int{}}
		r.Source = counter
		before := fixture.TreeSnapshot(t, repo.Root)
		result, err := r.AuditForHandoff(handoffSelector(original))
		if err != nil {
			t.Fatal(err)
		}
		if result.Mode != ModeFull || result.HandoffPolicy.Compatible == restored || result.HandoffPolicy.OriginalPolicy.Seq != "1" || result.HandoffPolicy.OriginalReceiptSha256 == "" || result.HandoffPolicy.FirstAttemptSeq != "" || result.HistoricalAcceptance != "NOT_OBSERVED" {
			t.Fatalf("unexpected history: %+v", result.HandoffPolicy)
		}
		for seq := uint64(1); seq <= result.LastSeq.Uint64(); seq++ {
			name, _ := snapshot.ReceiptName(seq)
			if counter.reads["receipts/"+name] != 1 {
				t.Fatalf("receipt %d consumed %d times in one stable full audit", seq, counter.reads["receipts/"+name])
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.Root)) {
			t.Fatal("history audit mutated repository state")
		}
	}
}

func TestCALV0044_OriginalPolicyUsesAggregateByteBudget(t *testing.T) {
	repo, r := setup(t)
	original := read(t, filepath.Join(repo.IntentDir, "policy.json"))
	sel := handoffSelector(original)
	r.handoffPolicy = &sel
	for _, n := range []int{2*len(original) - 1, 2 * len(original)} {
		_, err := r.audit([]string{"intent/policy.json"}, "", limits{scan: wire.MaxArchiveScanEntries, selected: n}, true)
		if n == 2*len(original)-1 {
			requireCode(t, err, wire.CodeLimitExceeded)
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestCALV0044_IncompatibleHistoryStillChecksLaterCodec(t *testing.T) {
	repo, r := setup(t)
	original := read(t, filepath.Join(repo.IntentDir, "policy.json"))
	v, _ := wire.Parse(original)
	v.Obj.Set("policyVersion", str("2"))
	v.Obj.Set("cemRequired", wire.Bool(true))
	appendReceipt(t, repo, snapshot.StagePolicyUpdate, map[string][]byte{"intent/policy.json": wire.EncodeFile(v)}, "", true, true, false)
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("unknown", wire.Bool(true))
	appendReceipt(t, repo, snapshot.StagePolicyUpdate, map[string][]byte{"intent/policy.json": wire.EncodeFile(v)}, "", true, true, false)
	_, err := r.AuditForHandoff(handoffSelector(original))
	requireCode(t, err, wire.CodeMalformed)
}
