// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// gatedLocatorsPath is the planner's gated root-locator declaration
// (TSP-V0-003, affected.GatedRootLocatorsPath). Select honours it as the
// planner does (TSP-V0-862): a declared package whose root locating runs only
// behind its gate keeps its own reason but is no longer a root locator, so its
// dependents leave rule (d).
const gatedLocatorsPath = ".corvint/gated-root-locators.json"

const (
	gatedLocatorsProfile    = "corvint-gated-root-locators/0"
	maxGatedLocatorBytes    = 64 << 10
	maxGatedLocatorPackages = 64
	maxGatedLocatorReason   = 512
)

type gatedLocator struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

// applyGatedLocators clears locatesRoot on every declared package when the
// declaration is valid for this index: the closed profile and bounds the
// planner's readGatedLocators applies, and each declared directory a package
// whose non-test code locates the root and whose non-test Go sources carry the
// gate as an interpreted string literal. A missing declaration changes
// nothing; any other failure declares nothing and returns the reason, so the
// wider closure stays.
func (index *repositoryIndex) applyGatedLocators(root string) string {
	declared, err := readGatedLocators(root)
	if err != nil {
		return err.Error()
	}
	for directory, entry := range declared {
		pkg := index.packages[directory]
		if pkg == nil || !pkg.locatesRoot {
			return fmt.Sprintf("%s: package %q does not locate the root", gatedLocatorsPath, directory)
		}
		if !carriesGate(root, directory, entry.Gate) {
			return fmt.Sprintf("%s: package %q does not carry its gate %q", gatedLocatorsPath, directory, entry.Gate)
		}
	}
	for directory := range declared {
		index.packages[directory].locatesRoot = false
	}
	return ""
}

func readGatedLocators(root string) (map[string]gatedLocator, error) {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(gatedLocatorsPath)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", gatedLocatorsPath)
	}
	body, err := io.ReadAll(io.LimitReader(file, maxGatedLocatorBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxGatedLocatorBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", gatedLocatorsPath, maxGatedLocatorBytes)
	}
	var declared struct {
		Profile  string                  `json:"profile"`
		Packages map[string]gatedLocator `json:"packages"`
	}
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", gatedLocatorsPath, err)
	}
	if declared.Profile != gatedLocatorsProfile || declared.Packages == nil || len(declared.Packages) > maxGatedLocatorPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", gatedLocatorsPath, gatedLocatorsProfile, maxGatedLocatorPackages)
	}
	for directory, entry := range declared.Packages {
		if directory == "" || directory == "." || path.Clean(directory) != directory || path.IsAbs(directory) || directory == ".." || strings.HasPrefix(directory, "../") || !utf8.ValidString(directory) || strings.ContainsFunc(directory, unicode.IsControl) {
			return nil, fmt.Errorf("%s: package %q", gatedLocatorsPath, directory)
		}
		if !path.IsAbs(entry.Gate) || path.Clean(entry.Gate) != entry.Gate || entry.Gate == "/" || !utf8.ValidString(entry.Gate) || strings.ContainsFunc(entry.Gate, unicode.IsControl) {
			return nil, fmt.Errorf("%s: package %q gate %q", gatedLocatorsPath, directory, entry.Gate)
		}
		if entry.Reason == "" || len(entry.Reason) > maxGatedLocatorReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: package %q reason", gatedLocatorsPath, directory)
		}
	}
	return declared.Packages, nil
}

// carriesGate reports whether a non-test Go source of directory holds gate as
// an interpreted string literal; an unreadable directory or source is false.
func carriesGate(root, directory, gate string) bool {
	literal := []byte(strconv.Quote(gate))
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(directory), name))
		if err != nil {
			return false
		}
		if bytes.Contains(body, literal) {
			return true
		}
	}
	return false
}
