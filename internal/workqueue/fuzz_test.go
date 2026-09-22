package workqueue

import (
	"bytes"
	"testing"
)

// reencoders parse one closed work-queue document kind and return the
// canonical bytes of what was parsed. The fuzz input's first byte picks one.
var reencoders = []func([]byte) ([]byte, error){
	func(raw []byte) ([]byte, error) {
		document, err := ParseSnapshot(raw)
		return canonicalOf(document, err)
	},
	func(raw []byte) ([]byte, error) {
		document, err := ParseEnvelope(raw)
		return canonicalOf(document, err)
	},
	func(raw []byte) ([]byte, error) { document, err := ParsePolicy(raw); return canonicalOf(document, err) },
	func(raw []byte) ([]byte, error) {
		document, err := ParseCheckpoint(raw)
		return canonicalOf(document, err)
	},
	func(raw []byte) ([]byte, error) {
		document, err := ParseDetails(raw)
		return canonicalOf(document, err)
	},
}

func canonicalOf[T interface{ Canonical() []byte }](document T, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return document.Canonical(), nil
}

// FuzzParsedDocumentsReencodeByteExact feeds adapter- and repository-supplied
// bytes to the five closed document parsers. A parser must refuse rather than
// panic, and because every parser admits only canonical bytes, an accepted
// document's canonical encoding must be exactly the bytes it was parsed from:
// a difference is a member the parser dropped, defaulted, or rewrote.
func FuzzParsedDocumentsReencodeByteExact(f *testing.F) {
	ticket := testTicket("one", 7)
	ticket.TouchPaths = []string{"internal/a.go", "internal/dir/"}
	ticket.CapacityUses = []CapacityUse{{ClassID: "capacity:corvint:worklist:cpu", Units: 2}}
	RefreshTicket(&ticket)
	populated := testSnapshot(ticket)
	populated.CapacityClasses = []CapacityClass{{ID: "capacity:corvint:worklist:cpu", AvailableUnits: 4}}
	refreshSnapshotID(populated)
	seeds := [][]byte{populated.Canonical(), testEnvelope(populated).Canonical(), repairPolicy().Canonical(), repairCheckpoint().Canonical(), repairDetails().Canonical()}
	for kind, seed := range seeds {
		f.Add(append([]byte{byte(kind)}, seed...))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		raw := data[1:]
		encoded, err := reencoders[int(data[0])%len(reencoders)](raw)
		if err != nil {
			return
		}
		if !bytes.Equal(encoded, raw) {
			t.Fatalf("parser %d accepted\n%q\nbut re-encodes it as\n%q", int(data[0])%len(reencoders), raw, encoded)
		}
	})
}
