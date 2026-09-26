package contextindex

import (
	"encoding/json"
	"os"
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
)

type buildCostRecord struct {
	Format            string `json:"format"`
	BuildMilliseconds int64  `json:"buildMilliseconds"`
}

// RecordBuildCost stores how long the explicit `index` writer took to build
// the index it just published, replacing any earlier record. Only that writer
// calls it; hooks and read commands only read the record (IDX-SNAP-V0-012).
func RecordBuildCost(root string, cost time.Duration) error {
	directory, present := snapshotDirectoryPresent(root)
	if !present {
		return &Error{Message: "index build cost not recorded: no snapshot store"}
	}
	encoded, err := json.Marshal(buildCostRecord{Format: buildCostFormat, BuildMilliseconds: cost.Milliseconds()})
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "build-cost-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	_, writeErr := temporary.Write(encoded)
	if err := firstError(writeErr, syncAndClose(temporary)); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), filepath.Join(directory, buildCostName))
}

// RecordedBuildCost returns the last recorded `index` build cost, or ok=false
// when there is none or it is not a well-formed record. It runs no Git
// process and writes nothing, so a hook can consult it in milliseconds.
func RecordedBuildCost(root string) (time.Duration, bool) {
	directory, present := snapshotDirectoryPresent(root)
	if !present {
		return 0, false
	}
	path := filepath.Join(directory, buildCostName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > buildCostMaxBytes {
		return 0, false
	}
	content, err := os.ReadFile(path)
	if err != nil || len(content) > buildCostMaxBytes {
		return 0, false
	}
	var record buildCostRecord
	if json.Unmarshal(content, &record) != nil || record.Format != buildCostFormat || record.BuildMilliseconds < 0 {
		return 0, false
	}
	return time.Duration(record.BuildMilliseconds) * time.Millisecond, true
}
