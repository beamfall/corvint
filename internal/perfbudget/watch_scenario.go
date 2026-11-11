// SPDX-License-Identifier: Apache-2.0
package perfbudget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// The watch-replan inputs (PRB-V0-1014): the index file name in the fixture directory, the code
// packages it records, and the edit each measured run answers for: one body line of a multi-line
// method, so the analyzers resolve it through recorded coverage rather than the structural-edit
// fallback a one-line function forces (LPCV-V0-8207).
const (
	codeLineIndex    = "code-linecov.idx"
	codeIndexedCount = 4
	codeEditBefore   = "\treturn r.Name + Word0002()\n"
	codeEditAfter    = "\treturn r.Name + \" \" + Word0002()\n"
)

// collectCodeIndex collects the code repository's line index over its first codeIndexedCount
// packages with the shared collector and writes it to out, so changed lines in them resolve
// through recorded coverage while the other packages keep whole, as in a partly indexed module.
func collectCodeIndex(ctx context.Context, code, out string) error {
	commit, err := gitOutput(ctx, code, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	runner, goflags, err := linecov.RunnerID(ctx, code, nil, linecov.DefaultTestTimeout, "")
	if err != nil {
		return fmt.Errorf("perf-budget fixture: runner identity: %w", err)
	}
	work, err := os.MkdirTemp("", "corvint-perf-linecov-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	var packages []string
	for n := 1; n <= codeIndexedCount; n++ {
		packages = append(packages, fmt.Sprintf("pkg/p%04d", n))
	}
	outcomes, err := linecov.Collect(ctx, code, linecov.CollectOptions{Module: "example.com/codeapp", Packages: packages, Work: work, Parallel: min(runtime.NumCPU(), 4), Timeout: 10 * time.Minute, SuiteTimeout: 10 * time.Minute, TestTimeout: linecov.DefaultTestTimeout, GOFLAGS: goflags})
	if err != nil {
		return fmt.Errorf("perf-budget fixture: collect line index: %w", err)
	}
	var records []linecov.Record
	var unordered, ran []string
	for _, o := range outcomes {
		if o.Record == nil {
			if o.Name == linecov.InOrder {
				unordered = append(unordered, o.Package)
			}
			continue
		}
		if o.Name == linecov.InOrder {
			ran = append(ran, o.Package)
		}
		records = append(records, *o.Record)
	}
	if len(records) == 0 {
		return errors.New("perf-budget fixture: the line index collected no record")
	}
	return writeIndex(ctx, code, "example.com/codeapp", commit, runner, records, unordered, ran, out)
}

// writeIndex builds and writes one index for root at commit.
func writeIndex(ctx context.Context, root, module, commit, runner string, records []linecov.Record, unordered, ran []string, out string) error {
	oids, err := linecov.OIDs(ctx, root, commit)
	if err != nil {
		return err
	}
	ix, err := linecov.Build(module, commit, runner, oids, records)
	if err != nil {
		return err
	}
	ix.TestTimeout, ix.SuiteTimeout = 10*time.Minute, 10*time.Minute
	ix.Unordered, ix.InOrderRan = unordered, ran
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := ix.Encode(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// writeCodeEdit edits the fixture's Go source in the working tree: the one-line method body
// the watch-replan scenario answers for. It is idempotent, since it runs before every sample.
func writeCodeEdit(f *Fixture) error {
	path := filepath.Join(f.Code, filepath.FromSlash(f.GoPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), codeEditAfter) {
		return nil // the previous run's edit stands; every run answers for the same dirty tree
	}
	edited := strings.Replace(string(raw), codeEditBefore, codeEditAfter, 1)
	if edited == string(raw) {
		return fmt.Errorf("perf-budget: %s holds no %q to edit", f.GoPath, codeEditBefore)
	}
	return os.WriteFile(path, []byte(edited), 0o644)
}

// restoreCodeEdit puts the edited source back to its committed content.
func restoreCodeEdit(ctx context.Context, f *Fixture) error {
	return runContained(gitCommand(ctx, f.Code, "checkout", "--", filepath.FromSlash(f.GoPath)))
}

// measureSelfWatch measures watch-replan-self over Corvint's own repository (PRB-V0-1014): the
// working tree as it stands, never edited, against SelfIndex or a records-free index built for
// its HEAD, so every package keeps whole and the answer path runs without coverage lookups.
func measureSelfWatch(ctx context.Context, id string, o Options, samples int) (ScenarioMeasured, error) {
	m := ScenarioMeasured{ID: id, Samples: []Sample{}}
	root := o.SelfRoot
	if root == "" {
		top, err := gitOutput(ctx, ".", "rev-parse", "--show-toplevel")
		if err != nil {
			m.Error = "self root: the working directory is not in a Git repository and --self-root names none: " + err.Error()
			m.Command = selfDisplayCommand()
			return m, nil
		}
		root = top
	}
	index := o.SelfIndex
	if index == "" {
		work, err := os.MkdirTemp("", "corvint-perf-self-")
		if err != nil {
			return m, err
		}
		defer os.RemoveAll(work)
		index = filepath.Join(work, "self-linecov.idx")
		if err := recordsFreeIndex(ctx, root, index); err != nil {
			m.Error = "self index: " + err.Error()
			m.Command = selfDisplayCommand()
			return m, nil
		}
	}
	m, err := measureCLI(ctx, id, root, []string{o.Corvint, "--root", root, "watch", "--index", index, "--once", "--dry-run"}, samples, 1, nil)
	m.Command = selfDisplayCommand()
	return m, err
}

func selfDisplayCommand() []string {
	return []string{"corvint", "--root", "<self>", "watch", "--index", "<self-index>", "--once", "--dry-run"}
}

// recordsFreeIndex writes an index for root's HEAD under its current runner identity that
// records no test, the fail-closed input a watch answers with every package whole.
func recordsFreeIndex(ctx context.Context, root, out string) error {
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	module, err := linecov.ModulePath(root)
	if err != nil {
		return err
	}
	runner, _, err := linecov.RunnerID(ctx, root, nil, linecov.DefaultTestTimeout, "")
	if err != nil {
		return err
	}
	return writeIndex(ctx, root, module, commit, runner, nil, nil, nil, out)
}
