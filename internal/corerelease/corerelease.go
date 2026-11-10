// Package corerelease holds the Core release helpers that outlived the retired
// companion bundle (decision 0594): the clean-checkout Corvint source archive a
// Core-only release candidate carries (PRS-V1-005), and the cem/1.0 stable
// release-readiness declaration, result and gate-outcome readers. Every read
// it makes is either a Git object read against an explicit HEAD, or a bounded
// subprocess run in a scratch directory the caller owns. It never tags, pushes,
// or publishes anything.
package corerelease

import (
	"encoding/json"
	"fmt"
	"io"
)

// stableResultByteLimit bounds one cem/1.0 stable result document.
const stableResultByteLimit = 8 << 20

// sourceArchiveEntries converts an Export's hash-verified files into
// ArchiveEntry values for the deterministic tar.gz assembler.
func sourceArchiveEntries(export Export) []ArchiveEntry {
	entries := make([]ArchiveEntry, 0, len(export.Files))
	for _, f := range export.Files {
		mode := int64(0o644)
		if f.Mode == "100755" {
			mode = 0o755
		}
		entries = append(entries, ArchiveEntry{Path: f.Path, Mode: mode, Data: f.Data})
	}
	return entries
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}
