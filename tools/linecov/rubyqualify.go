// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
)

// RubyQualificationProfile names a Ruby fault-run report (LPCV-V0-7608).
const RubyQualificationProfile = "linecov-ruby-qualification/1"

// RubyQualReport is a seeded fault run on a pinned public corpus: for each
// runner, its complete baseline and, per fault, the complete suite's failing
// tests (the oracle) and the tests the candidate plan selected.
type RubyQualReport struct {
	Profile          string                  `json:"profile"`
	Corpus           linecov.QualifiedCorpus `json:"corpus"`
	Seed             int64                   `json:"seed"`
	SelectorRevision string                  `json:"selectorRevision"`
	Runners          []RubyQualRunner        `json:"runners"`
}

// RubyQualRunner is one runner's part of a report.
type RubyQualRunner struct {
	Family      string          `json:"family"`
	Runner      string          `json:"runner"`
	IndexDigest string          `json:"indexDigest"`
	Baseline    RubySuiteResult `json:"baseline"`
	// Unordered is the index's files kept whole whatever the change.
	Unordered []string        `json:"unordered"`
	Faults    []RubyQualFault `json:"faults"`
}

// RubySuiteResult is one complete suite run.
type RubySuiteResult struct {
	Exit    int         `json:"exit"`
	Tests   int         `json:"tests"`
	Failing [][2]string `json:"failing"`
	Skipped int         `json:"skipped"`
	Outside int         `json:"outside"`
	// Inventory lists every test of a baseline run.
	Inventory [][2]string `json:"inventory,omitempty"`
}

// RubyQualFault is one committed fault: its site, the complete suite's
// result at its head and the candidate selection for its one-line change.
type RubyQualFault struct {
	Kind     string          `json:"kind"`
	Path     string          `json:"path"`
	Line     int             `json:"line"`
	Control  bool            `json:"control"`
	Head     string          `json:"head"`
	Suite    RubySuiteResult `json:"suite"`
	Selected [][2]string     `json:"selected"`
	// KeptByCoverage counts the selected tests whose record reached the
	// change (plan reason selected or stale).
	KeptByCoverage int    `json:"keptByCoverage"`
	Fallback       string `json:"fallback,omitempty"`
}

// rubyFaultResult is the verdict on one fault.
type rubyFaultResult struct {
	Kind     string      `json:"kind"`
	Path     string      `json:"path"`
	Line     int         `json:"line"`
	Control  bool        `json:"control"`
	Killed   bool        `json:"killed"`
	Failing  int         `json:"failing"`
	Selected int         `json:"selected"`
	Missed   [][2]string `json:"missed,omitempty"`
	// Passed is set for a control that killed nothing and kept no test.
	Passed bool `json:"controlPassed,omitempty"`
	// Narrowed is set for a killed fault whose selected set is smaller than
	// the suite.
	Narrowed bool `json:"narrowed,omitempty"`
	// Regardless counts the selected tests of files kept whole whatever the change.
	Regardless int    `json:"keptRegardless"`
	Fallback   string `json:"fallback,omitempty"`
}

