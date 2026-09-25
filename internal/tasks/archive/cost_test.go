package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func costManifest(t *testing.T) *Manifest {
	t.Helper()
	queueID, err := wire.ParseQueueID("/queueId", "queue:corvint:fixture")
	if err != nil {
		t.Fatal(err)
	}
	digest := wire.Digest(strings.Repeat("a", 64))
	return &Manifest{
		QueueID:           queueID,
		ExportedAtSeq:     "9",
		HeadSha256:        digest,
		VersionSha256:     digest,
		PrimaryWorktree:   "/tmp/corvint fixture",
		IntentTreeSha256:  digest,
		ReceiptCount:      "9",
		LastReceiptSha256: digest,
		HeadGeneration:    "10",
		Complete:          true,
	}
}

func encodedArchive(t *testing.T, m *Manifest, bodies map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	if err := writeEntry(tw, ManifestName, m.Encode()); err != nil {
		t.Fatal(err)
	}
	ordered := append([]FileEntry(nil), m.Files...)
	SortFiles(ordered)
	for _, f := range ordered {
		body, ok := bodies[f.Path]
		if !ok || uint64(len(body)) != f.Bytes.Uint64() {
			t.Fatalf("missing or wrongly sized body for %q", f.Path)
		}
		if err := writeEntry(tw, f.Path, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertMeasuredEncoding(t *testing.T, m *Manifest, bodies map[string][]byte) {
	t.Helper()
	got, err := MeasureManifestEncoding(m)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	stream := encodedArchive(t, m, bodies)
	var payload uint64
	for _, body := range bodies {
		payload += uint64(len(body))
	}
	if got.Files != len(m.Files) || got.PayloadBytes != payload || got.ManifestBytes != uint64(len(m.Encode())) || got.TarBytes != uint64(len(stream)) {
		t.Fatalf("measurement=%+v actual={files:%d payload:%d manifest:%d tar:%d}", got, len(m.Files), payload, len(m.Encode()), len(stream))
	}
}

// TestTMV0022_AS22_MeasureManifestEncodingParity proves analytical sizes
// against complete production manifest and tar encodings.
func TestTMV0022_AS22_MeasureManifestEncodingParity(t *testing.T) {
	digest := wire.Digest(strings.Repeat("b", 64))
	for _, size := range []int{0, 1, 511, 512, 513} {
		t.Run(fmt.Sprintf("padding_%d", size), func(t *testing.T) {
			m := costManifest(t)
			m.Files = []FileEntry{{Path: "padding.dat", Sha256: digest, Bytes: wire.SizeOf(uint64(size))}}
			assertMeasuredEncoding(t, m, map[string][]byte{"padding.dat": make([]byte, size)})
		})
	}

	t.Run("empty", func(t *testing.T) {
		assertMeasuredEncoding(t, costManifest(t), map[string][]byte{})
	})
	t.Run("multiple_pax_utf8_and_json_escaping", func(t *testing.T) {
		m := costManifest(t)
		long := strings.Repeat("p", 101)
		m.Files = []FileEntry{
			{Path: `dir/"quoted".json`, Sha256: digest, Bytes: "1"},
			{Path: "café.json", Sha256: digest, Bytes: "2"},
			{Path: long, Sha256: digest, Bytes: "3"},
		}
		bodies := map[string][]byte{
			`dir/"quoted".json`: {1},
			"café.json":         {1, 2},
			long:                {1, 2, 3},
		}
		assertMeasuredEncoding(t, m, bodies)
		headerBytes, err := measureHeaderBytes(long, 3)
		if err != nil || headerBytes <= tarBlockBytes {
			t.Fatalf("PAX header not counted: bytes=%d err=%v", headerBytes, err)
		}
	})
}

// TestTMV0022_AS22_MeasureMetadataWidthAndNullParity covers metadata-only
// byte-width transitions without requiring a verified store inventory.
func TestTMV0022_AS22_MeasureMetadataWidthAndNullParity(t *testing.T) {
	barrier := wire.Digest(strings.Repeat("c", 64))
	for _, tc := range []struct {
		name       string
		seq        wire.Size
		generation wire.Size
		barrier    *wire.Digest
	}{
		{name: "narrow_null", seq: "9", generation: "0"},
		{name: "wider_digest", seq: "10", generation: "18446744073709551615", barrier: &barrier},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := costManifest(t)
			m.ExportedAtSeq = tc.seq
			m.ReceiptCount = tc.seq
			m.HeadGeneration = tc.generation
			m.BarrierSha256 = tc.barrier
			assertMeasuredEncoding(t, m, map[string][]byte{})
		})
	}
}

// TestTMV0022_AS22_MeasureRejectsTypedAliasesAndMembers exercises the same
// closed scalar, path and per-entry bounds before encoding-size use.
func TestTMV0022_AS22_MeasureRejectsTypedAliasesAndMembers(t *testing.T) {
	digest := wire.Digest(strings.Repeat("d", 64))
	validFile := FileEntry{Path: "file", Sha256: digest, Bytes: "0"}
	cases := []struct {
		name string
		code string
		edit func(*Manifest)
	}{
		{"nil_manifest", wire.CodeMalformed, nil},
		{"queue_alias", wire.CodeMalformed, func(m *Manifest) { m.QueueID.Raw = "queue" }},
		{"metadata_size_alias", wire.CodeMalformed, func(m *Manifest) { m.ExportedAtSeq = "01" }},
		{"metadata_digest_alias", wire.CodeMalformed, func(m *Manifest) { m.HeadSha256 = wire.Digest(strings.Repeat("A", 64)) }},
		{"barrier_digest_alias", wire.CodeMalformed, func(m *Manifest) { d := wire.Digest("bad"); m.BarrierSha256 = &d }},
		{"path_text_alias", wire.CodeMalformed, func(m *Manifest) { m.PrimaryWorktree = "relative" }},
		{"incomplete", wire.CodeMalformed, func(m *Manifest) { m.Complete = false }},
		{"sequence_mismatch", wire.CodeJournalForked, func(m *Manifest) { m.ReceiptCount = "8" }},
		{"empty_path", wire.CodeMalformed, func(m *Manifest) { f := validFile; f.Path = ""; m.Files = []FileEntry{f} }},
		{"unclean_path", wire.CodeMalformed, func(m *Manifest) { f := validFile; f.Path = "a//b"; m.Files = []FileEntry{f} }},
		{"identifier_bound", wire.CodeLimitExceeded, func(m *Manifest) { f := validFile; f.Path = strings.Repeat("x", 129); m.Files = []FileEntry{f} }},
		{"digest_alias", wire.CodeMalformed, func(m *Manifest) { f := validFile; f.Sha256 = "bad"; m.Files = []FileEntry{f} }},
		{"size_alias", wire.CodeMalformed, func(m *Manifest) { f := validFile; f.Bytes = "01"; m.Files = []FileEntry{f} }},
		{"int64_overflow", wire.CodeLimitExceeded, func(m *Manifest) { f := validFile; f.Bytes = "18446744073709551615"; m.Files = []FileEntry{f} }},
		{"path_size_bound", wire.CodeLimitExceeded, func(m *Manifest) {
			f := validFile
			f.Bytes = wire.SizeOf(uint64(wire.MaxAttemptRecordBytes + 1))
			m.Files = []FileEntry{f}
		}},
		{"reserved_manifest", wire.CodeDuplicateID, func(m *Manifest) { f := validFile; f.Path = ManifestName; m.Files = []FileEntry{f} }},
		{"duplicate", wire.CodeDuplicateID, func(m *Manifest) { m.Files = []FileEntry{validFile, validFile} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m *Manifest
			if tc.edit != nil {
				m = costManifest(t)
				tc.edit(m)
			}
			if _, err := MeasureManifestEncoding(m); wire.CodeOf(err) != tc.code {
				t.Fatalf("code=%q want=%q err=%v", wire.CodeOf(err), tc.code, err)
			}
		})
	}

	m := costManifest(t)
	m.Files = []FileEntry{{Path: strings.Repeat("x", wire.MaxIdentifierBytes), Sha256: digest, Bytes: "0"}}
	if _, err := MeasureManifestEncoding(m); err != nil {
		t.Fatalf("exact Identifier128 boundary refused: %v", err)
	}
}

// TestTMV0022_AS22_MeasureCapsAndCheckedArithmetic covers count, manifest,
// exact-tar and integer conversion boundaries without giant bodies.
func TestTMV0022_AS22_MeasureCapsAndCheckedArithmetic(t *testing.T) {
	if err := validateManifestFileCount(wire.MaxArchiveFiles); err != nil {
		t.Fatalf("exact count cap: %v", err)
	}
	if err := validateManifestFileCount(wire.MaxArchiveFiles + 1); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("count overflow: %v", err)
	}
	if got, err := addWithin(uint64(MaxManifestBytes)-1, 1, uint64(MaxManifestBytes), ManifestName); err != nil || got != uint64(MaxManifestBytes) {
		t.Fatalf("exact manifest cap: got=%d err=%v", got, err)
	}
	if _, err := addWithin(uint64(MaxManifestBytes)-1, 2, uint64(MaxManifestBytes), ManifestName); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("manifest overflow: %v", err)
	}
	if got, err := addArchiveBytes(wire.MaxArchiveBytes-1, 1); err != nil || got != wire.MaxArchiveBytes {
		t.Fatalf("exact tar cap: got=%d err=%v", got, err)
	}
	if _, err := addArchiveBytes(wire.MaxArchiveBytes, 1); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("tar overflow: %v", err)
	}
	if _, err := roundTarBody(math.MaxUint64); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("padding overflow: %v", err)
	}
	if _, err := measureHeaderBytes("file", uint64(math.MaxInt64)+1); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("header conversion overflow: %v", err)
	}

	m := costManifest(t)
	digest := wire.Digest(strings.Repeat("e", 64))
	m.Files = make([]FileEntry, 1024)
	for i := range m.Files {
		m.Files[i] = FileEntry{
			Path:   "evidence/" + fmt.Sprintf("%064x", i),
			Sha256: digest,
			Bytes:  wire.SizeOf(wire.MaxEvidenceBlobBytes),
		}
	}
	if _, err := MeasureManifestEncoding(m); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("encoded tar over cap accepted: %v", err)
	}
}

