package contextindex

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// analyzerSchemaID versions the facts and encoding in an opt-in Corvint pack.
// Bump it after reviewing any change to the inputs pinned by
// TestAnalyzerSchemaInputs. The audit digest is a maintenance guard, not the
// runtime key: an unrelated executable rebuild must keep using the same pack.
const analyzerSchemaID = "corvint-analyzer/81"

// AnalyzerSchemaID is shared by experimental immutable stores of analyzer facts.
func AnalyzerSchemaID() string { return analyzerSchemaID }

func analyzerEngine() string {
	// The Go standard-library parsers participate in extraction. A toolchain
	// change conservatively invalidates facts even when the schema is unchanged.
	digest := sha256.Sum256([]byte(analyzerSchemaID + "\n" + runtime.Version()))
	return hex.EncodeToString(digest[:])[:16]
}

// probeAnalyzerPack reads the pack the way a full load does: a compact read
// verifies only the identity section, so a corrupt body every loader refuses
// would probe fresh and `index --if-stale` would never rewrite it.
func probeAnalyzerPack(directory string, identity repositoryIdentity) (SnapshotProbe, bool, error) {
	engineID := analyzerEngine()
	path := packPath(directory, identity.objectFormat, identity.treeRevision, engineID)
	index, err := readPackSnapshot(path, identity, engineID, loadFull)
	if err != nil {
		return SnapshotProbe{}, false, nil
	}
	forgetPackHistory(index)
	return SnapshotProbe{Path: path, Tree: identity.treeRevision, Commit: identity.commitRevision, Engine: engineID}, true, nil
}

// Analyzer-key packs have a different stem from the executable-key gob.
// Bound their own inventory, including packs from older schema versions, and
// reserve one of the store's bound slots for the current pack even if its
// clock is old.
func evictAnalyzerPacks(directory, keep string, bound int) int {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0
	}
	type agedPack struct {
		path string
		when int64
	}
	packs := make([]agedPack, 0, len(entries))
	remaining := bound
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != packExtension {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if path == keep {
			remaining--
			continue
		}
		packs = append(packs, agedPack{path, info.ModTime().UnixNano()})
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].when != packs[j].when {
			return packs[i].when > packs[j].when
		}
		return packs[i].path < packs[j].path
	})
	evicted := 0
	for _, pack := range packs[min(remaining, len(packs)):] {
		if os.Remove(pack.path) == nil {
			evicted++
		}
	}
	return evicted
}
