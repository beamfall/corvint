package jstestprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

const freshDependencyFileBytes = 32 << 20
const freshDependencyTotalBytes = 128 << 20

// FreshDependencyManifest is jstestreceipt.FreshDependencyManifest, kept under its old name.
type FreshDependencyManifest = jstestreceipt.FreshDependencyManifest

// FreshDependencyObservation is jstestreceipt.FreshDependencyObservation, kept under its old name.
type FreshDependencyObservation = jstestreceipt.FreshDependencyObservation

// FreshDependencyDigest is jstestreceipt.FreshDependencyDigest, kept under its old name.
func FreshDependencyDigest(m *FreshDependencyManifest) string {
	return jstestreceipt.FreshDependencyDigest(m)
}

// ObserveFreshDependencies is a bounded read of the declared local tree, not
// an approval or a claim that arbitrary imported code is qualified. Unexpected
// files are retained so added inputs cannot silently disappear from the join.
func ObserveFreshDependencies(m *FreshDependencyManifest) *FreshDependencyObservation {
	out := &FreshDependencyObservation{Files: map[string]string{}, Failures: []string{}}
	if m == nil || len(m.Roots) > 8 || len(m.Files) > jstestreceipt.FreshDependencyFiles {
		out.Failures = append(out.Failures, "dependency-manifest-unavailable")
		return out
	}
	total := int64(0)
	seen := map[string]bool{}
	halted := false
	read := func(path string) {
		if halted || seen[path] {
			return
		}
		seen[path] = true
		if total >= freshDependencyTotalBytes {
			out.Failures = append(out.Failures, "dependency-input-bound")
			halted = true
			return
		}
		if len(seen) > jstestreceipt.FreshDependencyFiles || !jstestreceipt.FreshDependencyPath(path) {
			halted = len(seen) > jstestreceipt.FreshDependencyFiles
			out.Failures = append(out.Failures, "dependency-input-bound")
			return
		}
		// The walk ends below the filesystem root, whose Dir is itself: "/" on
		// Unix, a volume such as `C:\` on Windows, where a test for "/" never
		// ended (PTF-V0-755, V1-2492).
		for parent := path; filepath.Dir(parent) != parent; parent = filepath.Dir(parent) {
			info, e := os.Lstat(parent)
			if e != nil || info.Mode()&os.ModeSymlink != 0 {
				out.Failures = append(out.Failures, "dependency-input-unavailable")
				return
			}
			if parent == path && !info.Mode().IsRegular() {
				out.Failures = append(out.Failures, "dependency-input-bound")
				return
			}
		}
		f, e := freshOpenDependency(path)
		if e != nil {
			out.Failures = append(out.Failures, "dependency-input-unavailable")
			return
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() > freshDependencyFileBytes {
			out.Failures = append(out.Failures, "dependency-input-bound")
			return
		}
		remaining := int64(freshDependencyTotalBytes) - total
		if info.Size() > remaining {
			out.Failures = append(out.Failures, "dependency-input-bound")
			halted = true
			return
		}
		h := sha256.New()
		// One overflow sentinel catches growth after Stat without reading later
		// inputs. Even changing files cannot spend more than the total cap + 1.
		limit := min(int64(freshDependencyFileBytes), remaining)
		n, e := io.Copy(h, io.LimitReader(f, limit+1))
		total += n
		if e != nil || n > limit {
			out.Failures = append(out.Failures, "dependency-input-bound")
			halted = total >= freshDependencyTotalBytes
			return
		}
		out.Files[path] = hex.EncodeToString(h.Sum(nil))
		if total == freshDependencyTotalBytes {
			// Exhaustion ends the inventory too: remaining entries cannot be certified.
			out.Failures = append(out.Failures, "dependency-input-bound")
			halted = true
		}
	}
	for _, root := range m.Roots {
		if halted {
			break
		}
		if !jstestreceipt.FreshDependencyPath(root) {
			out.Failures = append(out.Failures, "dependency-root-invalid")
			continue
		}
		count := 0
		e := filepath.WalkDir(root, func(path string, entry os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			count++
			if count > jstestreceipt.FreshDependencyFiles*4 {
				return io.ErrShortBuffer
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return io.ErrUnexpectedEOF
			}
			if !entry.IsDir() {
				read(path)
				if halted {
					return filepath.SkipAll
				}
			}
			return nil
		})
		if e != nil {
			out.Failures = append(out.Failures, "dependency-root-unavailable")
		}
	}
	for path := range m.Files {
		if halted {
			break
		}
		read(path)
	}
	return out
}

// FreshDependenciesMatch is jstestreceipt.FreshDependenciesMatch, kept under its old name.
func FreshDependenciesMatch(m *FreshDependencyManifest, o *FreshDependencyObservation) bool {
	return jstestreceipt.FreshDependenciesMatch(m, o)
}
