// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// Options limits refresh to matching declared instances. Collect is a test seam;
// the product uses Collect, which delegates to the existing language collectors.
type Options struct {
	Git          testinventory.Git
	Runner, Root string
	Collect      func(context.Context, testinventory.Git, testinventory.Declared, []testinventory.Declared, map[string]string, string) (*linecov.Index, error)
}

type Refreshed struct {
	Instance string `json:"instance"`
	Path     string `json:"path"`
}

func Refresh(ctx context.Context, o Options) ([]Refreshed, error) {
	head, err := o.Git.Head(ctx)
	if err != nil {
		return nil, err
	}
	if !refreshTreeClean(ctx, o.Git) {
		return nil, errors.New("line-evidence: refresh requires a clean tree")
	}
	tree, err := testinventory.NewWorkTree(ctx, o.Git)
	if err != nil {
		return nil, err
	}
	files, _ := tree.Files()
	committed, err := testinventory.NewCommitTree(ctx, o.Git, head)
	if err != nil {
		return nil, err
	}
	clean, _ := committed.Files()
	if !maps.Equal(files, clean) {
		return nil, errors.New("line-evidence: refresh requires a clean tree")
	}
	declared := testinventory.Detect(files, tree.Read)
	store, err := Store(ctx, o.Git)
	if err != nil {
		return nil, err
	}
	collect := o.Collect
	if collect == nil {
		collect = Collect
	}
	result := []Refreshed{}
	for _, d := range declared {
		if o.Runner != "" && d.Runner != o.Runner || o.Root != "" && d.Root != o.Root {
			continue
		}
		if !testinventoryRunner(d.Runner) {
			if o.Runner != "" {
				return result, fmt.Errorf("line-evidence: %s has no collector in 1.0", d.Runner)
			}
			continue
		}
		ix, err := collect(ctx, o.Git, d, declared, files, head)
		if err != nil {
			return result, fmt.Errorf("%s: %w", d.Key, err)
		}
		after, err := testinventory.NewWorkTree(ctx, o.Git)
		if err != nil {
			return result, err
		}
		afterFiles, _ := after.Files()
		afterHead, err := o.Git.Head(ctx)
		if err != nil || afterHead != head || !maps.Equal(files, afterFiles) || !refreshTreeClean(ctx, o.Git) {
			return result, errors.New("line-evidence: tree changed during collection")
		}
		name, err := Write(store, d, ix)
		if err != nil {
			return result, err
		}
		result = append(result, Refreshed{Instance: d.Key, Path: name})
	}
	if len(result) == 0 {
		return result, errors.New("line-evidence: no matching collectable runner instance")
	}
	return result, nil
}