// rubyQualify folds a Ruby fault-run report (LPCV-V0-7608): every baseline
// must pass completely, every fault must run the complete suite, and a
// failing test outside the selected set is a miss. A control fails when it
// kills a test, causes a fallback or keeps a test other than those of files
// the index keeps whole whatever the change. A run qualifies its runner only
// when a killed fault kept fewer tests than the suite and every control kept
// no test at all: a run whose control kept tests the index keeps whatever
// the change has no control (GLS-V0-032) and is restated as unqualified.
// With -record it writes the Ruby qualification record, naming the
// qualifying runners, bound to the RubyQualifiedSources blobs at -root's
// HEAD, which must equal those of the report's selector revision.
func rubyQualify(args []string) error {
	fs := flag.NewFlagSet("ruby-qualify", flag.ContinueOnError)
	reportFile := fs.String("report", "", "Ruby fault-run report (required)")
	root := fs.String("root", ".", "Corvint checkout whose HEAD holds the qualified selector sources")
	record := fs.String("record", "", "write the Ruby qualification record here")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: linecov ruby-qualify -report FILE [-root CORVINT_DIR -record FILE]\nExample: linecov ruby-qualify -report /tmp/ruby-report.json -root . -record .github/linecov/ruby-qualification.json\nOutput: the per-runner verdicts; exit 1 on a miss, a failed control, an incomplete run or when no runner qualifies.\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *reportFile == "" {
		return errors.New("-report is required")
	}
	data, err := os.ReadFile(*reportFile)
	if err != nil {
		return err
	}
	var rep RubyQualReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return fmt.Errorf("%s: %w", *reportFile, err)
	}
	runs, verdicts, err := foldRubyReport(rep)
	summary := map[string]any{"seed": rep.Seed, "corpus": rep.Corpus, "runs": runs, "faults": verdicts}
	out, merr := json.MarshalIndent(summary, "", "  ")
	if merr != nil {
		return merr
	}
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	if *record == "" {
		return nil
	}
	q := linecov.Qualified{Profile: linecov.QualifiedProfile, Family: "ruby", IndexCommit: rep.Corpus.Commit, Corpus: &rep.Corpus, Sources: map[string]string{}}
	sum := sha256.Sum256(data)
	q.Report = linecov.QualifiedReport{Digest: "sha256:" + hex.EncodeToString(sum[:]), Seed: rep.Seed, Runs: runs}
	paths := map[string]bool{}
	for i, r := range rep.Runners {
		if runs[i].Unqualified == "" {
			q.Runners = append(q.Runners, r.Runner)
		}
		q.Report.Suite = append(q.Report.Suite, r.Family)
		for _, f := range r.Faults {
			paths[f.Path] = true
		}
	}
	q.Report.Packages = sortedKeys(paths)
	for _, r := range runs {
		q.Report.Mutants += r.Faults
		q.Report.Killed += r.Killed
		q.Report.Controls += r.Controls
		q.Report.Misses += r.Misses
		q.Report.Sites = max(q.Report.Sites, r.Faults+r.Controls)
	}
	if q.Report.Mutants == 0 || q.Report.Killed == 0 || q.Report.Controls == 0 {
		return errors.New("ruby-qualify: a record needs at least one killed fault and one passing control")
	}
	if rep.SelectorRevision == "" {
		return errors.New("ruby-qualify: the report names no selector revision")
	}
	for _, p := range linecov.RubyQualifiedSources {
		head, err := output(*root, "git", "rev-parse", "HEAD:"+p)
		if err != nil {
			return fmt.Errorf("ruby-qualify: %s is not committed at HEAD: %w", p, err)
		}
		if dirty, _ := output(*root, "git", "status", "--porcelain", "--", p); dirty != "" {
			return fmt.Errorf("ruby-qualify: %s has uncommitted changes", p)
		}
		blob, err := output(*root, "git", "rev-parse", rep.SelectorRevision+":"+p)
		if err != nil || blob != head {
			return fmt.Errorf("ruby-qualify: %s at HEAD is blob %s, the qualified selector %s held %q; rerun the qualification", p, head, rep.SelectorRevision, blob)
		}
		q.Sources[p] = head
	}
	return writeJSON(*record, q)
}

