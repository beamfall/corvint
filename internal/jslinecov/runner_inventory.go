// SPDX-License-Identifier: Apache-2.0
package jslinecov

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testrunner"
)

const MaxRunnerArtifact = 8 << 20

// Discovery describes core-captured, pinned inputs. Tracked is the complete
// regular-file tree at Key.Checkout, never a glob over the mutable worktree.
// ConfigKnown means the core has admitted the effective configuration (including
// environment and CLI overrides); the zero value deliberately refuses discovery.
// ListOK is set only after a successful, uncapped native list invocation.
type Discovery struct {
	Key                   testrunner.InventoryKey
	Root, Version, Config string
	NodeOptions           string
	Tracked, Files        []string
	ConfigKnown, ListOK   bool
}

// RunnerVersion admits documented stable versions only. The bool indicates
// Jest's per-test collection rather than its file-level list mode.
func RunnerVersion(runner, version string) (major, minor int, collect bool, err error) {
	fail := func() (int, int, bool, error) { return 0, 0, false, fmt.Errorf("inventory-unavailable") }
	p := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(p) != 3 {
		return fail()
	}
	nums := [3]int{}
	for i, s := range p {
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 || strconv.Itoa(n) != s {
			return fail()
		}
		nums[i] = n
	}
	major, minor = nums[0], nums[1]
	switch runner {
	case "vitest":
		if major < 2 || major > 4 {
			return fail()
		}
	case "jest":
		if major < 29 || major > 30 || major == 30 && minor > 5 {
			return fail()
		}
		collect = major == 30 && minor >= 4
	case "node-test":
		if major != 22 || minor < 18 || minor > 23 {
			return fail()
		}
	case "playwright":
		if major != 1 || minor < 50 || minor > 63 {
			return fail()
		}
	default:
		return fail()
	}
	return
}

func (d Discovery) Check() error {
	if _, _, _, err := RunnerVersion(d.Key.Runner, d.Version); err != nil {
		return err
	}
	if strings.ContainsAny(d.NodeOptions, "\x00\r\n") || d.Key.Runner == "node-test" && d.NodeOptions != "" {
		return fmt.Errorf("config-unsupported")
	}
	if !d.ConfigKnown || !filepath.IsAbs(d.Root) || strings.ContainsRune(d.Root, 0) || len(d.Tracked) > testrunner.MaxTests || len(d.Files) > testrunner.MaxTests {
		return fmt.Errorf("config-unsupported")
	}
	if d.Config != "" && (!SafeRunnerPath(d.Config) || d.Key.Runner == "node-test") {
		return fmt.Errorf("config-unsupported")
	}
	if d.Config != "" {
		switch path.Ext(d.Config) {
		case ".js", ".cjs", ".mjs", ".ts", ".cts", ".mts", ".json":
		default:
			return fmt.Errorf("config-unsupported")
		}
	}
	seen := map[string]bool{}
	for _, p := range d.Tracked {
		if !SafeRunnerPath(p) || seen[p] {
			return fmt.Errorf("inventory-unavailable")
		}
		seen[p] = true
	}
	for _, p := range d.Files {
		if !SafeRunnerPath(p) || !seen[p] {
			return fmt.Errorf("config-unsupported")
		}
	}
	if d.Config != "" && !seen[d.Config] {
		return fmt.Errorf("config-unsupported")
	}
	return nil
}

func SafeRunnerPath(p string) bool {
	return p != "" && p != "." && p != ".." && path.Clean(p) == p && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "-") && !strings.ContainsAny(p, "\\:\x00\r\n*?[]{}!,")
}

// Relative accepts native absolute report paths only inside the pinned root;
// every source must be in the captured tree. No filesystem reads occur here.
func (d Discovery) Relative(p string) (string, error) {
	if filepath.IsAbs(p) {
		r, e := filepath.Rel(d.Root, p)
		if e != nil {
			return "", fmt.Errorf("list-output-malformed")
		}
		p = filepath.ToSlash(r)
	}
	if !SafeRunnerPath(p) {
		return "", fmt.Errorf("list-output-malformed")
	}
	for _, known := range d.Tracked {
		if p == known {
			return p, nil
		}
	}
	return "", fmt.Errorf("list-output-malformed")
}

// DecodeRunnerJSON rejects duplicate keys, invalid UTF-8, trailing documents,
// null roots and oversized artifacts before interpreting a runner's report.
func DecodeRunnerJSON(raw string, out any) error {
	s := strings.TrimSpace(raw)
	if len(raw) > MaxRunnerArtifact || s == "" || s == "null" || jsonv2.Unmarshal([]byte(s), out, jsonv2.MatchCaseInsensitiveNames(true)) != nil {
		return fmt.Errorf("list-output-malformed")
	}
	return nil
}

// TestID encodes an unambiguous tuple without relying on display separators.
func TestID(runner, file, project, name string) string {
	b, _ := json.Marshal([]string{file, project, name})
	return runner + ":" + string(b)
}

