package transaction

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestCALV0018_SequenceCapacityPrefixes(t *testing.T) {
	t.Run("CAL-V0-018 exact capacity prefixes", testCALV0018CapacityPrefixes)
}
func testCALV0018CapacityPrefixes(t *testing.T) {
	for _, barrier := range []bool{false, true} {
		t.Run(fmt.Sprint(barrier), func(t *testing.T) {
			in := roadmapInitialized(t)
			if barrier {
				p := Model(admin(Pause, "pause"), in)
				if p.Plan == nil {
					t.Fatal(p)
				}
				in = commitModel(t, in, p.Plan)
				in.HeadReceipt = p.Plan.Receipt()
			}
			seq, err := NewImportSequence(admin(ImportApply, "x").QueueID, admin(ImportApply, "x").Actor, in)
			if err != nil {
				t.Fatal(err)
			}
			defer seq.Close()
			for n := 0; n < 12; n++ {
				size := []int{1, 511, 512, 513, 7000, 1}[n%6]
				rec := imported("BF-1", "body")
				body := strings.Repeat("x", size)
				rec.Body = &body
				if n > 0 {
					previous, _ := seq.records[rec.TicketID.Raw]
					hash := previous.FileDigest()
					rec.PreviousRecordSha256 = &hash
					rec.Revision = wire.CountOf(int64(n + 1))
					rec.AcceptanceRevision = rec.Revision
				}
				request := importRequest(rec)
				request.RequestID = fmt.Sprintf("capacity-%d", n)
				oracle := Model(request, in)
				if oracle.Plan == nil {
					t.Fatalf("oracle %d: %+v", n, oracle)
				}
				full, err := CheckCapacity(oracle.Plan)
				if err != nil {
					t.Fatal(err)
				}
				result, batch := seq.Prepare(request, in)
				if batch == nil {
					t.Fatal(result)
				}
				if !reflect.DeepEqual(full.Prefixes, batch.Prefixes()) {
					t.Fatalf("prefix mismatch barrier=%v batch=%d", barrier, n)
				}
				// Replay every exact maximum and one below it through both capacity paths.
				for _, field := range []string{"files", "scan", "manifest", "tar", "journal-count", "journal-bytes", "evidence", "intent"} {
					maximum := uint64(0)
					value := func(c Cost) uint64 {
						switch field {
						case "files":
							return c.Files
						case "scan":
							return c.ScannedEntries
						case "manifest":
							return c.ManifestBytes
						case "tar":
							return c.TarBytes
						case "journal-count":
							return c.JournalCount
						case "journal-bytes":
							return c.JournalBytes
						case "evidence":
							return c.EvidenceBytes
						default:
							return c.IntentBytes
						}
					}
					for _, p := range full.Prefixes {
						if v := value(p.Cost); v > maximum {
							maximum = v
						}
						if p.AbortToUnpause != nil {
							if v := value(*p.AbortToUnpause); v > maximum {
								maximum = v
							}
						}
					}
					for _, cap := range []uint64{maximum, maximum - 1} {
						if maximum == 0 {
							continue
						}
						limits := hardLimits()
						switch field {
						case "files":
							limits.files = cap
						case "scan":
							limits.scan = cap
						case "manifest":
							limits.manifest = cap
						case "tar":
							limits.tar = cap
						case "journal-count":
							limits.journalCount = cap
						case "journal-bytes":
							limits.journalBytes = cap
						case "evidence":
							limits.evidence = cap
						case "intent":
							limits.intent = cap
						}
						want, we := checkCapacity(oracle.Plan, limits)
						copy := *oracle.Plan
						copy.base = seq.input.Inventory
						got, ge := checkCapacity(&copy, limits)
						if wire.CodeOf(we) != wire.CodeOf(ge) || (we == nil) != (ge == nil) || !reflect.DeepEqual(want.Prefixes, got.Prefixes) {
							t.Fatalf("limit parity %s/%d: %v / %v", field, cap, we, ge)
						}
					}
				}
				if err = seq.Advance(batch); err != nil {
					t.Fatal(err)
				}
				in = commitModel(t, in, oracle.Plan)
				in.HeadReceipt = oracle.Plan.Receipt()
				in.CanonicalTickets = [][]byte{rec.Encode()}
			}
		})
	}
	// Overflow must refuse atomically; a failed contribution cannot poison totals.
	cost := importCost{}
	cost.encoding.PayloadBytes = math.MaxUint64
	before := cost
	err := cost.replace(archive.FileEntry{}, false, archive.FileEntry{Path: "VERSION", Sha256: wire.Sum(nil), Bytes: "1"})
	if err == nil || cost != before {
		t.Fatal("overflow did not preserve counters")
	}
}

