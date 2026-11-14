// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// maxMakeFileBytes bounds one make file read at the head or the base.
const maxMakeFileBytes = 4 << 20

// makeReachDeadline bounds the driver unit read behind one make edit.
const makeReachDeadline = time.Minute

// makeSelection is one group of packages an attributed make edit adds:
// `make <pkg> <- <file>` for a package a changed target can run, and
// `make-ci <pkg> <- <workflow>` for a reader of a workflow that invokes a
// changed target directly.
type makeSelection struct {
	kind        string
	directories []string
	cause       string
}

// makeEditAttribution decides an edit of the make file p (NCM-V1-010 to
// NCM-V1-017, the owner-delegated amendment of NCM-V1-005 for V1-5465). It
// returns the changed explicit targets and the packages they add, or why the
// edit runs everything: an unknown base (no full commit id, or a --snapshot
// plan), a file deleted, renamed, new or not a regular file on either side,
// an edit AttributeMakeEdit does not attribute, or targets whose reach
// ReachMakeEdit cannot bound. The make file's own readers keep this
// index's rule (c) attribution beside what it returns.
//
// This file is the selector's only entry into the affected engine besides
// qualified.go and recordadditions.go; tools/gate-ledger's frozen selector
// fixture replaces it with a stub that attributes no make edit.
func makeEditAttribution(plan Receipt, index *repositoryIndex, root, p string) ([]string, []makeSelection, string) {
	commit := plan.Range.Base
	if commit == "" {
		commit = plan.Revision
	}
	if plan.Snapshot != nil || !fullObjectID.MatchString(commit) {
		return nil, nil, "the plan's base commit is unknown"
	}
	if slices.Contains(plan.Plan.Deleted, p) {
		return nil, nil, "the make file was deleted"
	}
	head, reason := headMakeFile(root, p)
	if reason != "" {
		return nil, nil, reason
	}
	base, reason := baseMakeFile(root, commit, p)
	if reason != "" {
		return nil, nil, reason
	}
	edit := affected.AttributeMakeEdit(p, base, head)
	if edit.Reason != "" {
		return nil, nil, edit.Reason
	}
	if len(edit.Targets) == 0 {
		return edit.Targets, nil, ""
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, "the repository root cannot be made absolute"
	}
	ctx, cancel := context.WithTimeout(context.Background(), makeReachDeadline)
	defer cancel()
	reach := affected.ReachMakeEdit(ctx, absolute, p, edit)
	if reach.Reason != "" {
		return nil, nil, reach.Reason
	}
	if missing := index.missingSelector(reach.Selectors); missing != "" {
		if index.inNestedModule(missing) {
			return nil, nil, "a changed make target runs Go selector " + strconv.QuoteToASCII(missing) + " inside a nested module"
		}
		return nil, nil, "a changed make target runs Go package " + strconv.QuoteToASCII(missing) + ", which names no package of the repository"
	}
	selections := []makeSelection{{kind: "make", directories: index.runPackages(reach.Paths), cause: p}}
	for _, workflow := range reach.Workflows {
		selections = append(selections, makeSelection{kind: "make-ci", directories: index.readers(workflow), cause: workflow})
	}
	return edit.Targets, selections, ""
}

// headMakeFile reads the make file from root, or says why it cannot.
func headMakeFile(root, p string) ([]byte, string) {
	name := filepath.Join(root, filepath.FromSlash(p))
	info, err := os.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, "the make file was deleted"
	case err != nil || !info.Mode().IsRegular():
		return nil, "the make file at the head is not a regular file"
	case info.Size() > maxMakeFileBytes:
		return nil, "the make file at the head exceeds the size bound"
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, "the make file at the head cannot be read"
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxMakeFileBytes+1))
	if err != nil || len(body) > maxMakeFileBytes {
		return nil, "the make file at the head cannot be read"
	}
	return body, ""
}

