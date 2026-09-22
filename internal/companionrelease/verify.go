package companionrelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const (
	maxRetainedArchiveBytes = 512 << 20
	maxRetainedEntries      = 20_000
)

func decodeTarGz(data []byte, maxEntries int, maxBytes int64) ([]ArchiveEntry, error) {
	if int64(len(data)) > maxRetainedArchiveBytes {
		return nil, fmt.Errorf("archive size %d exceeds bound %d", len(data), maxRetainedArchiveBytes)
	}
	reader := bytes.NewReader(data)
	gr, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	gr.Multistream(false)
	tr := tar.NewReader(gr)
	var entries []ArchiveEntry
	var names []string
	var total int64
	seen := map[string]struct{}{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unsafe archive member type %v at %s", header.Typeflag, header.Name)
		}
		if err := refuseUnsafePath(header.Name); err != nil {
			return nil, fmt.Errorf("archive member: %w", err)
		}
		if header.Mode != 0o644 && header.Mode != 0o755 {
			return nil, fmt.Errorf("archive member %s has unsupported mode %o", header.Name, header.Mode)
		}
		if header.Size < 0 || header.Size > maxBytes-total {
			return nil, fmt.Errorf("archive contents exceed %d byte bound", maxBytes)
		}
		if len(entries) >= maxEntries {
			return nil, fmt.Errorf("archive entries exceed bound %d", maxEntries)
		}
		if _, ok := seen[header.Name]; ok {
			return nil, fmt.Errorf("duplicate archive member %s", header.Name)
		}
		body := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, body); err != nil {
			return nil, fmt.Errorf("archive member %s body: %w", header.Name, err)
		}
		seen[header.Name] = struct{}{}
		names = append(names, header.Name)
		entries = append(entries, ArchiveEntry{Path: header.Name, Mode: header.Mode, Data: body})
		total += header.Size
	}
	if err := refuseCaseFoldCollisions(names); err != nil {
		return nil, fmt.Errorf("archive member: %w", err)
	}
	trailing, err := io.Copy(io.Discard, gr)
	if err != nil {
		return nil, fmt.Errorf("gzip trailing data: %w", err)
	}
	if trailing != 0 {
		return nil, fmt.Errorf("archive has %d bytes after the tar terminator", trailing)
	}
	if err := gr.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	if reader.Len() != 0 {
		return nil, fmt.Errorf("archive has %d trailing bytes after the gzip member", reader.Len())
	}
	return entries, nil
}

// verifyTarGz re-decodes data with a fresh gzip.Reader and tar.Reader — the
// same reads any consumer of the shipped archive would perform, never the
// in-memory entries buildTarGz was given — and requires the closed member
// inventory (exact path set, exact modes, exact bytes) and a clean end of
// stream: no trailing tar padding beyond the terminator, and no bytes after
// the single gzip member.
func verifyTarGz(data []byte, expected []ArchiveEntry) error {
	want := make(map[string]ArchiveEntry, len(expected))
	for _, e := range expected {
		if _, dup := want[e.Path]; dup {
			return fmt.Errorf("expected inventory has duplicate path %s", e.Path)
		}
		want[e.Path] = e
	}

	actual, err := decodeTarGz(data, maxRetainedEntries, maxRetainedArchiveBytes)
	if err != nil {
		return err
	}
	found := make(map[string]struct{}, len(actual))
	for _, entry := range actual {
		wantEntry, ok := want[entry.Path]
		if !ok {
			return fmt.Errorf("unexpected archive member %s", entry.Path)
		}
		if entry.Mode != wantEntry.Mode {
			return fmt.Errorf("archive member %s mode %o != expected %o", entry.Path, entry.Mode, wantEntry.Mode)
		}
		if len(entry.Data) != len(wantEntry.Data) {
			return fmt.Errorf("archive member %s size %d != expected %d", entry.Path, len(entry.Data), len(wantEntry.Data))
		}
		if !bytes.Equal(entry.Data, wantEntry.Data) {
			return fmt.Errorf("archive member %s content mismatch", entry.Path)
		}
		found[entry.Path] = struct{}{}
	}
	if len(found) != len(want) {
		return fmt.Errorf("archive inventory has %d members, expected %d", len(found), len(want))
	}
	return nil
}
