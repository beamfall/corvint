package contextindex

import (
	"encoding/json"
	"io"
	"math"
	"path/filepath"
	"time"
)

// buildCostFormat versions the store's one build-cost record, a sidecar
// beside the snapshots that is not itself a snapshot: eviction counts only
// `.gob` files, and no snapshot byte depends on it (IDX-SNAP-V0-012).
const (
	buildCostFormat   = "corvint-index-build-cost/0"
	buildCostName     = "build-cost.json"
	buildCostMaxBytes = 256
	// A longer record cannot be a Duration the writer measured; converting it would overflow.
	buildCostMaxMilliseconds = math.MaxInt64 / int64(time.Millisecond)
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

// RecordedBuildCost returns the last recorded `index` build cost, or ok=false
// when there is none or it is not a well-formed record. It runs no Git
// process and writes nothing, so a hook can consult it in milliseconds.
func RecordedBuildCost(root string) (time.Duration, bool) {
	base, target, present := buildCostTarget(root)
	if !present {
		return 0, false
	}
	file, err := openBlobShard(base, target)
	if err != nil {
		return 0, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > buildCostMaxBytes {
		return 0, false
	}
	content, err := io.ReadAll(io.LimitReader(file, buildCostMaxBytes+1))
	if err != nil || len(content) > buildCostMaxBytes {
		return 0, false
	}
	var record buildCostRecord
	if json.Unmarshal(content, &record) != nil || record.Format != buildCostFormat || record.BuildMilliseconds < 0 || record.BuildMilliseconds > buildCostMaxMilliseconds {
		return 0, false
	}
	return time.Duration(record.BuildMilliseconds) * time.Millisecond, true
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
// writes, and the record's confined publication, which a killed writer can leave.
func isStoreTemporary(name string) bool {
	snapshot, _ := filepath.Match("snapshot-*.tmp", name)
	record, _ := filepath.Match("blob-*.tmp", name)
	return snapshot || record
}
