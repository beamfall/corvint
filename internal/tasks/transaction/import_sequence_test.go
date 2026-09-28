package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
	"testing"
)

func TestCALV0018_SequenceParityAndOwnership(t *testing.T) {
	t.Run("CAL-V0-018 sequence parity", testCALV0018SequenceParity)
}
func testCALV0018SequenceParity(t *testing.T) {
	in := roadmapInitialized(t)
	request := importRequest(imported("BF-1", "one"))
	sequence, err := NewImportSequence(request.QueueID, request.Actor, in)
	if err != nil {
		t.Fatal(err)
	}
	defer sequence.Close()
	originalQueue := bytes.Clone(in.Queue)
	originalHead := bytes.Clone(in.Head)
	in.Queue[0] = '!'
	in.Head[0] = '!'
	delete(in.Inventory.files, "VERSION")
	// The caller can destroy all seed containers without changing owned state.
	in = roadmapInitialized(t)
	if !bytes.Equal(sequence.input.Queue, originalQueue) || !bytes.Equal(sequence.input.Head, originalHead) || !sequence.Has("VERSION") {
		t.Fatal("seed aliases")
	}
	var retained *ImportBatch
	var priorReceipt, priorDescriptor []byte
	var priorArtifacts []Artifact
	for n := 0; n < 3; n++ {
		rec := imported("BF-1", "changed")
		if n == 0 {
			rec = imported("BF-1", "one")
		} else {
			pre, _ := ticket.Decode(in.CanonicalTickets[0])
			rec.Revision = wire.CountOf(int64(n + 1))
			rec.AcceptanceRevision = wire.CountOf(int64(n + 1))
			hash := pre.FileDigest()
			rec.PreviousRecordSha256 = &hash
		}
		request = importRequest(rec)
		request.RequestID = "batch-" + string(wire.CountOf(int64(n+1)))
		oracle := Model(request, in)
		if oracle.Plan == nil {
			t.Fatalf("oracle: %+v", oracle)
		}
		capacity, err := CheckCapacity(oracle.Plan)
		if err != nil {
			t.Fatal(err)
		}
		before := sequence.input.Inventory.Files()
		result, batch := sequence.Prepare(request, in)
		if batch == nil {
			t.Fatalf("sequence: %+v", result)
		}
		if result.Plan != nil || result.Final != nil || result.Coverage != coverage() {
			t.Fatal("mutable authority escaped")
		}
		if !reflect.DeepEqual(before, sequence.input.Inventory.Files()) {
			t.Fatal("prepare advanced inventory")
		}
		if !bytes.Equal(batch.Descriptor(), oracle.Plan.Descriptor()) || !bytes.Equal(batch.Receipt(), oracle.Plan.Receipt()) || !bytes.Equal(batch.Head(), oracle.Plan.Head()) || !reflect.DeepEqual(batch.Artifacts(), oracle.Plan.Artifacts()) || !reflect.DeepEqual(batch.Prefixes(), capacity.Prefixes) {
			t.Fatalf("batch %d differs from full model", n)
		}
		got := batch.Receipt()
		got[0] = '!'
		arts := batch.Artifacts()
		arts[0].Data[0] = '!'
		prefix := batch.Prefixes()
		prefix[0].Cost.Files = 999
		if !bytes.Equal(batch.Receipt(), oracle.Plan.Receipt()) || !reflect.DeepEqual(batch.Artifacts(), oracle.Plan.Artifacts()) || !reflect.DeepEqual(batch.Prefixes(), capacity.Prefixes) {
			t.Fatal("accessor aliases")
		}
		if err = sequence.Advance(batch); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sequence.input.Inventory.Files(), capacity.Final.Files()) {
			t.Fatal("final metadata differs")
		}
		if n == 0 {
			retained = batch
			priorReceipt = batch.Receipt()
			priorDescriptor = batch.Descriptor()
			priorArtifacts = batch.Artifacts()
		}
		if !bytes.Equal(retained.Receipt(), priorReceipt) || !bytes.Equal(retained.Descriptor(), priorDescriptor) || !reflect.DeepEqual(retained.Artifacts(), priorArtifacts) {
			t.Fatal("retained publication changed")
		}
		in = commitModel(t, in, oracle.Plan)
		in.HeadReceipt = oracle.Plan.Receipt()
		in.CanonicalTickets = [][]byte{rec.Encode()}
	}
	if err = sequence.Advance(retained); err == nil {
		t.Fatal("duplicate advance accepted")
	}
	if _, batch := sequence.Prepare(request, in); batch != nil {
		t.Fatal("closed cursor reused")
	}
}

func TestCALV0018_SequenceRefusalParity(t *testing.T) {
	t.Run("CAL-V0-018 refusal parity", testCALV0018SequenceRefusal)
}
func testCALV0018SequenceRefusal(t *testing.T) {
	for _, name := range []string{"native-holder", "revision", "actor", "branch", "replay", "queue", "clock"} {
		t.Run(name, func(t *testing.T) {
			in := roadmapInitialized(t)
			r := importRequest(imported("BF-1", "one"))
			if name == "native-holder" {
				pre := imported("BF-1", "one")
				pre.Source.Kind = "NATIVE"
				pre.Source.SourceItemID = nil
				pre.Source.SourceRevisionSha256 = nil
				pre.Source.SourceQueueID = r.QueueID
				pre.ShadowOverlay = false
				in = withTicket(t, in, pre, pre.Encode())
			}
			seq, err := NewImportSequence(r.QueueID, r.Actor, in)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "revision":
				rec := imported("BF-1", "one")
				rec.Revision = "3"
				r = importRequest(rec)
			case "actor":
				r.Actor.Role = "WORKER"
			case "branch":
				in.Branch = "other"
			case "replay":
				in.Replay.State = "UNKNOWN"
			case "queue":
				r.QueueID = "queue:other:main"
			case "clock":
				in.RecordedAt = "not-time"
			}
			full := Model(r, in)
			fast, batch := seq.Prepare(r, in)
			if full.Kind == "Transaction" || batch != nil || fast.Kind == "Transaction" {
				t.Fatalf("refusal missing: full=%+v fast=%+v", full, fast)
			}
			if _, batch = seq.Prepare(importRequest(imported("BF-2", "two")), in); batch != nil {
				t.Fatal("refused cursor reusable")
			}
		})
	}
}
