package transaction

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// summarize returns inv with receipts 1..k and every request index file
// elided, as a writer checkpoint at seq k would summarize them.
func summarize(t *testing.T, inv *Inventory, k uint64) *Inventory {
	t.Helper()
	var explicit, elided []archive.FileEntry
	requests := map[string]bool{}
	e := Elided{Receipts: k}
	for _, f := range inv.Files() {
		seq, receipt := receiptNumber(f.Path)
		switch {
		case receipt && seq <= k:
			elided = append(elided, f)
			e.ReceiptBytes += f.Bytes.Uint64()
			if seq == 1 {
				e.FirstReceiptSha256 = f.Sha256
			}
			if seq == k {
				e.LastReceiptSha256 = f.Sha256
			}
		case strings.HasPrefix(f.Path, "requests/"):
			elided = append(elided, f)
			requests[f.Path] = true
		default:
			explicit = append(explicit, f)
		}
	}
	e.Requests = uint64(len(requests))
	e.HasRequest = func(p string) bool { return requests[p] }
	cost, err := archive.MeasureFileSet(elided)
	if err != nil {
		t.Fatal(err)
	}
	e.Cost = cost
	out, err := NewSummarizedInventory(explicit, inv.Directories(), e)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCALV0117_SummarizedInventoryCostParity(t *testing.T) {
	in := paused(t)
	for x, op := range []string{Unpause, Pause, Unpause} {
		r := Model(admin(op, fmt.Sprintf("parity-%d", x)), in)
		if r.Kind != "Transaction" {
			t.Fatal(op, r)
		}
		in = commitModel(t, in, r.Plan)
	}
	h, e := snapshot.DecodeHead(in.Head)
	if e != nil {
		t.Fatal(e)
	}
	last := h.LastSeq.Uint64()
	for _, op := range []string{Pause, Unpause} {
		next := in
		if op == Unpause {
			r := Model(admin(Pause, "parity-barrier"), in)
			if r.Kind != "Transaction" {
				t.Fatal(r)
			}
			next = commitModel(t, in, r.Plan)
			last++
		}
		full := Model(admin(op, "parity-next"), next)
		if full.Kind != "Transaction" {
			t.Fatal(full)
		}
		fullCost, e := CheckCapacity(full.Plan)
		if e != nil {
			t.Fatal(e)
		}
		for k := uint64(1); k <= last; k++ {
			summarized := next
			summarized.Inventory = summarize(t, next.Inventory, k)
			got := Model(admin(op, "parity-next"), summarized)
			if got.Kind != "Transaction" || summarized.Inventory.Incomplete() {
				t.Fatalf("%s k=%d: %+v incomplete=%v", op, k, got, summarized.Inventory.Incomplete())
			}
			if !bytes.Equal(got.Plan.descriptor, full.Plan.descriptor) || !bytes.Equal(got.Plan.head, full.Plan.head) {
				t.Fatalf("%s k=%d: plan differs from the complete inventory's", op, k)
			}
			cost, e := CheckCapacity(got.Plan)
			if e != nil {
				t.Fatal(op, k, e)
			}
			if !reflect.DeepEqual(cost.Prefixes, fullCost.Prefixes) {
				t.Fatalf("%s k=%d: summarized capacity differs\n got %+v\nwant %+v", op, k, cost.Prefixes, fullCost.Prefixes)
			}
			if summarized.Inventory.Incomplete() {
				t.Fatalf("%s k=%d: capacity needed elided metadata", op, k)
			}
		}
	}
}

func TestCALV0117_SummarizedInventoryElidedMetadataMarksIncomplete(t *testing.T) {
	in := paused(t)
	h, _ := snapshot.DecodeHead(in.Head)
	first, _ := snapshot.ReceiptName(1)
	var request string
	for _, f := range in.Inventory.Files() {
		if strings.HasPrefix(f.Path, "requests/") {
			request = f.Path
		}
	}
	probes := map[string]func(*Inventory){
		"Files":             func(i *Inventory) { i.Files() },
		"FilesUnder":        func(i *Inventory) { i.FilesUnder("requests/") },
		"matches receipt":   func(i *Inventory) { i.matches("receipts/"+first, nil) },
		"lookup request":    func(i *Inventory) { i.lookup(request) },
		"countPrefix":       func(i *Inventory) { countPrefix(i, "receipts/") },
		"clone shares miss": func(i *Inventory) { i.clone().lookup(request) },
	}
	for name, probe := range probes {
		inv := summarize(t, in.Inventory, 1)
		if inv.Incomplete() || !inv.Has("receipts/"+first) || !inv.Has(request) {
			t.Fatal(name, "fresh summary")
		}
		if e := inv.chain(h); e != nil || inv.Incomplete() {
			t.Fatal(name, e)
		}
		inv.FilesUnder("intent/")
		if inv.Incomplete() {
			t.Fatal(name, "intent listing touched the elided prefix")
		}
		probe(inv)
		if !inv.Incomplete() {
			t.Fatal(name, "elided metadata was answered")
		}
	}
	// A complete inventory never reports a miss.
	in.Inventory.Files()
	if in.Inventory.Incomplete() || in.Inventory.Summarized() {
		t.Fatal("complete inventory")
	}
	// A summary bound to another chain is refused by the head binding.
	inv := summarize(t, in.Inventory, h.LastSeq.Uint64())
	inv.elided.LastReceiptSha256 = wire.Sum([]byte("other"))
	if e := inv.chain(h); e == nil {
		t.Fatal("stale chain head accepted")
	}
	inv = summarize(t, in.Inventory, 1)
	inv.elided.FirstReceiptSha256 = wire.Sum([]byte("other"))
	if e := inv.chain(h); e == nil {
		t.Fatal("foreign genesis accepted")
	}
}

func TestCALV0117_NewSummarizedInventoryRefusesMalformedSummary(t *testing.T) {
	in := paused(t)
	base := summarize(t, in.Inventory, 1)
	files, dirs := base.explicitFiles(), base.Directories()
	first, _ := snapshot.ReceiptName(1)
	full, _ := in.Inventory.lookup("receipts/" + first)
	cases := map[string]func(*Elided) ([]archive.FileEntry, []string){
		"no receipts": func(e *Elided) ([]archive.FileEntry, []string) { e.Receipts = 0; return files, dirs },
		"cost count": func(e *Elided) ([]archive.FileEntry, []string) {
			e.Cost.Files++
			return files, dirs
		},
		"receipt bytes": func(e *Elided) ([]archive.FileEntry, []string) {
			e.ReceiptBytes = e.Cost.PayloadBytes + 1
			return files, dirs
		},
		"nil request set": func(e *Elided) ([]archive.FileEntry, []string) { e.HasRequest = nil; return files, dirs },
		"explicit elided receipt": func(e *Elided) ([]archive.FileEntry, []string) {
			out := append([]archive.FileEntry{full}, files...)
			archive.SortFiles(out)
			return out, dirs
		},
		"explicit elided request": func(e *Elided) ([]archive.FileEntry, []string) {
			e.HasRequest = func(string) bool { return true }
			for _, f := range in.Inventory.Files() {
				if strings.HasPrefix(f.Path, "requests/") {
					e.Requests--
					e.Cost.Files--
					return append(append([]archive.FileEntry{}, files...), f), dirs
				}
			}
			t.Fatal("fixture has no request")
			return nil, nil
		},
		"missing receipts directory": func(e *Elided) ([]archive.FileEntry, []string) {
			var out []string
			for _, d := range dirs {
				if d != "receipts" {
					out = append(out, d)
				}
			}
			return files, out
		},
	}
	for name, mutate := range cases {
		e := *base.elided
		f, d := mutate(&e)
		if _, err := NewSummarizedInventory(f, d, e); err == nil {
			t.Fatal(name, "accepted")
		}
	}
	if _, err := NewSummarizedInventory(files, dirs, *base.elided); err != nil {
		t.Fatal(err)
	}
}
