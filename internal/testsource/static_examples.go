package testsource

import (
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// StaticExampleFile is one committed test file's example readings (LPCV-V0-150): the examples
// keyed by declaring line, or the reader's uncertainty reason with no examples.
type StaticExampleFile struct {
	Path     string
	Examples map[int]testvalidity.ExampleSource
	Reason   string
}

// StaticExamples reads every committed RSpec (`_spec.rb`) and JS/TS test file at revision (HEAD
// when "") under the named repository-relative paths, or the whole tree when none is named, through
// the example readers that discovery uses (LPCV-V0-150). It reads Git objects only and writes
// nothing (LPCV-V0-152). The test files and every RSpec support set share one command budget of
// MaxFiles files and maxTotalBytes bytes; exceeding it anywhere refuses tests-bound-exceeded. A
// malformed path is tests-invalid-arguments and a named path holding no committed test file is
// tests-source-unavailable.
func StaticExamples(r Repo, revision string, paths []string) (string, []StaticExampleFile, error) {
	return staticExamplesWithin(r, revision, paths, newReadBudget())
}

// staticExamplesWithin is StaticExamples charged to budget.
func staticExamplesWithin(r Repo, revision string, paths []string, budget *readBudget) (string, []StaticExampleFile, error) {
	for _, name := range paths {
		if !fs.ValidPath(name) || name == ".git" || strings.HasPrefix(name, ".git/") || strings.ContainsAny(name, "\x00\n\r\\") {
			return "", nil, refuse("tests-invalid-arguments", "%q is not a repository-relative path", name)
		}
	}
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return "", nil, err
	}
	under := func(name, root string) bool {
		return root == "." || name == root || strings.HasPrefix(name, root+"/")
	}
	keep := func(name string) bool {
		if !strings.HasSuffix(name, "_spec.rb") && !IsTestFile(name) {
			return false
		}
		return len(paths) == 0 || slices.ContainsFunc(paths, func(root string) bool { return under(name, root) })
	}
	texts, _, err := r.readFilesWithin(commit, keep, budget)
	if err != nil {
		return "", nil, err
	}
	for _, root := range paths {
		found := false
		for name := range texts {
			if under(name, root) {
				found = true
				break
			}
		}
		if !found {
			return "", nil, refuse("tests-source-unavailable", "%s holds no committed RSpec or JS/TS test file at the revision", root)
		}
	}
	names := make([]string, 0, len(texts))
	for name := range texts {
		names = append(names, name)
	}
	slices.Sort(names)
	supports := map[string]testvalidity.RubySupport{}
	files := make([]StaticExampleFile, 0, len(names))
	for _, name := range names {
		file := StaticExampleFile{Path: name}
		if strings.HasSuffix(name, "_spec.rb") {
			support, err := r.staticRubySupport(commit, name, supports, budget)
			if err != nil {
				return "", nil, err
			}
			file.Examples, file.Reason = RSpecExampleSourcesWithSupport(name, texts[name], support)
		} else {
			file.Examples, file.Reason = JSExampleSources(name, texts[name])
		}
		files = append(files, file)
	}
	return commit, files, nil
}

// staticRubySupport reads a spec's committed support set once per prefix, charged to the static
// command's shared budget (LPCV-V0-152). A bound refusal propagates as tests-bound-exceeded; any
// other read failure leaves the set unbound as rspec-support-unreadable, as discovery does.
func (r Repo) staticRubySupport(commit, name string, cache map[string]testvalidity.RubySupport, budget *readBudget) (testvalidity.RubySupport, error) {
	prefix := specPrefix(name)
	if support, ok := cache[prefix]; ok {
		return support, nil
	}
	files, _, err := r.readFilesWithin(commit, RubySupportPaths(prefix), budget)
	support := testvalidity.RubySupport{Files: files}
	if err != nil {
		var refusal *Error
		if errors.As(err, &refusal) && refusal.Code == "tests-bound-exceeded" {
			return testvalidity.RubySupport{}, err
		}
		support = testvalidity.RubySupport{Unbound: "rspec-support-unreadable"}
	}
	cache[prefix] = support
	return support, nil
}
