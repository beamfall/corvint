package jstestprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const freshDependencyFiles = 512
const freshDependencyFileBytes = 32 << 20
const freshDependencyTotalBytes = 128 << 20

// FreshDependencyManifest is the operator-approved complete local CJS input
// set. Roots bind every installed file, including previously unloaded modules;
// Files additionally pins the admitted test/config/server/observer inputs.
// Native libraries, ESM/dynamic remote loading and hostile mutation containment
// remain outside the trusted source-direct tuple's qualification.
type FreshDependencyManifest struct {
	Roots []string          `json:"roots"`
	Files map[string]string `json:"files"`
}
type FreshDependencyObservation struct {
	Files    map[string]string `json:"files"`
	Failures []string          `json:"failures"`
}

func FreshDependencyDigest(m *FreshDependencyManifest) string {
	b, _ := json.Marshal(m)
	return sha256Hex(b)
}
func freshDependencyPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != "/" && len(path) <= 4096 && !strings.ContainsAny(path, "\x00\r\n") && !secretscreen.MatchString(path)
}
func freshManifestComplete(m *FreshDependencyManifest) bool {
	if m == nil || len(m.Roots) < 1 || len(m.Roots) > 8 || len(m.Files) < 1 || len(m.Files) > freshDependencyFiles || !slices.IsSorted(m.Roots) {
		return false
	}
	for i, root := range m.Roots {
		if !freshDependencyPath(root) || (i > 0 && m.Roots[i-1] == root) {
			return false
		}
		found := false
		for path := range m.Files {
			if strings.HasPrefix(path, root+"/") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	for path, digest := range m.Files {
		if !freshDependencyPath(path) || !rawDigestPattern.MatchString(digest) {
			return false
		}
	}
	return true
}

// ObserveFreshDependencies is a bounded read of the declared local tree, not
// an approval or a claim that arbitrary imported code is qualified. Unexpected
// files are retained so added inputs cannot silently disappear from the join.
func ObserveFreshDependencies(m *FreshDependencyManifest) *FreshDependencyObservation {
	out := &FreshDependencyObservation{Files: map[string]string{}, Failures: []string{}}
	if m == nil || len(m.Roots) > 8 || len(m.Files) > freshDependencyFiles {
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
		if len(seen) > freshDependencyFiles || !freshDependencyPath(path) {
			halted = len(seen) > freshDependencyFiles
			out.Failures = append(out.Failures, "dependency-input-bound")
			return
		}
		for parent := path; parent != "/"; parent = filepath.Dir(parent) {
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
		if !freshDependencyPath(root) {
			out.Failures = append(out.Failures, "dependency-root-invalid")
			continue
		}
		count := 0
		e := filepath.WalkDir(root, func(path string, entry os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			count++
			if count > freshDependencyFiles*4 {
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
func FreshDependenciesMatch(m *FreshDependencyManifest, o *FreshDependencyObservation) bool {
	return freshManifestComplete(m) && o != nil && len(o.Failures) == 0 && reflect.DeepEqual(m.Files, o.Files)
}

func freshImportsBound(imports map[string]string, required string, m *FreshDependencyManifest) bool {
	if m == nil || len(imports) == 0 || len(imports) > freshDependencyFiles || imports[required] == "" {
		return false
	}
	for path, digest := range imports {
		if !rawDigestPattern.MatchString(digest) || m.Files[path] != digest {
			return false
		}
	}
	return true
}
