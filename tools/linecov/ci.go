package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
)

// readSelection reads a lineSelection: the member of an affected-plan/1
// receipt, or a bare affected-line-selection/0 document.
func readSelection(file string) (linecov.Selection, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return linecov.Selection{}, err
	}
	var receipt struct {
		LineSelection *linecov.Selection `json:"lineSelection"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return linecov.Selection{}, err
	}
	if receipt.LineSelection != nil {
		if receipt.LineSelection.Profile != linecov.SelectionProfile {
			return linecov.Selection{}, fmt.Errorf("unknown line selection profile %q", receipt.LineSelection.Profile)
		}
		return *receipt.LineSelection, nil
	}
	var sel linecov.Selection
	if err := json.Unmarshal(data, &sel); err != nil {
		return linecov.Selection{}, err
	}
	if sel.Profile != linecov.SelectionProfile {
		return linecov.Selection{}, fmt.Errorf("%s holds no %s", file, linecov.SelectionProfile)
	}
	return sel, nil
}

// CIPlan is plan.json of `linecov ci -selection` (GLS-V0-042, GLS-V0-044).
type CIPlan struct {
	State     string                      `json:"state"`
	Reason    string                      `json:"reason,omitempty"`
	Detail    string                      `json:"detail,omitempty"`
	Index     *linecov.SelectionIndex     `json:"index,omitempty"`
	Excluded  []string                    `json:"excluded"`
	Sliced    []string                    `json:"sliced"`
	Kept      []string                    `json:"kept"`
	Exclusion *linecov.SelectionExclusion `json:"exclusion,omitempty"`
}

// ciSelection rewrites one CI shard's whole-package list by a line
// selection: an exclude unit leaves it, a tests unit becomes a -run slice, a
// compile unit -run ^$, and every other package stays; slices pass through
// and no package is added (GLS-V0-042). A selection that is not NARROWED
// leaves both lists as they are (GLS-V0-043).
func ciSelection(selFile string, whole, slices []string, out string) error {
	sel, err := readSelection(selFile)
	plan := CIPlan{Excluded: []string{}, Sliced: []string{}, Kept: []string{}}
	keepWhole, keepSlices := whole, slices
	switch {
	case err != nil:
		plan.State, plan.Reason, plan.Detail = linecov.StateFallback, "selection-unreadable", err.Error()
	case sel.State != linecov.StateNarrowed:
		plan.State, plan.Reason, plan.Detail, plan.Index = sel.State, sel.Reason, sel.Detail, sel.Index
	default:
		plan.State, plan.Index, plan.Exclusion = sel.State, sel.Index, sel.Exclusion
		split := map[string]bool{}
		for _, s := range slices {
			if f := strings.Fields(s); len(f) == 3 {
				split[f[2]] = true
			} else {
				return fmt.Errorf("ci: malformed slice %q", s)
			}
		}
		units := map[string]linecov.UnitSelection{}
		for _, u := range sel.Units {
			units[strings.TrimPrefix(u.Unit, "go:")] = u
		}
		keepWhole, keepSlices = nil, nil
		for _, pkg := range whole {
			u, ok := units[pkg]
			switch {
			case split[pkg] || !ok || u.Mode == linecov.Whole:
				keepWhole = append(keepWhole, pkg)
				plan.Kept = append(plan.Kept, pkg)
			case u.Mode == linecov.Tests && validRun(u.Run):
				keepSlices = append(keepSlices, "-run "+u.Run+" "+pkg)
				plan.Sliced = append(plan.Sliced, pkg)
			case u.Mode == linecov.Compile:
				keepSlices = append(keepSlices, "-run ^$ "+pkg)
				plan.Sliced = append(plan.Sliced, pkg)
			case u.Mode == linecov.Exclude:
				plan.Excluded = append(plan.Excluded, pkg)
			default: // a pattern the shard's helper could not take: keep it whole
				keepWhole = append(keepWhole, pkg)
				plan.Kept = append(plan.Kept, pkg)
			}
		}
		keepSlices = append(keepSlices, slices...)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	for name, body := range map[string]string{
		"packages":  joinLines(keepWhole),
		"slices":    joinLines(keepSlices),
		"plan.json": string(encoded) + "\n",
	} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// runPattern is the anchored list of plain test names the CI slice helper
// accepts.
var runPattern = regexp.MustCompile(`^\^\((Test|Example|Fuzz)\w*(\|(Test|Example|Fuzz)\w*)*\)\$$`)

func validRun(run string) bool { return runPattern.MatchString(run) }

// Suspect is one suspected miss (GLS-V0-046): a test the main full run
// failed that a narrowed run's line selection left out.
type Suspect struct {
	Test    string `json:"test"`
	Package string `json:"package"`
	Run     string `json:"run"`
	Event   string `json:"event"`
	Base    string `json:"base"`
	Head    string `json:"head"`
	Record  string `json:"record"`
	Mode    string `json:"mode"`
	Merged  string `json:"merged,omitempty"`
	// Runner is the runner identity of the index the narrowed selection
	// read: the build, wrapper and test deadline a confirmation must
	// reproduce, or it confirms nothing (GLS-V0-046).
	Runner string `json:"runner,omitempty"`
}

// failedRun is a failed.json document of a failed main shard.
type failedRun struct {
	Reasons []struct {
		Package   string   `json:"package"`
		Tests     []string `json:"tests"`
		TestCount int      `json:"testCount"`
		Reason    string   `json:"reason"`
	} `json:"reasons"`
}

// miss lists the suspected misses of one failed main full run: every
// failed top-level test, from the shards' failed.json files, that a
// linecov-narrow artifact's NARROWED selection left out. Each -run
// directory holds selection.json and meta (lines "run ID", "event E",
// "base SHA", "head SHA"). A package that failed outside every test counts
// as every test of it failing. It prints the suspects as JSON and decides
// nothing: confirmation reruns the test (GLS-V0-046).
func miss(args []string) error {
	fs := flag.NewFlagSet("miss", flag.ExitOnError)
	var failed, runs multiFlag
	fs.Var(&failed, "failed", "failed.json of a failed main shard (repeatable)")
	fs.Var(&runs, "run", "a linecov-narrow artifact directory of a merged change (repeatable)")
	fs.Parse(args)
	if len(failed) == 0 {
		return errors.New("miss: -failed is required")
	}
	failedTests := map[string]map[string]bool{} // package -> tests; "" = outside every test
	for _, file := range failed {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var f failedRun
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("miss: %s: %w", file, err)
		}
		for _, r := range f.Reasons {
			if failedTests[r.Package] == nil {
				failedTests[r.Package] = map[string]bool{}
			}
			if len(r.Tests) == 0 || r.TestCount > len(r.Tests) {
				failedTests[r.Package][""] = true // some failure is not named
			}
			for _, t := range r.Tests {
				if !strings.Contains(t, "/") {
					failedTests[r.Package][t] = true
				}
			}
		}
	}
	suspects := []Suspect{}
	for _, dir := range runs {
		applied, err := os.ReadFile(filepath.Join(dir, "applied"))
		if err != nil || strings.TrimSpace(string(applied)) != linecov.StateNarrowed {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "after", "plan.json"))
		if err != nil {
			continue
		}
		var plan CIPlan
		if json.Unmarshal(data, &plan) != nil || plan.State != linecov.StateNarrowed {
			continue
		}
		decided := map[string]bool{}
		for _, p := range append(plan.Excluded, plan.Sliced...) {
			decided[p] = true
		}
		meta, err := readMeta(filepath.Join(dir, "meta"))
		if err != nil {
			return fmt.Errorf("miss: %s: %w", dir, err)
		}
		sel, err := readSelection(filepath.Join(dir, "selection.json"))
		if err != nil || sel.State != linecov.StateNarrowed || sel.Base != meta["base"] || sel.Head != meta["head"] {
			continue // an unreadable or FALLBACK selection excluded nothing
		}
		record := ""
		if sel.Exclusion != nil {
			record = sel.Exclusion.Record
		}
		runner := ""
		if sel.Index != nil {
			runner = sel.Index.Runner
		}
		for _, u := range sel.Units {
			pkg := strings.TrimPrefix(u.Unit, "go:")
			tests := failedTests[pkg]
			if !decided[pkg] || len(tests) == 0 || u.Mode == linecov.Whole {
				continue
			}
			kept := map[string]bool{}
			for _, k := range u.Kept {
				kept[k.Name] = true
			}
			for _, t := range sortedKeys(tests) {
				if kept[t] || t == "" && len(u.Kept) > 0 && u.Excluded == 0 {
					continue
				}
				name := t
				if name == "" {
					name = "(outside every test)"
				}
				suspects = append(suspects, Suspect{Test: name, Package: pkg, Run: meta["run"], Event: meta["event"], Base: meta["base"], Head: meta["head"], Record: record, Mode: string(u.Mode), Merged: meta["merged"], Runner: runner})
			}
		}
	}
	sort.Slice(suspects, func(i, j int) bool {
		a, b := suspects[i], suspects[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		return a.Run < b.Run
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(suspects)
}

func readMeta(file string) (map[string]string, error) {
	lines, err := lines(file)
	if err != nil {
		return nil, err
	}
	meta := map[string]string{}
	for _, l := range lines {
		if k, v, ok := strings.Cut(l, " "); ok {
			meta[k] = v
		}
	}
	for _, k := range []string{"run", "event", "base", "head"} {
		if meta[k] == "" {
			return nil, fmt.Errorf("meta has no %s", k)
		}
	}
	return meta, nil
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }
