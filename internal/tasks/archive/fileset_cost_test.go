package archive

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0117_FileSetCostParity proves that charging an elided, disjoint file
// set by its additive cost measures exactly what the complete manifest
// measures, for every split point, and that the caps still refuse.
func TestCALV0117_FileSetCostParity(t *testing.T) {
	digest := wire.Digest(strings.Repeat("c", 64))
	var all []FileEntry
	for i := 1; i <= 40; i++ {
		all = append(all, FileEntry{Path: fmt.Sprintf("receipts/%012d.json", i), Sha256: digest, Bytes: wire.SizeOf(uint64(300 + i*37))})
	}
	all = append(all,
		FileEntry{Path: "requests/ab/" + strings.Repeat("r", 101) + ".json", Sha256: digest, Bytes: "513"},
		FileEntry{Path: "evidence/" + strings.Repeat("d", 64), Sha256: digest, Bytes: "0"},
		FileEntry{Path: "head.json", Sha256: digest, Bytes: "512"},
	)
	whole := costManifest(t)
	whole.Files = all
	want, err := MeasureManifestEncoding(whole)
	if err != nil {
		t.Fatal(err)
	}
	for split := 0; split <= len(all); split++ {
		prefix, err := MeasureFileSet(all[:split])
		if err != nil {
			t.Fatalf("split %d: %v", split, err)
		}
		var summed FileSetCost
		for _, f := range all[:split] {
			one, err := EntryCost(f.Path, f.Bytes.Uint64())
			if err != nil {
				t.Fatal(err)
			}
			if summed, err = summed.Add(one); err != nil {
				t.Fatal(err)
			}
		}
		if summed != prefix {
			t.Fatalf("split %d: summed %+v != measured %+v", split, summed, prefix)
		}
		m := costManifest(t)
		m.Files = append([]FileEntry(nil), all[split:]...)
		got, err := MeasureManifestEncodingWith(m, prefix)
		if err != nil {
			t.Fatalf("split %d: %v", split, err)
		}
		if got != want {
			t.Fatalf("split %d: with-prefix %+v != whole %+v", split, got, want)
		}
	}

	if _, err := EntryCost("evidence/"+strings.Repeat("e", 64), wire.MaxEvidenceBlobBytes+1); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("over-bound elided entry accepted: %v", err)
	}
	big := make([]FileEntry, 1024)
	for i := range big {
		big[i] = FileEntry{Path: "evidence/" + fmt.Sprintf("%064x", i), Sha256: digest, Bytes: wire.SizeOf(wire.MaxEvidenceBlobBytes)}
	}
	prefix, err := MeasureFileSet(big)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MeasureManifestEncodingWith(costManifest(t), prefix); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("elided set over the tar cap accepted: %v", err)
	}
}
