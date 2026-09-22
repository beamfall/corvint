package companionrelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// archiveEpoch is the fixed modification time every archive member carries.
// A deterministic archive cannot depend on wall-clock build time: two
// assemblies of the same inputs must produce identical bytes.
var archiveEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// ArchiveEntry is one file this package will place in a tar.gz it builds.
// Mode must be exactly 0o644 or 0o755; there is no other member kind.
type ArchiveEntry struct {
	Path string
	Mode int64
	Data []byte
}

// buildTarGz assembles entries into a deterministic gzip-compressed tar: a
// stable member order, a fixed mtime, no uid/gid/uname/gname, and gzip's
// mtime/OS/extra-flag header fields zeroed. Calling it twice on an
// unchanged entry set byte-for-byte reproduces the previous archive.
func buildTarGz(entries []ArchiveEntry) ([]byte, error) {
	sorted := append([]ArchiveEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	seen := make(map[string]struct{}, len(sorted))
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, e := range sorted {
		if e.Mode != 0o644 && e.Mode != 0o755 {
			return nil, fmt.Errorf("archive entry %s has unsupported mode %o", e.Path, e.Mode)
		}
		if _, dup := seen[e.Path]; dup {
			return nil, fmt.Errorf("duplicate archive entry %s", e.Path)
		}
		seen[e.Path] = struct{}{}
		header := &tar.Header{
			Name:     e.Path,
			Typeflag: tar.TypeReg,
			Mode:     e.Mode,
			Size:     int64(len(e.Data)),
			ModTime:  archiveEpoch,
			Format:   tar.FormatPAX,
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write header %s: %w", e.Path, err)
		}
		if _, err := tw.Write(e.Data); err != nil {
			return nil, fmt.Errorf("write body %s: %w", e.Path, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	var gzBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gw.Name = ""
	gw.Comment = ""
	gw.ModTime = time.Time{}
	gw.OS = 255 // "unknown", so the field never leaks the building host's OS.
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return gzBuf.Bytes(), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// buildTarGzTwice calls build twice — each call independently reconstructs
// the entry set, mirroring how buildComponentTwice reruns go build from
// cold separate caches rather than reusing one build's output — and
// requires byte-identical tar.gz output before accepting the archive. A
// build that returns a different entry set (or otherwise behaves
// non-deterministically) on its second call is refused, not tolerated.
func buildTarGzTwice(build func() ([]ArchiveEntry, error)) ([]byte, error) {
	firstEntries, err := build()
	if err != nil {
		return nil, err
	}
	first, err := buildTarGz(firstEntries)
	if err != nil {
		return nil, err
	}
	secondEntries, err := build()
	if err != nil {
		return nil, err
	}
	second, err := buildTarGz(secondEntries)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(first, second) {
		return nil, fmt.Errorf("two independently built archive assemblies disagree (%d entries then %d entries)", len(firstEntries), len(secondEntries))
	}
	return first, nil
}