func TestCALV0018_ReserveProbeOwnership(t *testing.T) {
	t.Run("CAL-V0-018 reserve probe ownership", testCALV0018ReserveOwnership)
}
func testCALV0018ReserveOwnership(t *testing.T) {
	in := roadmapInitialized(t)
	pause := Model(admin(Pause, "pause"), in)
	in = commitModel(t, in, pause.Plan)
	for shard := 0; shard < 256; shard++ {
		in.Inventory.dirs[fmt.Sprintf("requests/%02x", shard)] = true
	}
	for n := 1; n <= 128; n++ {
		path := fmt.Sprintf("requests/00/%064x.json", n)
		in.Inventory.files[path] = archive.FileEntry{Path: path, Sha256: wire.Sum(nil), Bytes: "1"}
	}
	request := importRequest(imported("BF-1", "one"))
	full := Model(request, in)
	if full.Plan == nil {
		t.Fatal(full)
	}
	want, err := CheckCapacity(full.Plan)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := NewImportSequence(request.QueueID, request.Actor, in)
	if err != nil {
		t.Fatal(err)
	}
	before := seq.input.Inventory.Files()
	hints := seq.input.Inventory.incremental.reserveNext
	copy := *full.Plan
	copy.base = seq.input.Inventory
	for n := 0; n < 3; n++ {
		got, err := CheckCapacity(&copy)
		if err != nil || !reflect.DeepEqual(got.Prefixes, want.Prefixes) {
			t.Fatal("reserve cost mismatch", err)
		}
		if hints != seq.input.Inventory.incremental.reserveNext || !reflect.DeepEqual(before, seq.input.Inventory.Files()) {
			t.Fatal("reserve simulation consumed owned state")
		}
	}
	path := "requests/00/" + strings.Repeat("0", 64) + ".json"
	if err = seq.input.Inventory.put(archive.FileEntry{Path: path, Sha256: wire.Sum(nil), Bytes: "1"}, true); err != nil {
		t.Fatal(err)
	}
	if seq.input.Inventory.incremental.reserveNext[0] != 129 {
		t.Fatal("real insertion did not advance contiguous probe prefix")
	}
}

func TestCALV0018_SequenceMixedBatches(t *testing.T) {
	t.Run("CAL-V0-018 mixed batch prefixes", testCALV0018MixedBatches)
}

// Each batch mixes additions with growing and shrinking replacements across
// archive block boundaries; every prefix, the receipt and the final metadata
// must equal the full recomputation.
func testCALV0018MixedBatches(t *testing.T) {
	plan := [][][2]int{ // {ticket, body bytes}
		{{1, 1}, {2, 7000}, {3, 511}},
		{{1, 7000}, {2, 1}, {4, 512}},
		{{1, 511}, {3, 513}, {4, 1}, {5, 7000}},
		{{1, 1}, {2, 513}, {3, 7000}, {4, 512}, {5, 1}, {6, 511}},
	}
	for _, barrier := range []bool{false, true} {
		t.Run(fmt.Sprint(barrier), func(t *testing.T) {
			in := roadmapInitialized(t)
			if barrier {
				p := Model(admin(Pause, "pause"), in)
				in = commitModel(t, in, p.Plan)
				in.HeadReceipt = p.Plan.Receipt()
			}
			seq, err := NewImportSequence(admin(ImportApply, "x").QueueID, admin(ImportApply, "x").Actor, in)
			if err != nil {
				t.Fatal(err)
			}
			defer seq.Close()
			current := map[string]*ticket.Record{}
			for n, batchPlan := range plan {
				request := importRequest()
				request.RequestID = fmt.Sprintf("mixed-%d", n)
				next := map[string]*ticket.Record{}
				for _, item := range batchPlan {
					local := fmt.Sprintf("BF-%d", item[0])
					rec := imported(local, "body")
					body := strings.Repeat("y", item[1])
					rec.Body = &body
					if prev := current[local]; prev != nil {
						hash := prev.FileDigest()
						rec.PreviousRecordSha256 = &hash
						rec.Revision = wire.CountOf(prev.Revision.Int() + 1)
						rec.AcceptanceRevision = rec.Revision
					}
					request.Records = append(request.Records, rec.Encode())
					next[local] = rec
				}
				oracle := Model(request, in)
				if oracle.Plan == nil {
					t.Fatalf("oracle %d: %+v", n, oracle)
				}
				full, err := CheckCapacity(oracle.Plan)
				if err != nil {
					t.Fatal(err)
				}
				result, batch := seq.Prepare(request, in)
				if batch == nil {
					t.Fatalf("sequence %d: %+v", n, result)
				}
				if !reflect.DeepEqual(full.Prefixes, batch.Prefixes()) || !bytes.Equal(batch.Receipt(), oracle.Plan.Receipt()) || !bytes.Equal(batch.Head(), oracle.Plan.Head()) {
					t.Fatalf("batch %d differs from full model", n)
				}
				if err = seq.Advance(batch); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(seq.input.Inventory.Files(), full.Final.Files()) || !reflect.DeepEqual(seq.input.Inventory.Directories(), full.Final.Directories()) {
					t.Fatalf("batch %d final metadata differs", n)
				}
				in = commitModel(t, in, oracle.Plan)
				in.HeadReceipt = oracle.Plan.Receipt()
				for local, rec := range next {
					current[local] = rec
				}
				in.CanonicalTickets = nil
				for _, rec := range current {
					in.CanonicalTickets = append(in.CanonicalTickets, rec.Encode())
				}
			}
		})
	}
}
