package contextindex

import (
	"encoding/json"
	"path/filepath"
	"time"
)

// buildCostFormat versions the store's one build-cost record, a sidecar
// beside the snapshots that is not itself a snapshot: eviction counts only
// `.gob` files, and no snapshot byte depends on it (IDX-SNAP-V0-012).
const (
	buildCostFormat = "corvint-index-build-cost/0"
	buildCostName   = "build-cost.json"
)

type buildCostRecord struct {
	Format            string `json:"format"`
	BuildMilliseconds int64  `json:"buildMilliseconds"`
}

// RecordBuildCost stores how long the explicit `index` writer took to build
// the index it just published, replacing any earlier record. Only that writer
// calls it; hooks and read commands only read the record (IDX-SNAP-V0-012).
func RecordBuildCost(root string, cost time.Duration) error {
	base, target, present := buildCostTarget(root)
	if !present {
		return &Error{Message: "index build cost not recorded: no snapshot store"}
	}
	encoded, err := json.Marshal(buildCostRecord{Format: buildCostFormat, BuildMilliseconds: cost.Milliseconds()})
	if err != nil {
		return err
	}
	return publishBlobFact(base, target, encoded)
}

// buildCostTarget names the record for the store's confined no-follow primitives
// (blob_shards_open.go): every directory from the store base is pinned without
// following links and the leaf opens without blocking, so a swapped directory,
// FIFO or device cannot redirect, stall or inflate the record's read or write.
func buildCostTarget(root string) (base, target string, present bool) {
	directory, present := snapshotDirectoryPresent(root)
	return locateSnapshotStore(root).base, filepath.Join(directory, buildCostName), present
}

// isStoreTemporary names the temporaries the writer sweeps once stale: snapshot
// writes, the record's confined publication and the .gitignore rewrite, which a
// killed writer can leave.
func isStoreTemporary(name string) bool {
	snapshot, _ := filepath.Match("snapshot-*.tmp", name)
	record, _ := filepath.Match("blob-*.tmp", name)
	ignore, _ := filepath.Match(".gitignore-*.tmp", name)
	return snapshot || record || ignore
}
