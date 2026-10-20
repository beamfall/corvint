package testsource

import (
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
// nothing (LPCV-V0-152). Files are bounded by MaxFiles and the shared byte budget, refused as
// tests-bound-exceeded; a malformed path is tests-invalid-arguments and a named path holding no
// committed test file is tests-source-unavailable.
func StaticExamples(r Repo, revision string, paths []string) (string, []StaticExampleFile, error) {
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
	texts, _, err := r.readFiles(commit, keep)
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
			file.Examples, file.Reason = RSpecExampleSourcesWithSupport(name, texts[name], r.rubySupport(commit, name, supports))
		} else {
			file.Examples, file.Reason = JSExampleSources(name, texts[name])
		}
		files = append(files, file)
	}
	return commit, files, nil
}
