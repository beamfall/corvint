package contextindex

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// analyzerSchemaID versions the facts and encoding in an opt-in Corvint pack.
// Bump it after reviewing any change to the inputs pinned by
// TestAnalyzerSchemaInputs. The audit digest is a maintenance guard, not the
// runtime key: an unrelated executable rebuild must keep using the same pack.
const analyzerSchemaID = "corvint-analyzer/108"

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
// clock is old. Each removed pack is named; one whose tree is live is
// flagged but ranks as before (IDX-SNAP-V0-025).
func evictAnalyzerPacks(directory, keep string, bound int, live map[string]bool) []EvictedSnapshot {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	type agedPack struct {
		path  string
		when  int64
		bytes int64
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
		packs = append(packs, agedPack{path, info.ModTime().UnixNano(), info.Size()})
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].when != packs[j].when {
			return packs[i].when > packs[j].when
		}
		return packs[i].path < packs[j].path
	})
	var evicted []EvictedSnapshot
	for _, pack := range packs[min(remaining, len(packs)):] {
		if os.Remove(pack.path) == nil {
			tree := snapshotTreeOf(pack.path)
			name := strings.TrimSuffix(filepath.Base(pack.path), packExtension)
			evicted = append(evicted, EvictedSnapshot{
				Kind: "pack", Path: pack.path, Bytes: pack.bytes,
				Tree: tree, Engine: name[strings.LastIndexByte(name, '-')+1:], LiveHead: live[tree],
			})
		}
	}
	return evicted
}
