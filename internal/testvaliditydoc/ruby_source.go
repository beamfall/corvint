package testvaliditydoc

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ExampleSourceReader reads one test file's text and returns each example's source facts keyed by
// its declaring line, with the reader's uncertainty reason ("" when it read the whole file).
// testsource.ExampleSources implements it for RSpec (TSA-V0-062) and JS/TS (TSA-V0-073) files.
type ExampleSourceReader func(name, text string) (map[int]testvalidity.ExampleSource, string)

type rubySourceFile struct {
	examples map[int]testvalidity.ExampleSource
	reason   string
}

// applyRubySources recomputes, in place, the hygiene and strength of each Ruby example whose
// retained spec blob is bound (TSA-V0-068, TSA-V0-069). The spec file is read through the
// confined, no-follow, bounded receipt reader and used only when its Git blob ID equals the
// retained one; each file is read once. An example with no bound blob keeps its projection.
func applyRubySources(tests []Test, document rubytestprovider.Document, worktree *os.Root, read ExampleSourceReader) {
	files := map[string]rubySourceFile{}
	for index, example := range document.Examples {
		if index >= len(tests) || tests[index].ID != example.ID || example.SpecBlob == "" {
			continue
		}
		file, ok := files[example.File]
		if !ok {
			file = readRubySource(worktree, document.ObjectFormat, example, read)
			files[example.File] = file
		}
		source := testvalidity.ExampleSource{Read: file.reason, Findings: []testvalidity.SourceFinding{}}
		if file.reason == "" {
			if located, found := file.examples[example.Line]; found {
				source = located
			} else {
				source.Read = testvalidity.HygieneReasonRSpecExampleUnplaced
			}
		}
		tests[index].Projection = testvalidity.ApplyExampleSource(tests[index].Projection, source, example.ID)
		tests[index].SourceRules = &source
	}
}

func readRubySource(worktree *os.Root, format string, example rubytestprovider.Example, read ExampleSourceReader) rubySourceFile {
	if namesGit(example.File) {
		return rubySourceFile{reason: testvalidity.HygieneReasonRSpecSourceUnbound}
	}
	data, err := ReadFile(worktree, filepath.FromSlash(example.File))
	if err != nil || rubytestprovider.BlobID(format, data) != example.SpecBlob {
		return rubySourceFile{reason: testvalidity.HygieneReasonRSpecSourceUnbound}
	}
	examples, reason := read(example.File, string(data))
	return rubySourceFile{examples: examples, reason: reason}
}
