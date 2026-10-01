// Package testconfine enforces the repository's declared test read scopes
// (AFP-V0-023). Its declaration grammar mirrors
// internal/liveverify/affected/golang/readscopes.go and
// tools/gate-affected-select/readscopes.go, which narrow selection by it; this
// owner-protected copy is the one CI trusts to confine what a test reads.
package testconfine

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// ScopesPath is the declaration, relative to the repository root.
	ScopesPath    = ".corvint/test-read-scopes.json"
	scopesProfile = "corvint-test-read-scopes/0"
	maxBytes      = 1 << 20
	maxPackages   = 4096
	maxEntries    = 256
)

type scopeFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// Load returns the declared scopes by package directory: nil with no
// declaration, an error for any declaration that is not exactly valid.
func Load(root string) (map[string][]string, error) {
	name := filepath.Join(root, filepath.FromSlash(ScopesPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", ScopesPath)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", ScopesPath, maxBytes)
	}
	var declared scopeFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != scopesProfile || declared.Packages == nil || len(declared.Packages) > maxPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", ScopesPath, scopesProfile, maxPackages)
	}
	for directory, entries := range declared.Packages {
		if !validRelativePath(directory) || entries == nil || len(entries) > maxEntries {
			return nil, fmt.Errorf("%s: package %q", ScopesPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			trimmed := strings.TrimSuffix(entry, "/")
			if !validRelativePath(trimmed) || trimmed == ".git" || strings.HasPrefix(trimmed, ".git/") || entry <= previous {
				return nil, fmt.Errorf("%s: package %q entry %q", ScopesPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

func validRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.HasPrefix(value, "/") || strings.ContainsRune(value, 0) || strings.Contains(value, `\`) {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

// Rule is one hierarchy a confined test may read: a directory and everything
// below it, or one file.
type Rule struct {
	Path string
	Dir  bool
}

// Rules lists what a declared package's tests may read: every path outside
// root, the package's directory subtree, and each declared entry that exists.
// Everything else below root is denied. root must be absolute and clean, with
// no symbolic link in it, and directory a declared package directory.
func Rules(root, directory string, entries []string) ([]Rule, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == string(filepath.Separator) {
		return nil, fmt.Errorf("root %q is not an absolute, clean, non-root directory", root)
	}
	var rules []Rule
	for inner := root; inner != string(filepath.Separator); inner = filepath.Dir(inner) {
		parent := filepath.Dir(inner)
		names, err := readDirNames(parent)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if name == filepath.Base(inner) {
				continue
			}
			rule, ok, err := ruleFor(filepath.Join(parent, name))
			if err != nil {
				return nil, err
			}
			// A symbolic link is not granted: its target outside root is
			// granted in its own right, and inside root it must stay denied.
			if ok && rule.Mode&fs.ModeSymlink == 0 {
				rules = append(rules, Rule{Path: rule.Path, Dir: rule.Mode.IsDir()})
			}
		}
	}
	for index, relative := range append([]string{directory}, entries...) {
		rule, ok, err := ruleFor(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(relative, "/"))))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		// The planner selects below an entry only when it ends in "/", so an
		// entry's form must match what it grants, and a link would grant its
		// target instead of the declared path.
		subtree := index == 0 || strings.HasSuffix(relative, "/")
		if resolved, err := filepath.EvalSymlinks(rule.Path); err != nil || resolved != rule.Path || rule.Mode.IsDir() != subtree {
			return nil, fmt.Errorf("declared path %q is a %v, which its form does not name", relative, rule.Mode.Type())
		}
		rules = append(rules, Rule{Path: rule.Path, Dir: subtree})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Path < rules[j].Path })
	return rules, nil
}

func readDirNames(directory string) ([]string, error) {
	file, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Readdirnames(-1)
}

// ruleFor examines one path without following a final symbolic link; a path
// that does not exist or cannot be examined grants nothing, which can only
// deny more.
func ruleFor(name string) (examined, bool, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return examined{}, false, nil
	}
	if err != nil {
		return examined{}, false, err
	}
	return examined{Path: name, Mode: info.Mode()}, true, nil
}

type examined struct {
	Path string
	Mode fs.FileMode
}

// ConfinedEnv returns env for a confined test with Go's VCS stamping off. A
// confined test cannot read the repository's .git, so a nested `go build`
// would otherwise fail to stamp; a GOFLAGS the test sets itself still wins.
func ConfinedEnv(env []string) []string {
	confined := make([]string, 0, len(env)+1)
	flags := ""
	for _, variable := range env {
		if value, ok := strings.CutPrefix(variable, "GOFLAGS="); ok {
			flags = value
			continue
		}
		confined = append(confined, variable)
	}
	return append(confined, "GOFLAGS="+strings.TrimSpace(flags+" -buildvcs=false"))
}
