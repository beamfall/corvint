package companionrelease

import (
	"crypto/sha256"
	"strings"
	"testing"
)

// PUB-V0-012 (V1-1529): an incompressible export whose archive would exceed
// the verifier's compressed-archive bound is refused by the export side, and
// an archive within the bound still decodes under that bound.
func TestPUBV0012IncompressibleExportRefusedBeforeVerifierBound(t *testing.T) {
	data := make([]byte, 0, 64<<10)
	block := sha256.Sum256([]byte("v1-1529"))
	for len(data) < 64<<10 {
		data = append(data, block[:]...)
		block = sha256.Sum256(block[:])
	}
	entries := []ArchiveEntry{{Path: "noise.bin", Mode: 0o644, Data: data}}
	previous := archiveSizeLimit
	t.Cleanup(func() { archiveSizeLimit = previous })

	// The limit equals the content size: tar headers and gzip framing on
	// incompressible bytes push the archive past it.
	archiveSizeLimit = int64(len(data))
	if _, err := buildTarGz(entries); err == nil || !strings.Contains(err.Error(), "exceeds the verifier's archive bound") {
		t.Fatalf("an archive over the verifier bound was emitted: %v", err)
	}

	archiveSizeLimit = previous
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(archive)) <= int64(len(data)) {
		t.Fatalf("archive of %d incompressible bytes is only %d bytes; the fixture is compressible", len(data), len(archive))
	}
	decoded, err := decodeTarGz(archive, 1, int64(len(data)))
	if err != nil || len(decoded) != 1 || string(decoded[0].Data) != string(data) {
		t.Fatalf("archive within the bound did not decode: %v", err)
	}
}
