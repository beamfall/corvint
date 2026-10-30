package gitauth

import (
	"bytes"
	"context"
)

// NonRegularProfile names the additive non-regular entry profile of
// CAX-V0-020. Under it a changed path whose base or target tree entry is not
// a regular file (mode 100644 or 100755) is excluded from the profile patch
// and reported as an explicit per-path unknown; the entry is never read as
// file content and a symbolic link is never followed.
const NonRegularProfile = "cem-non-regular/1"

// NonRegularEntry is one changed path excluded by NonRegularProfile. A side
// absent from its tree is "absent", as in UnsupportedModeChanges.
type NonRegularEntry struct {
	Path       string `json:"path"`
	BaseMode   string `json:"baseMode"`
	TargetMode string `json:"targetMode"`
}

func regularEntry(entry TreeEntry) bool {
	return entry.Mode == "" || entry.Mode == "100644" || entry.Mode == "100755"
}

func entrySide(entry TreeEntry) string {
	if entry.Mode == "" {
		return "absent"
	}
	return entry.Mode
}

// CanonicalRegularDiffWithCreateDestinations is the CAX-V0-020 derivation.
// It proves the full canonical patch exactly as CanonicalDiffWithCreateDestinations
// does, then removes every section of each changed path with a non-regular
// side (both sections of a change between kinds) and returns the remaining
// bytes, the create destinations of the remaining paths, and the excluded
// entries in path order. The remaining bytes are byte-identical to the
// canonical patch of the regular paths alone. With no non-regular path the
// bytes are the cem/1.0 canonical patch unchanged.
func (r *Repository) CanonicalRegularDiffWithCreateDestinations(ctx context.Context, baseOID, targetOID string) ([]byte, map[string]TreeEntry, []NonRegularEntry, error) {
	out, changed, err := r.provedDiff(ctx, baseOID, targetOID)
	if err != nil {
		return nil, nil, nil, err
	}
	sections := patchSections(changed)
	chunks := splitPatchSections(out)
	if len(chunks) != len(sections) {
		return nil, nil, nil, unavailable("the proved patch does not split into one chunk per predicted section")
	}
	excluded := make(map[string]bool)
	var nonRegular []NonRegularEntry
	var regular []changedEntry
	for _, entry := range changed {
		if regularEntry(entry.old) && regularEntry(entry.new) {
			regular = append(regular, entry)
			continue
		}
		excluded[entry.path] = true
		nonRegular = append(nonRegular, NonRegularEntry{Path: entry.path, BaseMode: entrySide(entry.old), TargetMode: entrySide(entry.new)})
	}
	var kept bytes.Buffer
	for index, section := range sections {
		if !excluded[section.path] {
			kept.Write(chunks[index])
		}
	}
	createDestinations, err := r.createDestinations(ctx, regular)
	if err != nil {
		return nil, nil, nil, err
	}
	return kept.Bytes(), createDestinations, nonRegular, nil
}

// splitPatchSections cuts proved patch bytes before each "diff --git " line.
// Inside a proved section every other line starts with a header keyword or
// with ' ', '+', '-' or '\', so the cut points are exactly the section starts.
func splitPatchSections(out []byte) [][]byte {
	var chunks [][]byte
	start := -1
	for offset := 0; offset < len(out); {
		end := bytes.IndexByte(out[offset:], '\n')
		next := len(out)
		if end >= 0 {
			next = offset + end + 1
		}
		if bytes.HasPrefix(out[offset:], []byte("diff --git ")) {
			if start >= 0 {
				chunks = append(chunks, out[start:offset])
			}
			start = offset
		}
		offset = next
	}
	if start >= 0 {
		chunks = append(chunks, out[start:])
	}
	return chunks
}
