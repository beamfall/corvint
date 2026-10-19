package testvaliditydoc

import (
	"fmt"
	"os"
	"path/filepath"
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
			} else {
				source.Read = testvalidity.HygieneReasonJSExampleUnplaced
			}
		}
		anchor := fmt.Sprintf("%s:%d", file.relative, line)
		tests[index].Projection = testvalidity.ApplyJSExampleSource(tests[index].Projection, source, anchor)
		tests[index].SourceRules = &source
	}
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
