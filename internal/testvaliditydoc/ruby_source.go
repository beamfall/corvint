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

// RubyReaders are the Ruby source readers that see the receipt's bound helper set: Examples reads
// one spec file with the support files a helper call can resolve to (TSA-V0-088).
// testsource.RSpecExampleSourcesWithSupport implements it; a nil member falls back to the plain
// ExampleSourceReader.
type RubyReaders struct {
	Examples func(name, text string, support testvalidity.RubySupport) (map[int]testvalidity.ExampleSource, string)
}

// applyRubySources recomputes, in place, the hygiene and strength of each Ruby example whose
// retained spec blob is bound (TSA-V0-068, TSA-V0-069). The spec file is read through the
// confined, no-follow, bounded receipt reader and used only when its Git blob ID equals the
// retained one; each file is read once. An example with no bound blob keeps its projection.
func applyRubySources(tests []Test, document rubytestprovider.Document, worktree *os.Root, read ExampleSourceReader, ruby RubyReaders) {
	files := map[string]rubySourceFile{}
	bound := boundRubyFiles{worktree: worktree, document: document, texts: map[string]*string{}}
	if ruby.Examples != nil {
		read = func(name, text string) (map[int]testvalidity.ExampleSource, string) {
			return ruby.Examples(name, text, bound.support())
		}
	}
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

// boundRubyFiles reads worktree files only when their Git blob ID equals the one the receipt
// retained, caching each read.
type boundRubyFiles struct {
	worktree *os.Root
	document rubytestprovider.Document
	texts    map[string]*string
	helpers  *testvalidity.RubySupport
}

func (b *boundRubyFiles) text(name, blob string) (string, bool) {
	key := name + "\x00" + blob
	if text, ok := b.texts[key]; ok {
		return derefText(text)
	}
	var text *string
	if !namesGit(name) {
		if data, err := ReadFile(b.worktree, filepath.FromSlash(name)); err == nil && rubytestprovider.BlobID(b.document.ObjectFormat, data) == blob {
			s := string(data)
			text = &s
		}
	}
	b.texts[key] = text
	return derefText(text)
}

func derefText(text *string) (string, bool) {
	if text == nil {
		return "", false
	}
	return *text, true
}

// support is the receipt's bound helper set, each file read only when its blob still matches; an
// unbound set or any changed helper leaves it unbound with a reason (TSA-V0-088).
func (b *boundRubyFiles) support() testvalidity.RubySupport {
	if b.helpers != nil {
		return *b.helpers
	}
	support := testvalidity.RubySupport{Files: map[string]string{}, Unbound: b.document.HelpersUnbound}
	for _, helper := range b.document.Helpers {
		if support.Unbound != "" {
			break
		}
		if text, ok := b.text(helper.Path, helper.Blob); ok {
			support.Files[helper.Path] = text
		} else {
			support.Unbound = testvalidity.HygieneReasonRSpecSourceUnbound
		}
	}
	if support.Unbound != "" {
		support.Files = nil
	}
	b.helpers = &support
	return support
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