// foldRubyReport checks a report and restates each runner's run, with why
// it does not qualify its runner when it does not. It fails on a miss, a
// failed control, an incomplete run, or when no run qualifies its runner.
func foldRubyReport(rep RubyQualReport) ([]linecov.QualifiedRun, []rubyFaultResult, error) {
	if rep.Profile != RubyQualificationProfile {
		return nil, nil, fmt.Errorf("ruby-qualify: profile %q", rep.Profile)
	}
	if rep.Corpus.Commit == "" || rep.Corpus.URL == "" || len(rep.Runners) == 0 {
		return nil, nil, errors.New("ruby-qualify: the report names no corpus or runner")
	}
	var runs []linecov.QualifiedRun
	var verdicts []rubyFaultResult
	var problems, unqualified []string
	qualifying := 0
	for _, r := range rep.Runners {
		kept := 0
		b := r.Baseline
		if r.Runner == "" || r.IndexDigest == "" {
			problems = append(problems, fmt.Sprintf("%s: no runner identity or index digest", r.Family))
		}
		if b.Exit != 0 || len(b.Failing) != 0 || b.Skipped != 0 || b.Outside != 0 || b.Tests <= 0 || len(b.Inventory) != b.Tests {
			problems = append(problems, fmt.Sprintf("%s: the baseline did not pass completely", r.Family))
		}
		run := linecov.QualifiedRun{Runner: r.Runner, IndexDigest: r.IndexDigest, Tests: b.Tests, Selected: []int{}, ControlsKept: []int{}, Unordered: len(r.Unordered)}
		unordered := map[string]bool{}
		for _, f := range r.Unordered {
			unordered[f] = true
		}
		inventory := map[[2]string]bool{}
		for _, t := range b.Inventory {
			inventory[t] = true
			if unordered[t[0]] {
				run.KeptRegardless++
			}
		}
		for _, f := range r.Faults {
			if f.Suite.Tests != b.Tests || f.Suite.Skipped != 0 || f.Suite.Outside != 0 {
				problems = append(problems, fmt.Sprintf("%s: %s %s:%d did not run the complete suite", r.Family, f.Kind, f.Path, f.Line))
			}
			selected := map[[2]string]bool{}
			for _, s := range f.Selected {
				selected[s] = true
			}
			v := rubyFaultResult{Kind: f.Kind, Path: f.Path, Line: f.Line, Control: f.Control, Killed: len(f.Suite.Failing) > 0, Failing: len(f.Suite.Failing), Selected: len(selected), Fallback: f.Fallback}
			for s := range selected {
				if unordered[s[0]] {
					v.Regardless++
				}
				if !inventory[s] {
					problems = append(problems, fmt.Sprintf("%s: %s %s:%d selected %v outside the baseline inventory", r.Family, f.Kind, f.Path, f.Line, s))
				}
			}
			for _, failing := range f.Suite.Failing {
				if !selected[failing] {
					v.Missed = append(v.Missed, failing)
				}
			}
			if f.Control {
				run.Controls++
				run.ControlsKept = append(run.ControlsKept, len(selected))
				if v.Killed || f.Fallback != "" || f.KeptByCoverage != 0 || v.Regardless != len(selected) {
					run.ControlsFailed++
					problems = append(problems, fmt.Sprintf("%s: control %s:%d killed %v, fallback %q, kept %d by coverage and %d of %d whatever the change", r.Family, f.Path, f.Line, v.Killed, f.Fallback, f.KeptByCoverage, v.Regardless, len(selected)))
				}
				v.Passed = !v.Killed && f.Fallback == "" && len(selected) == 0
				kept += len(selected)
			} else {
				run.Faults++
				run.Selected = append(run.Selected, len(selected))
				if v.Killed {
					run.Killed++
					if v.Narrowed = len(selected) < b.Tests; v.Narrowed {
						run.Narrowed++
					}
				}
				if len(v.Missed) > 0 {
					run.Misses++
					problems = append(problems, fmt.Sprintf("%s: %s %s:%d missed %d failing tests", r.Family, f.Kind, f.Path, f.Line, len(v.Missed)))
				}
			}
			verdicts = append(verdicts, v)
		}
		var why []string
		switch {
		case run.Controls == 0:
			why = append(why, "no control ran")
		case kept > 0 || run.KeptRegardless > 0:
			why = append(why, fmt.Sprintf("no control: the index keeps the %d tests of %d files whatever the change (a test of each has no isolated record), so its control kept %v tests instead of none", run.KeptRegardless, run.Unordered, run.ControlsKept))
		}
		if run.Narrowed == 0 {
			why = append(why, fmt.Sprintf("no killed fault narrowed the suite: each of the %d killed faults kept all %d tests", run.Killed, b.Tests))
		}
		if run.Unqualified = strings.Join(why, "; "); run.Unqualified == "" {
			qualifying++
		} else {
			unqualified = append(unqualified, r.Family+": "+run.Unqualified)
		}
		runs = append(runs, run)
	}
	if qualifying == 0 {
		problems = append(problems, fmt.Sprintf("no runner qualifies: %v", unqualified))
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return runs, verdicts, fmt.Errorf("ruby-qualify: %d problems: %v", len(problems), problems)
	}
	return runs, verdicts, nil
}