// TestTMV0022_AS22_MeasureDoesNotAllocateClaimedBodies checks that a 64 MiB
// claimed member changes arithmetic, not retained allocation by body length.
func TestTMV0022_AS22_MeasureDoesNotAllocateClaimedBodies(t *testing.T) {
	digest := wire.Digest(strings.Repeat("f", 64))
	path := "evidence/" + strings.Repeat("f", 64)
	measure := func(size uint64) int64 {
		m := costManifest(t)
		m.Files = []FileEntry{{Path: path, Sha256: digest, Bytes: wire.SizeOf(size)}}
		result := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := MeasureManifestEncoding(m); err != nil {
					b.Fatal(err)
				}
			}
		})
		return result.AllocedBytesPerOp()
	}
	small := measure(1)
	large := measure(wire.MaxEvidenceBlobBytes)
	if large > small+1024 {
		t.Fatalf("allocation follows claimed body: small=%d large=%d", small, large)
	}
}

// TestTMV0022_AS22_ArchiveHeaderFactoringPreservesBytes compares writeEntry
// with the pre-factoring literal header, including its PAX extension.
func TestTMV0022_AS22_ArchiveHeaderFactoringPreservesBytes(t *testing.T) {
	name := strings.Repeat("n", 101)
	body := []byte("body")
	var got bytes.Buffer
	gotWriter := tar.NewWriter(&got)
	if err := writeEntry(gotWriter, name, body); err != nil {
		t.Fatal(err)
	}
	if err := gotWriter.Close(); err != nil {
		t.Fatal(err)
	}

	var want bytes.Buffer
	wantWriter := tar.NewWriter(&want)
	legacy := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(body)),
		ModTime:  time.Unix(0, 0).UTC(),
		Format:   tar.FormatPAX,
	}
	if err := wantWriter.WriteHeader(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := wantWriter.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := wantWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("factored production header changed archive bytes")
	}
}
