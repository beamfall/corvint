package transaction

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
	"strings"
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
	zero := &ImportSequence{}
	if _, batch := zero.Prepare(importRequest(imported("BF-1", "one")), Input{}); batch != nil || zero.Has("head.json") {
		t.Fatal("zero cursor admitted")
	}

	for _, name := range []string{"native-holder", "revision", "actor", "branch", "replay", "queue", "clock", "fold-store", "fold-batch"} {
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
			if name == "fold-store" {
				pre := imported("BF-1", "one")
				in = withTicket(t, in, pre, pre.Encode())
				r = importRequest(imported("bf-1", "one"))
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
			case "fold-batch":
				r = importRequest(imported("BF-2", "two"), imported("bf-2", "two"))
			}
			full := Model(r, in)
			fast, batch := seq.Prepare(r, in)
			if full.Kind == "Transaction" || batch != nil || fast.Kind == "Transaction" {
				t.Fatalf("refusal missing: full=%+v fast=%+v", full, fast)
			}
			if strings.HasPrefix(name, "fold-") && (len(fast.Outcome.Codes) != 1 || fast.Outcome.Codes[0] != wire.CodeDuplicateID) {
				t.Fatalf("case-fold refusal: %v %s", fast.Outcome.Codes, fast.Detail)
			}
			// The sequence is bound to its actor, so a changed actor refuses as a binding mismatch.
			if name != "actor" && !reflect.DeepEqual(full.Outcome.Codes, fast.Outcome.Codes) {
				t.Fatalf("refusal codes differ: full=%v (%s) fast=%v (%s)", full.Outcome.Codes, full.Detail, fast.Outcome.Codes, fast.Detail)
			}
			if _, batch = seq.Prepare(importRequest(imported("BF-2", "two")), in); batch != nil {
				t.Fatal("refused cursor reusable")
			}
		})
	}
}

func TestCALV0018_SequenceInvalidation(t *testing.T) {
	t.Run("CAL-V0-018 cursor invalidation", testCALV0018Invalidation)
}
func testCALV0018Invalidation(t *testing.T) {
	for _, field := range []string{"head", "receipt", "queue", "policy", "barrier", "reservations", "pending", "foreign-token", "close", "actor"} {
		t.Run(field, func(t *testing.T) {
			in := roadmapInitialized(t)
			r := importRequest(imported("BF-1", "one"))
			seq, err := NewImportSequence(r.QueueID, r.Actor, in)
			if err != nil {
				t.Fatal(err)
			}
			before := seq.input.Inventory.Files()
			switch field {
			case "head":
				in.Head = []byte("{}\n")
			case "receipt":
				in.HeadReceipt = []byte("{}\n")
			case "queue":
				in.Queue = []byte("{}\n")
			case "policy":
				in.Policy = []byte("{}\n")
			case "barrier":
				in.Barrier = []byte("{}\n")
			case "reservations":
				in.Reservations = []byte("{}\n")
			case "actor":
				r.Actor.ID = "different"
			case "close":
				seq.Close()
			case "pending":
				if _, b := seq.Prepare(r, in); b == nil {
					t.Fatal("first prepare refused")
				}
			case "foreign-token":
				other, err := NewImportSequence(r.QueueID, r.Actor, in)
				if err != nil {
					t.Fatal(err)
				}
				_, foreign := other.Prepare(r, in)
				_, own := seq.Prepare(r, in)
				if own == nil || foreign == nil {
					t.Fatal("missing batch")
				}
				if err = seq.Advance(foreign); err == nil {
					t.Fatal("foreign token accepted")
				}
			}
			if result, b := seq.Prepare(r, in); b != nil || result.Kind == "Transaction" {
				t.Fatal("invalidated cursor admitted")
			}
			if !reflect.DeepEqual(before, seq.input.Inventory.Files()) {
				t.Fatal("failed computation advanced metadata")
			}
		})
	}
	for _, field := range []string{"head", "queue", "ticket", "inventory"} {
		t.Run("seed-"+field, func(t *testing.T) {
			in := roadmapInitialized(t)
			r := importRequest(imported("BF-1", "one"))
			switch field {
			case "head":
				in.Head = []byte("{}\n")
			case "queue":
				in.Queue = []byte("{}\n")
			case "ticket":
				in.CanonicalTickets = [][]byte{[]byte("{}\n")}
			case "inventory":
				in.Inventory = nil
			}
			if seq, err := NewImportSequence(r.QueueID, r.Actor, in); err == nil || seq != nil {
				t.Fatal("invalid seed admitted")
			}
		})
	}
}

