package testvaliditydoc

import (
	"os"
	"path"
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
// one spec file with the support files a helper call can resolve to (TSA-V0-088), and Inclusion
// reads a shared example through the one inclusion site its receipt id names (TSA-V0-092).
// testsource.RSpecExampleSourcesWithSupport and testsource.RSpecInclusionSource implement them;
// a nil member falls back to the plain ExampleSourceReader.
type RubyReaders struct {
	Examples  func(name, text string, support testvalidity.RubySupport) (map[int]testvalidity.ExampleSource, string)
	Inclusion func(testvalidity.RSpecInclusion) (testvalidity.ExampleSource, string)
}

// applyRubySources recomputes, in place, the hygiene and strength of each Ruby example whose
// retained spec blob is bound (TSA-V0-068, TSA-V0-069). The spec file is read through the
// confined, no-follow, bounded receipt reader and used only when its Git blob ID equals the
// retained one; each file is read once. An example with no bound blob keeps its projection. An
// example whose receipt id names another spec than its file ran through an inclusion and is read
// through it, anchored at the qualified reference (TSA-V0-092).
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
		anchor := example.ID
		var source testvalidity.ExampleSource
		if spec, ids, through := inclusionSite(document, example); through && ruby.Inclusion != nil {
			source, anchor = bound.inclusion(ruby.Inclusion, example, spec, ids)
		} else {
			file, ok := files[example.File]
			if !ok {
				file = readRubySource(worktree, document.ObjectFormat, example, read)
				files[example.File] = file
			}
			source = testvalidity.ExampleSource{Read: file.reason, Findings: []testvalidity.SourceFinding{}}
			if file.reason == "" {
				if located, found := file.examples[example.Line]; found {
					source = located
					tests[index].Location.placeSource(example.File, example.Line, source)
				} else {
					source.Read = testvalidity.HygieneReasonRSpecExampleUnplaced
				}
			}
		}
		tests[index].Projection = testvalidity.ApplyExampleSource(tests[index].Projection, source, anchor)
		tests[index].SourceRules = &source
	}
}

// inclusionSite reports the including spec and scoped id path of an example whose receipt id
// names another spec file than the one declaring it: a shared example run through an inclusion.
func inclusionSite(document rubytestprovider.Document, example rubytestprovider.Example) (string, []int, bool) {
	file, ids, ok := testvalidity.ParseRSpecScopedID(example.RSpecID)
	if !ok {
		return "", nil, false
	}
	spec := path.Join(document.ProjectDirectory, path.Clean(file))
	return spec, ids, spec != example.File
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

// inclusion reads a shared example through the inclusion its scoped id names. The including spec
// is bound by another example the receipt retained for that file; without one the read abstains.
func (b *boundRubyFiles) inclusion(read func(testvalidity.RSpecInclusion) (testvalidity.ExampleSource, string), example rubytestprovider.Example, spec string, ids []int) (testvalidity.ExampleSource, string) {
	unbound := testvalidity.ExampleSource{Read: testvalidity.InclusionSourceUnbound, Findings: []testvalidity.SourceFinding{}}
	shared, ok := b.text(example.File, example.SpecBlob)
	if !ok {
		return unbound, example.ID
	}
	specText, found := "", false
	for _, other := range b.document.Examples {
		if other.File == spec && other.SpecBlob != "" {
			specText, found = b.text(spec, other.SpecBlob)
			break
		}
	}
	if !found {
		return unbound, example.ID
	}
	return read(testvalidity.RSpecInclusion{
		SpecPath: spec, SpecText: specText, ScopedID: ids, Description: example.Description,
		SharedPath: example.File, SharedText: shared, ExampleLine: example.Line, Support: b.support(),
	})
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
