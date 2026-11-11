// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// Declared is one runner instance the tree declares by its config file
// (TIV-V1-003): the instance every reader expects a record for and that
// refresh lists. Lockfile is the nearest lockfile of the runner's ecosystem
// at or above the root, empty when there is none. Pattern, set only for an
// executing runner, names what in the config may move its test files off
// the default name pattern (configPattern); such an instance's candidates
// are every file of the runner's language under its root.
type Declared struct {
	Key      string `json:"key"`
	Runner   string `json:"runner"`
	Root     string `json:"root"`
	Config   string `json:"config"`
	Lockfile string `json:"lockfile,omitzero"`
	Pattern  string `json:"pattern,omitzero"`
}

// configNames lists, per runner, the config file names that declare an
// instance in their directory, in precedence order: the first present one
// is the instance's config (one instance per runner and root, TIV-V1-003).
// An RSpec or Minitest helper under spec/ or test/ declares the directory
// above it.
var configNames = map[string][]string{
	testid.Go:         {"go.mod"},
	testid.Vitest:     {"vitest.config.ts", "vitest.config.mts", "vitest.config.cts", "vitest.config.js", "vitest.config.mjs", "vitest.config.cjs"},
	testid.Jest:       {"jest.config.ts", "jest.config.mts", "jest.config.cts", "jest.config.js", "jest.config.mjs", "jest.config.cjs", "jest.config.json"},
	testid.Playwright: {"playwright.config.ts", "playwright.config.mts", "playwright.config.cts", "playwright.config.js", "playwright.config.mjs", "playwright.config.cjs"},
	testid.Pytest:     {"pytest.ini", "pyproject.toml", "tox.ini", "setup.cfg"},
	testid.RSpec:      {".rspec", "spec/spec_helper.rb"},
	testid.Minitest:   {"test/test_helper.rb"},
}

// pytestSections is the section a shared Python config must hold to
// declare pytest; pytest.ini needs none.
var pytestSections = map[string]string{
	"pyproject.toml": "[tool.pytest.ini_options]",
	"tox.ini":        "[pytest]",
	"setup.cfg":      "[tool:pytest]",
}

// lockfileNames lists, per runner, the lockfiles of its ecosystem in
// precedence order.
var lockfileNames = map[string][]string{
	testid.Go:         {"go.sum"},
	testid.Vitest:     {"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"},
	testid.Jest:       {"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"},
	testid.Playwright: {"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"},
	testid.Pytest:     {"uv.lock", "poetry.lock", "Pipfile.lock", "pdm.lock"},
	testid.RSpec:      {"Gemfile.lock"},
	testid.Minitest:   {"Gemfile.lock"},
}

// maxConfigProbe bounds the bytes read from a shared Python config to see
// whether it declares pytest.
const maxConfigProbe = 1 << 20

// skipped reports whether a directory path holds no instance and no test
// file of any runner: a dependency, vendored or fixture tree (node_modules,
// vendor, testdata) or a hidden directory. Go also ignores directories whose
// name starts with an underscore.
func skipped(runner, dir string) bool {
	if dir == "." {
		return false
	}
	for _, element := range strings.Split(dir, "/") {
		switch {
		case element == "node_modules" || element == "vendor" || element == "testdata" || strings.HasPrefix(element, "."):
			return true
		case runner == testid.Go && strings.HasPrefix(element, "_"):
			return true
		}
	}
	return false
}

// Detect lists the runner instances files declares, sorted by key: one per
// runner and directory holding one of its config names (configNames), never
// inside a skipped directory. read probes a shared Python config for its
// pytest section; a probe that fails declares nothing.
func Detect(files map[string]string, read func(name string, limit int) ([]byte, error)) []Declared {
	var declared []Declared
	for _, runner := range testid.Runners {
		roots := map[string]string{}
		for name := range files {
			for rank, config := range configNames[runner] {
				root, ok := configRoot(name, config)
				if !ok || skipped(runner, root) {
					continue
				}
				if section, shared := pytestSections[config]; runner == testid.Pytest && shared {
					data, err := read(name, maxConfigProbe)
					if err != nil || !hasSectionLine(data, section) {
						continue
					}
				}
				if current, held := roots[root]; !held || configRank(runner, current, root) > rank {
					roots[root] = name
				}
			}
		}
		for root, config := range roots {
			declared = append(declared, Declared{
				Key: Key(runner, root), Runner: runner, Root: root, Config: config,
				Lockfile: nearestLockfile(files, runner, root),
				Pattern:  configPattern(runner, config, read),
			})
		}
	}
	sort.Slice(declared, func(i, j int) bool { return declared[i].Key < declared[j].Key })
	return declared
}

// configRoot is the instance root a file declares as config name: its
// directory, or for a helper under spec/ or test/ the directory above.
func configRoot(file, config string) (string, bool) {
	if file == config {
		return ".", true
	}
	root, ok := strings.CutSuffix(file, "/"+config)
	return root, ok && root != ""
}

func configRank(runner, config, root string) int {
	for rank, name := range configNames[runner] {
		if path.Join(root, name) == config {
			return rank
		}
	}
	return len(configNames[runner])
}

