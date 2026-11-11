// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// Declared is one runner instance the tree declares by its config file
// (TIV-V1-003): the instance every reader expects a record for and that
// refresh lists. Lockfile is the nearest lockfile of the runner's ecosystem
// at or above the root, empty when there is none.
type Declared struct {
	Key      string `json:"key"`
	Runner   string `json:"runner"`
	Root     string `json:"root"`
	Config   string `json:"config"`
	Lockfile string `json:"lockfile,omitzero"`
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
// alone: the runner's default test file pattern.
func candidate(runner, file string) bool {
	base := path.Base(file)
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
// file under its root matching the runner's pattern, outside skipped
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
		if !under(instance.Root, name) || !candidate(instance.Runner, name) || skipped(instance.Runner, path.Dir(name)) {
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
