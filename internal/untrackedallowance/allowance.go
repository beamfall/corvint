// Package untrackedallowance decides which untracked worktree paths a
// committed-content result may tolerate without degrading (decision 0142).
//
// Paths excluded by ignore rules never reach this package: Git status does not
// list them. A listed untracked path is allowed only when it is provably
// disjoint from the Go build that the result depends on; everything else still
// degrades. The allowed set is bound into the result as a count and digest.
package untrackedallowance

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

// Rule names the disjointness rule and the digest domain.
const Rule = "corvint-untracked-allowance/1"

// Allowance is the receipt binding of the untracked paths a result tolerated.
type Allowance struct {
	Rule   string `json:"rule"`
	Count  int    `json:"count"`
	SHA256 string `json:"sha256"`
}

var errMalformed = errors.New("Git status output is malformed")

// goControlNames change what the Go toolchain or Git evaluates for other paths.
var goControlNames = map[string]struct{}{
	"go.mod": {}, "go.sum": {}, "go.work": {}, "go.work.sum": {},
	".gitignore": {}, ".gitattributes": {},
}

// ParseStatus splits `git status --porcelain=v1 -z` output into the untracked
// (`??`) paths and every other changed path, each sorted.
func ParseStatus(raw []byte) (untracked, tracked []string, err error) {
	untracked, tracked = []string{}, []string{}
	for offset := 0; offset < len(raw); {
		end := bytes.IndexByte(raw[offset:], 0)
		if end < 0 {
			return nil, nil, errMalformed
		}
		field := raw[offset : offset+end]
		offset += end + 1
		if len(field) <= 3 || field[2] != ' ' || !utf8.Valid(field[3:]) {
			return nil, nil, errMalformed
		}
		entry := string(field[3:])
		if string(field[:2]) == "??" {
			untracked = append(untracked, entry)
			continue
		}
		tracked = append(tracked, entry)
		if !bytes.ContainsAny(field[:2], "RC") {
			continue
		}
		next := bytes.IndexByte(raw[offset:], 0)
		if next <= 0 {
			return nil, nil, errMalformed
		}
		tracked = append(tracked, string(raw[offset:offset+next]))
		offset += next + 1
	}
	sort.Strings(untracked)
	sort.Strings(tracked)
	return untracked, tracked, nil
}

// GoPackageDirectories returns the directory of every tracked `.go` path.
func GoPackageDirectories(trackedPaths []string) map[string]struct{} {
	directories := make(map[string]struct{})
	for _, tracked := range trackedPaths {
		if strings.HasSuffix(tracked, ".go") {
			directories[path.Dir(tracked)] = struct{}{}
		}
	}
	return directories
}

// OverlapsGoBuild reports whether an untracked path can change a Go build,
// test, or Git evaluation over the committed tree. It is conservative: an
// untracked directory entry, any `.go` file, any Go or Git control file, any
// `vendor` segment, and anything inside a tracked Go package directory (where
// `//go:embed` and `testdata` reach) all overlap. The control-name and
// `vendor` checks fold case: on a case-insensitive filesystem (e.g. macOS
// APFS) an untracked path whose basename or segment differs from the control
// name only in case is the same directory entry the Go toolchain and Git
// resolve, so refusing the allowance is the safe reading on every host,
// including a case-sensitive one where folding merely loses an allowance.
func OverlapsGoBuild(untracked string, packageDirectories map[string]struct{}) bool {
	if strings.HasSuffix(untracked, "/") || strings.HasSuffix(untracked, ".go") {
		return true
	}
	if _, control := goControlNames[strings.ToLower(path.Base(untracked))]; control {
		return true
	}
	for _, segment := range strings.Split(untracked, "/") {
		if strings.EqualFold(segment, "vendor") {
			return true
		}
	}
	for directory := path.Dir(untracked); ; directory = path.Dir(directory) {
		if _, inside := packageDirectories[directory]; inside {
			return true
		}
		if directory == "." || directory == "/" {
			return false
		}
	}
}

// Partition splits untracked paths into the allowed set and the first
// overlapping path, which is empty when every path is allowed.
func Partition(untracked []string, overlaps func(string) bool) ([]string, string) {
	allowed := make([]string, 0, len(untracked))
	for _, candidate := range untracked {
		if overlaps(candidate) {
			return nil, candidate
		}
		allowed = append(allowed, candidate)
	}
	return allowed, ""
}

// Bind digests the sorted allowed set under the rule's domain.
func Bind(allowed []string) Allowance {
	sorted := append([]string(nil), allowed...)
	sort.Strings(sorted)
	digest := sha256.New()
	digest.Write([]byte(Rule))
	digest.Write([]byte{0})
	for _, entry := range sorted {
		digest.Write([]byte(entry))
		digest.Write([]byte{0})
	}
	return Allowance{Rule: Rule, Count: len(sorted), SHA256: fmt.Sprintf("%x", digest.Sum(nil))}
}
