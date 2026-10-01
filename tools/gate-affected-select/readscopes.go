package main

// The project-owned read-scope declaration (AFP-V0-023). It mirrors
// internal/liveverify/affected/golang/readscopes.go and InReadScope in
// internal/liveverify/affected/readers.go; the protected wrapper
// .github/testconfine enforces it. A declared package leaves rule (d): it is
// selected only when a dirty path is in its declared scope.

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	readScopesPath       = ".corvint/test-read-scopes.json"
	readScopesProfile    = "corvint-test-read-scopes/0"
	maxReadScopeBytes    = 1 << 20
	maxReadScopePackages = 4096
	maxReadScopeEntries  = 256
)

type readScopeFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// readScopes returns the declared scopes by package directory, nil with no
// declaration, or an error for any declaration that is not exactly valid,
// including one naming a directory that holds no indexed package.
func (index *repositoryIndex) readScopes(root string) (map[string][]string, error) {
	name := filepath.Join(root, filepath.FromSlash(readScopesPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", readScopesPath)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxReadScopeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxReadScopeBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", readScopesPath, maxReadScopeBytes)
	}
	var declared readScopeFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != readScopesProfile || declared.Packages == nil || len(declared.Packages) > maxReadScopePackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", readScopesPath, readScopesProfile, maxReadScopePackages)
	}
	for directory, entries := range declared.Packages {
		if !validRelativePath(directory) || index.packages[directory] == nil || entries == nil || len(entries) > maxReadScopeEntries {
			return nil, fmt.Errorf("%s: package %q", readScopesPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !validReadScopeEntry(entry) || entry <= previous {
				return nil, fmt.Errorf("%s: package %q entry %q", readScopesPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// inReadScope reports whether dirtyPath bears on the declared package in
// directory: the declaration itself, a path in the package's subtree, or a
// declared entry, a path in a declared subtree, or an ancestor of an entry.
func inReadScope(directory string, scope []string, dirtyPath string) bool {
	if dirtyPath == readScopesPath || dirtyPath == directory || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	for _, entry := range scope {
		target := strings.TrimSuffix(entry, "/")
		if dirtyPath == target || strings.HasPrefix(target, dirtyPath+"/") || (target != entry && strings.HasPrefix(dirtyPath, entry)) {
			return true
		}
	}
	return false
}

// validReadScopeEntry is a canonical path, optionally naming a subtree with
// one trailing "/", outside the root .git directory.
func validReadScopeEntry(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	return validRelativePath(trimmed) && trimmed != ".git" && !strings.HasPrefix(trimmed, ".git/")
}

// validRelativePath mirrors affected.ValidRelativePath.
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