func Inventory(d Discovery, output string) (testrunner.RunnerInventory, error) {
	fail := func(err error) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Key: d.Key, Reason: err.Error()}, err
	}
	if err := d.Check(); err != nil {
		return fail(err)
	}
	in := testrunner.RunnerInventory{Key: d.Key, Complete: true}
	add := func(file, project, name string) error {
		p, e := d.Relative(file)
		if e != nil {
			return e
		}
		in.Tests = append(in.Tests, testrunner.InventoryTest{ID: TestID(d.Key.Runner, p, project, name), Path: p})
		return nil
	}
	_, _, collect, _ := RunnerVersion(d.Key.Runner, d.Version)
	if d.Key.Runner == "node-test" {
		files := d.Files
		if len(files) == 0 {
			for _, p := range d.Tracked {
				if nodeDefault(p) {
					files = append(files, p)
				}
			}
		}
		for _, p := range files {
			if err := add(p, "", ""); err != nil {
				return fail(err)
			}
		}
	} else {
		if !d.ListOK {
			return fail(fmt.Errorf("inventory-unavailable"))
		}
		switch d.Key.Runner {
		case "vitest":
			var rows []struct{ Name, File, ProjectName string }
			if err := DecodeRunnerJSON(output, &rows); err != nil || rows == nil {
				return fail(fmt.Errorf("list-output-malformed"))
			}
			for _, r := range rows {
				if r.Name == "" {
					return fail(fmt.Errorf("list-output-malformed"))
				}
				// The admitted Vitest JSON reporter has no project identity.
				// Never certify an inventory its terminal report cannot reconcile.
				if r.ProjectName != "" {
					return fail(fmt.Errorf("config-unsupported"))
				}
				if err := add(r.File, r.ProjectName, r.Name); err != nil {
					return fail(err)
				}
			}
		case "jest":
			if collect {
				report, err := readJSReport(output)
				if err != nil {
					return fail(err)
				}
				for _, f := range report.TestResults {
					for _, a := range f.AssertionResults {
						if a.WouldRun == nil || a.FullName == "" {
							return fail(fmt.Errorf("list-output-malformed"))
						}
						if err := add(f.Name, "", a.FullName); err != nil {
							return fail(err)
						}
					}
				}
			} else {
				var files []string
				if err := DecodeRunnerJSON(output, &files); err != nil || files == nil {
					return fail(fmt.Errorf("list-output-malformed"))
				}
				for _, p := range files {
					if err := add(p, "", ""); err != nil {
						return fail(err)
					}
				}
			}
		default:
			return fail(fmt.Errorf("inventory-unavailable"))
		}
	}
	canonical, err := testrunner.CanonicalInventory(in, d.Key)
	if err != nil {
		return fail(fmt.Errorf("list-output-malformed: %w", err))
	}
	return canonical, nil
}

// Node 22.18..23 enables type stripping by default. ConfigKnown admits no
// NODE_OPTIONS or CLI override of that default. node_modules is never walked.
func nodeDefault(p string) bool {
	if strings.Contains("/"+p+"/", "/node_modules/") {
		return false
	}
	ext := path.Ext(p)
	switch ext {
	case ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts":
	default:
		return false
	}
	base := strings.TrimSuffix(path.Base(p), ext)
	return base == "test" || strings.HasPrefix(base, "test-") || strings.HasSuffix(base, ".test") || strings.HasSuffix(base, "-test") || strings.HasSuffix(base, "_test") || strings.Contains("/"+p, "/test/")
}

func ListArgs(d Discovery) ([]string, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	_, _, collect, _ := RunnerVersion(d.Key.Runner, d.Version)
	var args []string
	switch d.Key.Runner {
	case "vitest":
		args = []string{"list", "--json"}
	case "jest":
		args = []string{"--listTests", "--json"}
		if collect {
			args[0] = "--collectTests"
		}
	case "playwright":
		args = []string{"test", "--list", "--reporter=json"}
	case "node-test":
		return nil, nil // tracked-tree discovery, never execute to list
	}
	if d.Config != "" {
		args = append(args, "--config", d.Config)
	}
	// RSP-V0-002/004: discovery and execution must interpret explicit files
	// identically. Admit only runners with a known literal-path encoding.
	if len(d.Files) > 0 {
		switch d.Key.Runner {
		case "jest":
			args = append(args, "--runTestsByPath")
			args = append(args, d.Files...)
		case "playwright":
			for _, p := range d.Files {
				args = append(args, "^"+regexp.QuoteMeta(filepath.Join(d.Root, p))+"$")
			}
		default:
			return nil, fmt.Errorf("config-unsupported")
		}
	}
	return args, nil
}

// FilesFor selects whole file units; per-test inventories deliberately run a
// declared superset when a name filter cannot prove exactness.
func FilesFor(in testrunner.RunnerInventory, tests []string) ([]string, error) {
	if tests == nil {
		return nil, nil
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf("empty-run-set")
	}
	known := map[string]string{}
	for _, t := range in.Tests {
		known[t.ID] = t.Path
	}
	paths := map[string]bool{}
	for _, id := range tests {
		p, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("selection-not-in-inventory")
		}
		paths[p] = true
	}
	out := []string{}
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