// baseMakeFile reads the make file at commit with the read-only hermetic Git
// of baseGit, or says why it cannot: a file absent at the base is new or
// renamed, and not attributed.
func baseMakeFile(root, commit, p string) ([]byte, string) {
	listing, err := baseGit(root, maxBaseTreeBytes, "ls-tree", "-z", "--full-tree", commit+"^{commit}", "--", p)
	if err != nil {
		return nil, "the base commit cannot be read"
	}
	record := strings.TrimSuffix(string(listing), "\x00")
	if record == "" {
		return nil, "the make file is new or renamed at the head"
	}
	header, name, found := strings.Cut(record, "\t")
	fields := strings.Fields(header)
	if !found || name != p || strings.Contains(record, "\x00") || len(fields) != baseTreeEntryFields || !regularMode(fields[0]) || fields[1] != "blob" {
		return nil, "the make file at the base is not a regular file"
	}
	body, err := baseGit(root, maxMakeFileBytes, "cat-file", "blob", fields[2])
	if err != nil {
		return nil, "the make file at the base cannot be read"
	}
	return body, ""
}

// missingSelector is the first go package selector of selectors that names
// no indexed package, or "" when there is none (NCM-V1-013, V1-5552). The
// driver scans a nested make file's text from its own directory and from the
// root, one set each, and the text runs from one of them, so a set whose
// every selector names a package accounts for the text; runPackages would
// otherwise drop a misread selector, such as `--/b` for `cd -- tool && go
// test ./b`, without a word, and leave the package it meant unselected.
func (index *repositoryIndex) missingSelector(selectors []affected.MakeSelectors) string {
	for _, group := range selectors {
		missing := ""
		for _, pass := range group {
			unnamed := ""
			for _, selector := range pass {
				if !index.namesPackage(selector) {
					unnamed = selector
					break
				}
			}
			if unnamed == "" {
				missing = ""
				break
			}
			if missing == "" {
				missing = unnamed
			}
		}
		if missing != "" {
			return missing
		}
	}
	return ""
}

// inNestedModule reports a driver selector whose directory is, or is below,
// one holding a nested go.mod: the index skips that module's packages, so
// the selector names none of them (NCM-V1-013).
func (index *repositoryIndex) inNestedModule(selector string) bool {
	directory := strings.TrimSuffix(selector, "/...")
	if strings.HasSuffix(directory, ".go") {
		directory = path.Dir(directory)
	}
	for _, nested := range index.nested {
		if directory == nested || strings.HasPrefix(directory, nested+"/") {
			return true
		}
	}
	return false
}

// namesPackage reports whether a driver selector names an indexed package:
// a `/...` pattern one at or below its directory, a `.go` file one in its
// directory, and a directory its own.
func (index *repositoryIndex) namesPackage(selector string) bool {
	if prefix, pattern := strings.CutSuffix(selector, "/..."); pattern {
		for directory := range index.packages {
			if directory != "" && (directory == prefix || strings.HasPrefix(directory, prefix+"/")) {
				return true
			}
		}
		return false
	}
	if strings.HasSuffix(selector, ".go") {
		parent := path.Dir(selector)
		if parent == "." {
			parent = ""
		}
		return index.packages[parent] != nil
	}
	return index.packages[selector] != nil
}

// runPackages are the indexed package directories that the given paths name:
// a package's directory itself, a file directly in it, or the directory
// before a `/...` pattern and every package below it. The driver names the
// repository root's whole tree "..." and the root itself "." or "", so those
// select every package and the root package; a root-level file selects the
// root package (NCM-V1-013).
func (index *repositoryIndex) runPackages(paths []string) []string {
	directories := map[string]bool{}
	for _, value := range paths {
		value = strings.TrimPrefix(value, "./")
		switch {
		case value == "...":
			for directory := range index.packages {
				directories[directory] = true
			}
			continue
		case value == "." || value == "":
			if index.packages[""] != nil {
				directories[""] = true
			}
			continue
		}
		if prefix, pattern := strings.CutSuffix(value, "/..."); pattern {
			for directory := range index.packages {
				if directory != "" && validRelativePath(prefix) && (directory == prefix || strings.HasPrefix(directory, prefix+"/")) {
					directories[directory] = true
				}
			}
			continue
		}
		if !validRelativePath(value) {
			continue
		}
		parent := path.Dir(value)
		if parent == "." {
			parent = ""
		}
		if index.packages[value] != nil {
			directories[value] = true
		} else if index.packages[parent] != nil {
			directories[parent] = true
		}
	}
	return sortedKeys(directories)
}
