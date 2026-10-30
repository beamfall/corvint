package testvaliditydoc

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

type jsSourceFile struct {
	relative string
	examples map[int]testvalidity.ExampleSource
	reason   string
}

// applyJSSources recomputes, in place, the hygiene and strength of each JS/TS outcome whose anchor
// names one bound test file (TSA-V0-082). The file is read once through the confined receipt
// reader and used only when its SHA-256 equals the retained digest. An outcome with no anchor, an
// anchor that binds no single digest key, or a file the reader says is not a JS/TS test file keeps
// its projection.
func applyJSSources(tests []Test, receipt jstestprovider.Receipt, worktree *os.Root, root string, read ExampleSourceReader) {
	files := map[string]jsSourceFile{}
	for index, outcome := range receipt.Tests {
		if index >= len(tests) || tests[index].Name != outcome.Name || outcome.Anchor == nil || outcome.Anchor.Line <= 0 {
			continue
		}
		key, ok := boundTestFile(receipt.Identity.TestFileDigests, root, outcome.Anchor.File)
		if !ok {
			continue
		}
		file, seen := files[key]
		if !seen {
			file = readJSSource(worktree, root, key, receipt.Identity.TestFileDigests[key], read)
			files[key] = file
		}
		if file.reason == testvalidity.ExampleSourceNotJSTestFile {
			continue
		}
		source := testvalidity.ExampleSource{Read: file.reason, Findings: []testvalidity.SourceFinding{}}
		line := outcome.Anchor.Line
		if file.reason == "" {
			if placed, found := placeJSExample(file.examples, line); found {
				source, line = file.examples[placed], placed
				tests[index].Location.placeSource(file.relative, placed, source)
			} else {
				source.Read = testvalidity.HygieneReasonJSExampleUnplaced
				if tests[index].Location != nil {
					tests[index].Location.Absent[locationTitlePath] = LocationSourceUnplaced
				}
			}
		}
		anchor := fmt.Sprintf("%s:%d", file.relative, line)
		tests[index].Projection = testvalidity.ApplyJSExampleSource(tests[index].Projection, source, anchor)
		tests[index].SourceRules = &source
	}
	placeJSDeclarations(tests, receipt, files, worktree, root, read)
}

// placeJSDeclarations places, by title, each JS/TS outcome whose anchor did not place its
// declaration: a Vitest or unprofiled outcome with no anchor or a failure-frame anchor
// (LPCV-V0-291). Every retained test file bound inside root is read as applyJSSources reads it,
// and an example matches when its literal title path, joined by " " (Vitest) or " > " (Bun),
// equals the outcome's full name. Exactly one match sets the location's file, declaring line and
// title path; more than one names file and line declaration-ambiguous; none leaves the location
// as it was. The projection is unchanged: only an anchor-bound reading moves an axis.
func placeJSDeclarations(tests []Test, receipt jstestprovider.Receipt, files map[string]jsSourceFile, worktree *os.Root, root string, read ExampleSourceReader) {
	keys := []string{}
	for key := range receipt.Identity.TestFileDigests {
		if filepath.IsAbs(key) {
			if relative, err := filepath.Rel(root, key); err == nil && filepath.IsLocal(relative) {
				keys = append(keys, key)
			}
		}
	}
	slices.Sort(keys)
	for index, outcome := range receipt.Tests {
		if index >= len(tests) || tests[index].Name != outcome.Name || outcome.FullName == "" || !unplacedJSDeclaration(tests[index].Location) {
			continue
		}
		type match struct {
			file   string
			line   int
			source testvalidity.ExampleSource
		}
		var matches []match
		for _, key := range keys {
			file, seen := files[key]
			if !seen {
				file = readJSSource(worktree, root, key, receipt.Identity.TestFileDigests[key], read)
				files[key] = file
			}
			if file.reason != "" {
				continue
			}
			for line, example := range file.examples {
				if len(example.TitlePath) > 0 && (strings.Join(example.TitlePath, " ") == outcome.FullName || strings.Join(example.TitlePath, " > ") == outcome.FullName) {
					matches = append(matches, match{file.relative, line, example})
				}
			}
		}
		location := tests[index].Location
		switch len(matches) {
		case 0:
		case 1:
			location.placeSource(matches[0].file, matches[0].line, matches[0].source)
		default:
			location.Absent[locationFile] = LocationDeclarationAmbiguous
			location.Absent[locationLine] = LocationDeclarationAmbiguous
		}
	}
}

// unplacedJSDeclaration reports whether a JS/TS location still lacks its file and line because
// the provider recorded no anchor or its anchor is not the declaration.
func unplacedJSDeclaration(location *Location) bool {
	if location == nil || location.File != "" || location.Line > 0 {
		return false
	}
	reason := location.Absent[locationFile]
	return reason == LocationNotDeclaration || reason == LocationNotRecorded
}

// boundTestFile returns the one testFileDigests key that is an absolute path inside root and that
// equals, or ends with "/" plus, the anchor file.
func boundTestFile(digests map[string]string, root, file string) (string, bool) {
	file = filepath.ToSlash(file)
	match, count := "", 0
	for key := range digests {
		slashed := filepath.ToSlash(key)
		if slashed != file && !strings.HasSuffix(slashed, "/"+file) {
			continue
		}
		count++
		match = key
	}
	if count != 1 || !filepath.IsAbs(match) {
		return "", false
	}
	relative, err := filepath.Rel(root, match)
	if err != nil || !filepath.IsLocal(relative) {
		return "", false
	}
	return match, true
}

func readJSSource(worktree *os.Root, root, key, digest string, read ExampleSourceReader) jsSourceFile {
	relative, _ := filepath.Rel(root, key)
	file := jsSourceFile{relative: filepath.ToSlash(relative)}
	if namesGit(relative) {
		file.reason = testvalidity.HygieneReasonJSSourceUnbound
		return file
	}
	data, err := ReadFile(worktree, relative)
	if err != nil || digestHex(data) != digest {
		file.reason = testvalidity.HygieneReasonJSSourceUnbound
		return file
	}
	file.examples, file.reason = read(file.relative, string(data))
	return file
}

// placeJSExample returns the example declared at line, else the innermost example whose lines
// hold it.
func placeJSExample(examples map[int]testvalidity.ExampleSource, line int) (int, bool) {
	if _, ok := examples[line]; ok {
		return line, true
	}
	best := 0
	for start, example := range examples {
		if start < line && example.EndLine >= line && start > best {
			best = start
		}
	}
	return best, best != 0
}