func TestCALV0018_SequenceBatchGrowth(t *testing.T) {
	t.Run("CAL-V0-018 growing batch counts", testCALV0018BatchGrowth)
}
func testCALV0018BatchGrowth(t *testing.T) {
	for _, batches := range []int{1, 16, 64} {
		t.Run(fmt.Sprint(batches), func(t *testing.T) {
			in := roadmapInitialized(t)
			actor := admin(ImportApply, "x").Actor
			seq, err := NewImportSequence(admin(ImportApply, "x").QueueID, actor, in)
			if err != nil {
				t.Fatal(err)
			}
			baseFiles := len(seq.input.Inventory.files)
			seed := seq.input.Inventory
			for n := 0; n < batches; n++ {
				rec := imported(fmt.Sprintf("BF-%03d", n), "one")
				r := importRequest(rec)
				r.RequestID = fmt.Sprintf("growth-%d", n)
				result, batch := seq.Prepare(r, in)
				if batch == nil {
					t.Fatal(result)
				}
				if err = seq.Advance(batch); err != nil {
					t.Fatal(err)
				}
				if seq.input.Inventory != seed || len(seq.records) != n+1 || len(seed.files) != baseFiles+3*(n+1) {
					t.Fatal("owned inventory growth differs from posted records")
				}
				in.Head = batch.Head()
				in.HeadReceipt = batch.Receipt()
			}
			// A new ordinary Input independently validates the complete final view.
			plain, err := NewInventory(seed.Files(), seed.Directories())
			if err != nil {
				t.Fatal(err)
			}
			in.Inventory = plain
			for _, rec := range seq.records {
				in.CanonicalTickets = append(in.CanonicalTickets, rec.Encode())
			}
			r := importRequest(imported("ZZ-1", "next"))
			r.RequestID = "ordinary-after-growth"
			if full := Model(r, in); full.Plan == nil {
				t.Fatalf("final ordinary model refused: %+v", full)
			}
		})
	}
}

func TestCALV0018_SequenceInventoryBoundaries(t *testing.T) {
	t.Run("CAL-V0-018 incremental inventory boundaries", testCALV0018InventoryBoundaries)
}

// The sequence replaces ticket.NewInventory's whole-store checks with its
// owned indexes; a case-fold collision with a ticket posted by an earlier batch
// and the exact ticket-count limit must refuse with the full model's code and
// close the sequence.
func testCALV0018InventoryBoundaries(t *testing.T) {
	t.Run("fold-cross-batch", func(t *testing.T) {
		in := roadmapInitialized(t)
		r := importRequest(imported("BF-3", "three"))
		seq, err := NewImportSequence(r.QueueID, r.Actor, in)
		if err != nil {
			t.Fatal(err)
		}
		defer seq.Close()
		oracle := Model(r, in)
		_, batch := seq.Prepare(r, in)
		if oracle.Plan == nil || batch == nil || seq.Advance(batch) != nil {
			t.Fatal("first batch refused")
		}
		in = commitModel(t, in, oracle.Plan)
		in.HeadReceipt = oracle.Plan.Receipt()
		in.CanonicalTickets = [][]byte{imported("BF-3", "three").Encode()}
		r = importRequest(imported("bf-3", "three"))
		r.RequestID = "import-2"
		full := Model(r, in)
		fast, batch := seq.Prepare(r, in)
		if batch != nil || full.Kind == "Transaction" || !reflect.DeepEqual(full.Outcome.Codes, fast.Outcome.Codes) || fast.Outcome.Codes[0] != wire.CodeDuplicateID {
			t.Fatalf("cross-batch fold: full=%v fast=%v", full.Outcome.Codes, fast.Outcome.Codes)
		}
		if _, batch = seq.Prepare(importRequest(imported("BF-4", "four")), in); batch != nil {
			t.Fatal("refused cursor reusable")
		}
	})
	t.Run("ticket-limit", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds a store at the ticket-count limit")
		}
		in := importBenchStore(t, wire.MaxTicketsPerQueue-1)
		for _, tc := range []struct {
			records int
			admit   bool
		}{{1, true}, {2, false}} {
			r := importRequest()
			for k := 0; k < tc.records; k++ {
				r.Records = append(r.Records, imported(fmt.Sprintf("NEW-%d", k), "limit").Encode())
			}
			seq, err := NewImportSequence(r.QueueID, r.Actor, in)
			if err != nil {
				t.Fatal(err)
			}
			full := Model(r, in)
			fast, batch := seq.Prepare(r, in)
			seq.Close()
			if (full.Plan != nil) != tc.admit || (batch != nil) != tc.admit || !reflect.DeepEqual(full.Outcome.Codes, fast.Outcome.Codes) {
				t.Fatalf("%d records: full=%v %s fast=%v %s", tc.records, full.Outcome.Codes, full.Detail, fast.Outcome.Codes, fast.Detail)
			}
		}
	})
}
