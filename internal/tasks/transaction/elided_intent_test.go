package transaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// elideIntent returns inv with its intent/tickets/ and intent/releases/
// records summarized as a liveness writer audit summarizes them
// (JSG-V0-001), keeping inv's receipt and request summary.
func elideIntent(t *testing.T, inv *Inventory) (*Inventory, Elided) {
	t.Helper()
	var explicit, elided []archive.FileEntry
	live := map[string]bool{}
	in := &ElidedIntent{}
	for _, f := range inv.explicitFiles() {
		if !intentRecord(f.Path) {
			explicit = append(explicit, f)
			continue
		}
		elided = append(elided, f)
		live[f.Path] = true
		if strings.HasPrefix(f.Path, "intent/tickets/") {
			in.Tickets++
		} else {
			in.Releases++
		}
	}
	cost, err := archive.MeasureFileSet(elided)
	if err != nil {
		t.Fatal(err)
	}
	in.Cost, in.Has = cost, func(p string) bool { return live[p] }
	e := *inv.elided
	e.Intent = in
	out, err := NewSummarizedInventory(explicit, inv.Directories(), e)
	if err != nil {
		t.Fatal(err)
	}
	return out, e
}

// TestJSGV0001_ElidedIntentMeasureParity covers the capacity side of
// JSG-V0-001: an inventory whose ticket and release records are charged
// from the canonical sizes a writer checkpoint carries measures exactly as
// the inventory that lists them, at every receipt prefix, without reading
// their metadata; a question that needs an elided intent record marks the
// inventory incomplete, and a malformed intent summary is refused.
// Mutation: charging the elided records as scanned state entries, or not
// charging their intent bytes, breaks the parity.
func TestJSGV0001_ElidedIntentMeasureParity(t *testing.T) {
	in := paused(t)
	in.Inventory = in.Inventory.clone()
	for i := 1; i <= 3; i++ {
		rec := fixture.Ticket(fmt.Sprintf("T-%d", i))
		p := "intent/tickets/" + rec.TicketID.Local + ".json"
		in.Inventory.files[p] = bytesEntry(p, rec.Encode())
	}
	release := "intent/releases/R-1.json"
	in.Inventory.files[release] = bytesEntry(release, []byte(`{"release":"R-1"}`))
	h, err := snapshot.DecodeHead(in.Head)
	if err != nil {
		t.Fatal(err)
	}
	want, err := measure(in.Inventory, h, 7, 11, 2)
	if err != nil {
		t.Fatal(err)
	}
	for k := uint64(1); k <= h.LastSeq.Uint64(); k++ {
		receiptsOnly := summarize(t, in.Inventory, k)
		base, err := measure(receiptsOnly, h, 7, 11, 2)
		if err != nil || base != want {
			t.Fatalf("k=%d receipt summary: %+v %v, want %+v", k, base, err, want)
		}
		inv, _ := elideIntent(t, receiptsOnly)
		got, err := measure(inv, h, 7, 11, 2)
		if err != nil {
			t.Fatal(k, err)
		}
		if got != want {
			t.Fatalf("k=%d: elided intent measure differs\n got %+v\nwant %+v", k, got, want)
		}
		if inv.Incomplete() {
			t.Fatalf("k=%d: measure needed elided intent metadata", k)
		}
	}

	probes := map[string]func(*Inventory){
		"lookup ticket":        func(i *Inventory) { i.lookup("intent/tickets/T-1.json") },
		"lookup release":       func(i *Inventory) { i.lookup(release) },
		"list tickets":         func(i *Inventory) { i.FilesUnder("intent/tickets/") },
		"list intent":          func(i *Inventory) { i.FilesUnder("intent/") },
		"list everything":      func(i *Inventory) { i.FilesUnder("") },
		"count ticket records": func(i *Inventory) { countPrefix(i, "intent/tickets/") },
	}
	for name, probe := range probes {
		inv, _ := elideIntent(t, summarize(t, in.Inventory, 1))
		if !inv.Has("intent/tickets/T-2.json") || inv.Has("intent/tickets/T-9.json") || inv.Incomplete() {
			t.Fatal(name, "fresh summary")
		}
		inv.lookup("intent/queue.json")
		inv.FilesUnder("intent/queue")
		if inv.Incomplete() {
			t.Fatal(name, "queue lookup touched the elided intent records")
		}
		probe(inv)
		if !inv.Incomplete() {
			t.Fatalf("%s: elided intent metadata was answered", name)
		}
	}

	_, good := elideIntent(t, summarize(t, in.Inventory, 1))
	inv, _ := elideIntent(t, summarize(t, in.Inventory, 1))
	files, dirs := inv.explicitFiles(), inv.Directories()
	ticket := bytesEntry("intent/tickets/T-1.json", fixture.Ticket("T-1").Encode())
	for name, mutate := range map[string]func(*ElidedIntent) []archive.FileEntry{
		"nil lookup":      func(e *ElidedIntent) []archive.FileEntry { e.Has = nil; return files },
		"file count":      func(e *ElidedIntent) []archive.FileEntry { e.Cost.Files++; return files },
		"negative count":  func(e *ElidedIntent) []archive.FileEntry { e.Tickets, e.Cost.Files = -1, e.Cost.Files-4; return files },
		"ticket bound":    func(e *ElidedIntent) []archive.FileEntry { e.Tickets = wire.MaxTicketsPerQueue + 1; return files },
		"intent overflow": func(e *ElidedIntent) []archive.FileEntry { e.Cost.PayloadBytes = wire.MaxIntentTreeBytes; return files },
		"explicit ticket beside the summary": func(e *ElidedIntent) []archive.FileEntry {
			out := append([]archive.FileEntry{ticket}, files...)
			archive.SortFiles(out)
			return out
		},
	} {
		e := good
		intent := *good.Intent
		f := mutate(&intent)
		e.Intent = &intent
		if _, err := NewSummarizedInventory(f, dirs, e); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