func hasSectionLine(data []byte, section string) bool {
	for _, line := range bytes.Split(data, []byte("\n")) {
		if string(bytes.TrimSpace(line)) == section {
			return true
		}
	}
	return false
}

// patternKeys lists, per executing runner, the config members that set or
// may set a non-default test-file pattern: Vitest include (and projects or
// a workspace, whose members set their own), Jest testMatch and testRegex
// (and projects), Playwright testMatch (in a project too), and pytest
// python_files. The reader does not evaluate them. Playwright testDir is not
// one: it only narrows where the default name pattern applies, and a testDir
// outside the root fails the lister, whose files must lie under the root.
var patternKeys = map[string][]string{
	testid.Vitest:     {"include", "projects", "workspace"},
	testid.Jest:       {"testMatch", "testRegex", "projects"},
	testid.Playwright: {"testMatch"},
	testid.Pytest:     {"python_files"},
}

// patternWords holds one whole-word expression per patternKeys member.
var patternWords = func() map[string]*regexp.Regexp {
	words := map[string]*regexp.Regexp{}
	for _, keys := range patternKeys {
		for _, key := range keys {
			words[key] = regexp.MustCompile(`\b` + key + `\b`)
		}
	}
	return words
}()

// relativeImport matches a JavaScript import or require of a relative
// module, which may set a pattern the config's own bytes do not show.
var relativeImport = regexp.MustCompile(`(?:\bfrom|\brequire\s*\(|\bimport\s*\(|\bimport)\s*['"]\.\.?/`)

// configPattern is the instance's Pattern: the pattern members its config
// names as whole words, joined by ", " in patternKeys order, plus "a
// relative import" for a JavaScript config that imports a relative module,
// or "an unreadable config" when the config cannot be read within
// maxConfigProbe. It fails closed: a member named in a comment or string
// counts. Static runners and Go have no Pattern.
func configPattern(runner, config string, read func(name string, limit int) ([]byte, error)) string {
	keys, ok := patternKeys[runner]
	if !ok {
		return ""
	}
	data, err := read(config, maxConfigProbe)
	if err != nil {
		return "an unreadable config"
	}
	var found []string
	for _, key := range keys {
		if patternWords[key].Match(data) {
			found = append(found, key)
		}
	}
	if runner != testid.Pytest && relativeImport.Match(data) {
		found = append(found, "a relative import")
	}
	return strings.Join(found, ", ")
}

// nearestLockfile is the runner's lockfile in root or the nearest directory
// above it.
func nearestLockfile(files map[string]string, runner, root string) string {
	for dir := root; ; dir = path.Dir(dir) {
		for _, name := range lockfileNames[runner] {
			candidate := path.Join(dir, name)
			if _, ok := files[candidate]; ok {
				return candidate
			}
		}
		if dir == "." {
			return ""
		}
	}
}

// jsExtensions are the source extensions a JavaScript runner's test file
// may carry.
var jsExtensions = []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"}

// candidate reports whether file is a test file of runner by its name
// alone: the runner's default test file pattern, or with wide (a config
// Pattern the reader cannot evaluate) any file of the runner's language.
func candidate(runner, file string, wide bool) bool {
	base := path.Base(file)
	if wide {
		switch runner {
		case testid.Vitest, testid.Jest, testid.Playwright:
			for _, extension := range jsExtensions {
				if strings.HasSuffix(base, extension) {
					return true
				}
			}
			return false
		case testid.Pytest:
			return strings.HasSuffix(base, ".py")
		}
	}
	switch runner {
	case testid.Go:
		return strings.HasSuffix(base, "_test.go")
	case testid.Vitest, testid.Jest, testid.Playwright:
		for _, extension := range jsExtensions {
			stem, ok := strings.CutSuffix(base, extension)
			if !ok {
				continue
			}
			if strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec") {
				return true
			}
			return runner != testid.Playwright && strings.Contains("/"+path.Dir(file)+"/", "/__tests__/")
		}
		return false
	case testid.Pytest:
		return strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py"))
	case testid.RSpec:
		return strings.HasSuffix(base, "_spec.rb")
	case testid.Minitest:
		return base != "test_helper.rb" && strings.HasSuffix(base, ".rb") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.rb"))
	}
	return false
}

// Candidates is the sorted candidate test files of instance in files: every
// file under its root matching the runner's pattern (any file of its
// language when the instance has a Pattern), outside skipped
// directories and outside the root of another instance of the same runner
// nested below it.
func Candidates(files map[string]string, instance Declared, all []Declared) []string {
	var nested []string
	for _, other := range all {
		if other.Runner == instance.Runner && other.Root != instance.Root && under(instance.Root, other.Root) {
			nested = append(nested, other.Root)
		}
	}
	var out []string
	for name := range files {
		if !under(instance.Root, name) || !candidate(instance.Runner, name, instance.Pattern != "") || skipped(instance.Runner, path.Dir(name)) {
			continue
		}
		inner := false
		for _, root := range nested {
			if under(root, name) {
				inner = true
				break
			}
		}
		if !inner {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
