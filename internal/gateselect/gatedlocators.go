package gateselect

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file reads the project-owned gated root-locator declaration as the
// planner reads it (TSP-V0-003, internal/liveverify/affected/golang/
// gatedlocators.go), so the selector and the planner agree on rule (d)
// (TSP-V0-940). A declared package keeps its own unresolved reason; its
// dependents no longer depend on a root locator.

const (
	gatedLocatorsPath       = ".corvint/gated-root-locators.json"
	gatedLocatorsProfile    = "corvint-gated-root-locators/0"
	maxGatedLocatorBytes    = 64 << 10
	maxGatedLocatorPackages = 64
	maxGatedLocatorReason   = 512
)

type gatedLocator struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

type gatedLocatorFile struct {
	Profile  string                  `json:"profile"`
	Packages map[string]gatedLocator `json:"packages"`
}

// gatedLocators returns the declared packages by directory: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file, or invalid in any member.
func gatedLocators(root string) (map[string]gatedLocator, error) {
	name := filepath.Join(root, filepath.FromSlash(gatedLocatorsPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", gatedLocatorsPath)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxGatedLocatorBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxGatedLocatorBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", gatedLocatorsPath, maxGatedLocatorBytes)
	}
	var declared gatedLocatorFile
	if err := jsonv2.UnmarshalRead(bytes.NewReader(body), &declared, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != gatedLocatorsProfile || declared.Packages == nil || len(declared.Packages) > maxGatedLocatorPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", gatedLocatorsPath, gatedLocatorsProfile, maxGatedLocatorPackages)
	}
	for directory, entry := range declared.Packages {
		if directory == "" || directory == "." || path.Clean(directory) != directory || path.IsAbs(directory) || directory == ".." || strings.HasPrefix(directory, "../") {
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

// applyGatedLocators clears locatesRoot on each declared package once every
// declared directory is an indexed package whose non-test code locates the
// root and whose non-test sources carry the gate as a string literal; any
// failure declares nothing, the wider selection.
func (index *repositoryIndex) applyGatedLocators(root string, files []indexedFile) {
	declared, err := gatedLocators(root)
	if err != nil || len(declared) == 0 {
		return
	}
	for directory, entry := range declared {
		pkg := index.packages[directory]
		if pkg == nil || !pkg.locatesRoot || !carriesLiteral(root, directory, files, entry.Gate) {
			return
		}
	}
	for directory := range declared {
		index.packages[directory].locatesRoot = false
	}
}

// carriesLiteral reports whether a non-test source of the package in
// directory holds value as an interpreted string literal. An unreadable
// source reports false.
func carriesLiteral(root, directory string, files []indexedFile, value string) bool {
	literal := []byte(strconv.Quote(value))
	for _, file := range files {
		if parentDirectory(file.relative) != directory || strings.HasSuffix(file.relative, "_test.go") {
			continue
		}
		body, err := readBounded(filepath.Join(root, filepath.FromSlash(file.relative)), file.expected)
		if err != nil {
			return false
		}
		if bytes.Contains(body, literal) {
			return true
		}
	}
	return false
}
