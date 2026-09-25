package archive

import (
	"archive/tar"
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTMV0022_AS10_EncodedStreamLimit(t *testing.T) {
	_, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	stream, res, err := export(t, repo, staging)
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes >= uint64(len(stream)) {
		t.Fatal("fixture has no framing overhead")
	}
	for _, limit := range []uint64{res.Bytes, uint64(len(stream) - 1), uint64(len(stream))} {
		_, err := verifyLimit(bytes.NewReader(stream), limit)
		if limit < uint64(len(stream)) && wire.CodeOf(err) != wire.CodeLimitExceeded {
			t.Fatalf("verify cap=%d stream=%d: %v", limit, len(stream), err)
		}
		if limit == uint64(len(stream)) && err != nil {
			t.Fatalf("exact boundary refused: %v", err)
		}
		var out bytes.Buffer
		_, err = Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out, streamLimit: limit})
		if limit < uint64(len(stream)) && (wire.CodeOf(err) != wire.CodeLimitExceeded || out.Len() != 0) {
			t.Fatalf("export cap=%d wrote=%d: %v", limit, out.Len(), err)
		}
		if limit == uint64(len(stream)) && (err != nil || !bytes.Equal(out.Bytes(), stream)) {
			t.Fatalf("exact export boundary: %v", err)
		}
		if entries, _ := os.ReadDir(staging); len(entries) != 0 {
			t.Fatal("staging leaked")
		}
	}
}
func TestTMV0022_AS10_EncodedLimitIncludesPAX(t *testing.T) {
	// An extended PAX header is invisible in files[].bytes but consumes budget.
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	if err := tw.WriteHeader(&tar.Header{Name: ManifestName, Typeflag: tar.TypeReg, Size: 2, Mode: 0644, ModTime: time.Unix(0, 1), Format: tar.FormatPAX}); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("{}"))
	tw.Close()
	if raw.Len() <= 2048 {
		t.Fatal("PAX witness absent")
	}
	if _, err := verifyLimit(bytes.NewReader(raw.Bytes()), 512); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("PAX cap: %v", err)
	}
	var staged bytes.Buffer
	bounded := &streamWriter{w: &staged, limit: 512}
	tw = tar.NewWriter(bounded)
	err := tw.WriteHeader(&tar.Header{Name: ManifestName, Typeflag: tar.TypeReg, Size: 2, Mode: 0644, ModTime: time.Unix(0, 1), Format: tar.FormatPAX})
	if err == nil || !bounded.exceeded || staged.Len() > 512 {
		t.Fatalf("PAX staging cap: n=%d err=%v", staged.Len(), err)
	}
	// A budget never changes the meaning of a final source I/O error.
	r := &streamReader{r: terminalFailure{bytes.NewReader(nil)}, limit: 1}
	if _, err = io.ReadAll(r); err != syscall.EIO {
		t.Fatalf("terminal error lost: %v", err)
	}
}

// errorAt returns the complete requested bytes together with the source error
// at one boundary, then continues normally (including an eventual real EOF).
type errorAt struct {
	data       []byte
	offset, at int
	errorBytes int
	err        error
	hit        bool
}

func (r *errorAt) Read(p []byte) (int, error) {
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	end := len(r.data)
	if !r.hit {
		end = r.at
	}
	n := copy(p, r.data[r.offset:end])
	r.offset += n
	if !r.hit && r.offset == r.at {
		r.hit = true
		r.errorBytes = n
		return n, r.err
	}
	return n, nil
}
func TestTMV0022_AS09_BytesAndSourceError(t *testing.T) {
	_, repo := repoWithStore(t)
	stream, exported, err := export(t, repo, fixture.TempDirOutside(t))
	if err != nil {
		t.Fatal(err)
	}
	source := bytes.NewReader(stream)
	tr := tar.NewReader(source)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	// The manifest is retained with readExact/io.ReadFull, which can discard
	// an error returned with the complete body just as tar discards trailer EIO.
	bodyEnd := len(stream) - source.Len() + int(hdr.Size)
	for _, boundary := range []struct {
		name string
		at   int
	}{
		{"footer", len(stream)}, {"complete_manifest_body", bodyEnd},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			r := &errorAt{data: stream, at: boundary.at, err: syscall.EIO}
			res, err := verifyLimit(r, uint64(len(stream)))
			if boundary.name == "footer" && r.errorBytes != 512 {
				t.Fatalf("footer witness returned %d error bytes, want 512", r.errorBytes)
			}
			if !r.hit || res != nil || err == nil || !strings.Contains(err.Error(), syscall.EIO.Error()) {
				t.Fatalf("bytes+EIO accepted/lost: hit=%v result=%v err=%v", r.hit, res != nil, err)
			}
		})
	}
	r := &errorAt{data: stream, at: len(stream), err: io.EOF}
	res, err := verifyLimit(r, uint64(len(stream)))
	if err != nil || res.Bytes != exported.Bytes || !r.hit {
		t.Fatalf("bytes+EOF/metrics: %v", err)
	}
}
