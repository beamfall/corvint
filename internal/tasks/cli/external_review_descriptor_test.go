package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0009_ReviewDescriptorMeasured measures the MUTATE stage
// descriptor of an actual native review record against the six-artifact,
// 1670-byte bound (ERG-V0-009). The record uses the widest request id
// (64 bytes that each escape to two) and an EVIDENCE-free TREE candidate;
// the descriptor carries only paths, digests and sizes, so the request's
// reasons and evidence do not widen it. The descriptor is rebuilt from the
// committed receipt, its posted files and head exactly as the planner
// assembles it (POST per non-null post, RECEIPT, HEAD, EVIDENCE per blob
// post not itself posted), then encoded under the stage limits, and must
// not exceed the 1669-byte maximal derived-event descriptor that
// TestONV0006_DerivedEventSlotMeasuredAndNarrow measures for every MUTATE.
func TestERGV0009_ReviewDescriptorMeasured(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "measured", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	requestID := strings.Repeat(`"`, 64)
	runOK("gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject, "--expected-generation", "0",
		"--expected-revision", "0", "--request-id", requestID, "--reason", "TESTS:"+strings.Repeat("x", 200))

	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	headRaw := read(filepath.Join(repo.StateDir, "head.json"))
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	seq := head.LastSeq.Uint64()
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		t.Fatal(err)
	}
	receiptRaw := read(filepath.Join(repo.StateDir, "receipts", name))
	rc, err := snapshot.DecodeReceipt(receiptRaw)
	if err != nil || rc.RequestID == nil || *rc.RequestID != requestID || rc.Prev == nil {
		t.Fatalf("the head receipt is not the review record: %+v %v", rc, err)
	}
	d := snapshot.StageDescriptor{QueueID: head.QueueID.Raw, Operation: snapshot.StageMutate, RequestID: requestID,
		RequestSha256: wire.Sum([]byte("any digest has the same width")), RecordedAt: rc.RecordedAt,
		Base: &snapshot.StageBase{LastSeq: wire.SizeOf(seq - 1), LastReceiptSha256: *rc.Prev}}
	add := func(role, target string, raw []byte) {
		d.Artifacts = append(d.Artifacts, snapshot.StageDescription{Role: role, Target: target, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
	}
	posted := map[string]bool{}
	var blobs []string
	for _, p := range rc.Post {
		if p.Sha256 == nil {
			continue
		}
		posted[p.Path] = true
		var raw []byte
		if strings.HasPrefix(p.Path, "intent/") {
			raw = read(filepath.Join(repo.IntentRoot(), intent.Dir, strings.TrimPrefix(p.Path, "intent/")))
		} else {
			raw = read(filepath.Join(repo.StateDir, p.Path))
		}
		if wire.Sum(raw) != *p.Sha256 {
			t.Fatalf("%s does not hash to its post", p.Path)
		}
		add("POST", p.Path, raw)
		if p.BlobSha256 != nil {
			blobs = append(blobs, "evidence/"+string(*p.BlobSha256))
		}
	}
	for _, b := range blobs {
		if !posted[b] {
			add("EVIDENCE", b, read(filepath.Join(repo.StateDir, b)))
		}
	}
	add("RECEIPT", "receipts/"+name, receiptRaw)
	add("HEAD", "head.json", headRaw)
	sort.Slice(d.Artifacts, func(i, j int) bool {
		a, b := d.Artifacts[i], d.Artifacts[j]
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		return a.Target < b.Target
	})
	for i := range d.Artifacts {
		d.Artifacts[i].Slot = fmt.Sprintf("a%02d", i)
	}
	raw, err := d.Encode()
	slots, limit := snapshot.StageLimits(snapshot.StageMutate)
	if err != nil || len(d.Artifacts) > slots || len(raw) > limit || len(raw) > 1669 {
		t.Fatalf("review descriptor %d bytes / %d artifacts within %d / %d: %v", len(raw), len(d.Artifacts), limit, slots, err)
	}
	t.Logf("actual review MUTATE descriptor: %d artifacts, %d of %d bytes", len(d.Artifacts), len(raw), limit)
}