// Only a successful, empty Git status certifies cleanliness. Do not filter
// records by path or kind: modes, links, submodules, conflicts and future status
// forms must all refuse publication. Local ignore settings cannot hide gitlinks
// or executable-bit changes. Git.Output disables optional index writes.
func refreshTreeClean(ctx context.Context, git testinventory.Git) bool {
	status, err := git.Output(ctx, "-c", "core.fileMode=true", "-c", "core.symlinks=true",
		"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	return err == nil && len(status) == 0
}

// Command is the complete suite command at an instance root, never a shell string.
func Command(d testinventory.Declared) []string {
	switch d.Runner {
	case testid.Go:
		return []string{"go", "test", "./..."}
	case testid.Vitest:
		return []string{"node_modules/.bin/vitest", "run"}
	case testid.Jest:
		return []string{"node_modules/.bin/jest", "--runInBand"}
	case testid.Playwright:
		return []string{"node_modules/.bin/playwright", "test"}
	case testid.RSpec:
		if d.Lockfile != "" {
			return []string{"bundle", "exec", "rspec"}
		}
		return []string{"rspec"}
	case testid.Minitest:
		return []string{"ruby", "-Itest", "-e", `ARGV.shift(ARGV.length).each { |f| require File.expand_path(f) }`, "--"}
	}
	return nil
}

// CommandForTree binds collection to the inventory's candidate set. In particular,
// Minitest must not rediscover a smaller suite with a second filename glob.
func CommandForTree(d testinventory.Declared, files map[string]string, all []testinventory.Declared) []string {
	command := Command(d)
	if d.Runner == testid.Minitest {
		for _, file := range testinventory.Candidates(files, d, all) {
			command = append(command, Relative(d.Root, file))
		}
	}
	return command
}

// NativeIdentity uses the same Go runtime contract as explicit line indexes. For
// other runners their executable and interpreter versions are observed read-only.
func NativeIdentity(ctx context.Context, root string, d testinventory.Declared) (string, error) {
	dir := filepath.Join(root, filepath.FromSlash(d.Root))
	if d.Runner == testid.Go {
		id, _, err := linecov.RunnerID(ctx, dir, nil, linecov.DefaultTestTimeout, "")
		return id, err
	}
	argv := Command(d)
	if len(argv) == 0 {
		return "", errors.New("runner has no collector")
	}
	probes := [][]string{{argv[0], "--version"}}
	if d.Runner == testid.Minitest {
		probes = append(probes, []string{"ruby", "-rminitest", "-e", "puts Minitest::VERSION"})
	}
	if d.Runner == testid.RSpec {
		probes = [][]string{{"ruby", "--version"}, append(append([]string(nil), Command(d)...), "--version")}
	}
	if d.Runner == testid.Vitest || d.Runner == testid.Jest || d.Runner == testid.Playwright {
		probes = append(probes, []string{"node", "--version"})
	}
	var parts []string
	for _, args := range probes {
		out, err := testinventory.RunLister(ctx, dir, args)
		if err != nil {
			return "", err
		}
		parts = append(parts, string(out))
	}
	return strings.Join(parts, "\n"), nil
}

func Collect(ctx context.Context, git testinventory.Git, d testinventory.Declared, all []testinventory.Declared, files map[string]string, head string) (*linecov.Index, error) {
	dir := filepath.Join(git.Root, filepath.FromSlash(d.Root))
	native, err := NativeIdentity(ctx, git.Root, d)
	if err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp("", "line-evidence-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	candidates := testinventory.Candidates(files, d, all)
	var names []string
	oids := map[string]string{}
	for name, blob := range files {
		if d.Root == "." || strings.HasPrefix(name, d.Root+"/") {
			oids[Relative(d.Root, name)] = blob
		}
	}
	for _, name := range candidates {
		names = append(names, Relative(d.Root, name))
	}
	command := CommandForTree(d, files, all)
	runner := Binding(d, files, command, native)
	var ix *linecov.Index
	switch d.Runner {
	case testid.Go:
		module, err := linecov.ModulePath(dir)
		if err != nil {
			return nil, err
		}
		var packages []string
		for _, name := range names {
			packages = append(packages, path.Dir(name))
		}
		slices.Sort(packages)
		packages = slices.Compact(packages)
		_, flags, err := linecov.RunnerID(ctx, dir, nil, linecov.DefaultTestTimeout, "")
		if err != nil {
			return nil, err
		}
		outcomes, err := linecov.Collect(ctx, dir, linecov.CollectOptions{Module: module, Packages: packages, Work: work, Parallel: 1, Timeout: 10 * time.Minute, SuiteTimeout: 30 * time.Minute, TestTimeout: linecov.DefaultTestTimeout, GOFLAGS: flags})
		if err != nil {
			return nil, err
		}
		records, err := canonicalRecords(d, outcomes, nil)
		if err != nil {
			return nil, err
		}
		ix, err = linecov.Build(module, head, runner, oids, records)
		if err != nil {
			return nil, err
		}
		ix.Language = "go"
	case testid.Vitest, testid.Jest:
		inventory, err := testinventory.Build(ctx, testinventory.Options{Git: git, Runner: d.Runner, Root: d.Root})
		if err != nil {
			return nil, err
		}
		for _, failure := range inventory.Failures {
			if failure.Key == d.Key {
				return nil, fmt.Errorf("line-evidence: listing failed: %s", failure.Error)
			}
		}
		mapping := map[string]string{}
		for _, instance := range inventory.Instances {
			if instance.Key == d.Key {
				names = nil
				for _, file := range instance.Files {
					names = append(names, Relative(d.Root, file.Path))
				}
				for _, test := range instance.Tests {
					id, e := testid.Parse(test.ID)
					if e != nil || len(id.Segments) == 0 {
						continue
					}
					key := Relative(d.Root, id.File) + "\x00" + strings.Join(id.Segments, " ")
					if previous, ok := mapping[key]; ok && previous != test.ID {
						return nil, errors.New("line-evidence: ambiguous canonical title")
					}
					mapping[key] = test.ID
				}
			}
		}
		executable := filepath.Join(dir, "node_modules", ".bin", d.Runner)
		config := Relative(d.Root, d.Config)
		if path.Base(config) == "package.json" {
			config = ""
		}
		outcomes, err := jslinecov.Collect(ctx, dir, jslinecov.Options{Module: ".", Runner: d.Runner, Executable: executable, Files: names, Config: config, Work: work, Parallel: 1, Timeout: 10 * time.Minute, SuiteTimeout: 30 * time.Minute, Keep: func(p string) bool { return oids[p] != "" }})
		if err != nil {
			return nil, err
		}
		records, err := canonicalRecords(d, outcomes, mapping)
		if err != nil {
			return nil, err
		}
		ix, err = linecov.Build(".", head, runner, oids, records)
		if err != nil {
			return nil, err
		}
		ix.Language = "js"
	case testid.RSpec, testid.Minitest:
		run, err := rubylinecov.Collect(ctx, rubylinecov.Options{Root: dir, Work: work, Argv: command, Timeout: 30 * time.Minute, IsolatedTimeout: 10 * time.Minute, Parallel: 1})
		if err != nil {
			return nil, err
		}
		if run.ExitCode() != 0 {
			return nil, errors.New("line-evidence: suite failed")
		}
		ix, err = rubylinecov.Index(".", head, run, oids)
		if err != nil {
			return nil, err
		}
		var mapping map[string]string
		if d.Runner == testid.RSpec {
			mapping, err = rspecIDs(ctx, dir, d.Root, work, command)
			if err != nil {
				return nil, err
			}
		}
		for i, test := range ix.Tests {
			var id testid.ID
			if test.Package == rubylinecov.Load || test.Name == linecov.InOrder {
				file := joined(d.Root, test.Package)
				if test.Package == rubylinecov.Load {
					file = d.Root
				}
				id, err = testid.New(d.Runner, file)
			} else if d.Runner == testid.Minitest {
				class, method, ok := strings.Cut(test.Name, "#")
				if !ok {
					return nil, errors.New("line-evidence: unmapped Minitest method")
				}
				id, err = testid.MinitestMethod(joined(d.Root, test.Package), class, method)
			} else {
				id, err = testid.Parse(mapping[test.Name])
			}
			if err != nil {
				return nil, fmt.Errorf("line-evidence: canonical test identity: %w", err)
			}
			ix.Tests[i].Name = id.String()
			ix.Tests[i].Elapsed = 0
		}
		ix.Runner = runner
		// Renaming does not change numeric references; Encode preserves their order.
	}
	ix.RunnerFamily, ix.Config, ix.Command = d.Runner, d.Config, command
	ix.TestTimeout, ix.SuiteTimeout = linecov.DefaultTestTimeout, 30*time.Minute
	for i := range ix.Segments {
		ix.Segments[i].Elapsed = 0
	}
	return ix, nil
}

func canonicalRecords(d testinventory.Declared, outcomes []linecov.Outcome, mapping map[string]string) ([]linecov.Record, error) {
	var records []linecov.Record
	for _, o := range outcomes {
		if o.Record == nil {
			return nil, fmt.Errorf("line-evidence: incomplete test %s %s: %s", o.Package, o.Name, o.Divergent)
		}
		r := *o.Record
		var id testid.ID
		var err error
		if r.Name == linecov.InOrder {
			id, err = testid.New(d.Runner, joined(d.Root, r.Package))
		} else if d.Runner == testid.Go {
			id, err = testid.GoTest(joined(d.Root, r.Package), r.Name)
		} else {
			id, err = testid.Parse(mapping[r.Package+"\x00"+r.Name])
		}
		if err != nil {
			return nil, fmt.Errorf("line-evidence: canonical test identity: %w", err)
		}
		r.Name = id.String()
		r.Elapsed = 0
		for i := range r.Segments {
			r.Segments[i].Elapsed = 0
		}
		records = append(records, r)
	}
	return records, nil
}
