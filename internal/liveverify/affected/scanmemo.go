package affected

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

// ScanMemoProfile names the format of the gateselect scan memo file kept in a
// graph cache directory (AFP-V0-822). A file under any other profile is never
// trusted.
const ScanMemoProfile = "corvint-gateselect-scan-memo/0"

// ScanMemoEntryPath is where the scan memo for root and module sits in dir.
// It is named like a graph entry, so the AFP-V0-970 eviction bound counts it.
func ScanMemoEntryPath(dir, root, module string) string {
	sum := sha256.Sum256([]byte(ScanMemoProfile + "\x00" + root + "\x00" + module))
	return filepath.Join(dir, "affected-graph-scan-"+hex.EncodeToString(sum[:16])+".json")
}

// LoadScanMemo returns the verified body of the scan memo for root and module
// in dir, held to every graph cache placement and integrity rule, or nil.
func LoadScanMemo(dir, root string, excluded []string, module string) []byte {
	body, reason := readCacheFile(ScanMemoEntryPath(dir, root, module), root, excluded, ScanMemoProfile)
	if reason != "" {
		return nil
	}
	return body
}

// StoreScanMemo writes body as the scan memo for root and module in dir, as
// a graph entry is written, and returns why it was not stored, or "".
func StoreScanMemo(dir, root string, excluded []string, module string, body []byte) string {
	return writeCacheFile(ScanMemoEntryPath(dir, root, module), root, excluded, ScanMemoProfile, body)
}

// FileStamp is the stat identity a scan memo pins for one file: the stamp a
// graph entry pins, or false where the platform has no change time or the
// file changed at or after horizon, inside the racy window (AFP-V0-822).
func FileStamp(info fs.FileInfo, horizon time.Time) (string, bool) {
	stamp, ok := statStamp(info)
	if !ok || stamp.Mtime >= horizon.UnixNano() || stamp.Ctime >= horizon.UnixNano() {
		return "", false
	}
	return fmt.Sprintf("%o %d %d %d %d %d", stamp.Mode, stamp.Size, stamp.Mtime, stamp.Ctime, stamp.Inode, stamp.Device), true
}
