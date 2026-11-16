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
	Faults      []RubyQualFault `json:"faults"`
}

// RubySuiteResult is one complete suite run.
type RubySuiteResult struct {
	Exit    int         `json:"exit"`
	Tests   int         `json:"tests"`
	Failing [][2]string `json:"failing"`
	Skipped int         `json:"skipped"`
	Outside int         `json:"outside"`
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
	Fallback string          `json:"fallback,omitempty"`
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
	Passed   bool        `json:"controlPassed,omitempty"`
	Fallback string      `json:"fallback,omitempty"`
}

// rubyQualify folds a Ruby fault-run report (LPCV-V0-7608): every baseline
// must pass completely, every fault must run the complete suite, a failing
// test outside the selected set is a miss, and a control must select and
// kill nothing. With -record it writes the Ruby qualification record bound
// to the RubyQualifiedSources blobs at -root's HEAD, which must equal those
// of the report's selector revision.
func rubyQualify(args []string) error {
	fs := flag.NewFlagSet("ruby-qualify", flag.ContinueOnError)
	reportFile := fs.String("report", "", "Ruby fault-run report (required)")
	root := fs.String("root", ".", "Corvint checkout whose HEAD holds the qualified selector sources")
	record := fs.String("record", "", "write the Ruby qualification record here")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: linecov ruby-qualify -report FILE [-root CORVINT_DIR -record FILE]\nExample: linecov ruby-qualify -report /tmp/ruby-report.json -root . -record .github/linecov/ruby-qualification.json\nOutput: the per-runner verdicts; exit 1 on a miss, a failed control or an incomplete run.\n")
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
	q := linecov.Qualified{Profile: QualifiedProfile, Family: "ruby", IndexCommit: rep.Corpus.Commit, Corpus: &rep.Corpus, Sources: map[string]string{}}
	sum := sha256.Sum256(data)
	q.Report = linecov.QualifiedReport{Digest: "sha256:" + hex.EncodeToString(sum[:]), Seed: rep.Seed, Runs: runs}
	paths := map[string]bool{}
	for _, r := range rep.Runners {
		q.Runners = append(q.Runners, r.Runner)
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
		qualified, err := output(*root, "git", "rev-parse", rep.SelectorRevision+":"+p)
		if err != nil || qualified != head {
			return fmt.Errorf("ruby-qualify: %s at HEAD is blob %s, the qualified selector %s held %q; rerun the qualification", p, head, rep.SelectorRevision, qualified)
		}
		q.Sources[p] = head
	}
	return writeJSON(*record, q)
}

// foldRubyReport checks a report and restates each runner's run.
func foldRubyReport(rep RubyQualReport) ([]linecov.QualifiedRun, []rubyFaultResult, error) {
	if rep.Profile != RubyQualificationProfile {
		return nil, nil, fmt.Errorf("ruby-qualify: profile %q", rep.Profile)
	}
	if rep.Corpus.Commit == "" || rep.Corpus.URL == "" || len(rep.Runners) == 0 {
		return nil, nil, errors.New("ruby-qualify: the report names no corpus or runner")
	}
	var runs []linecov.QualifiedRun
	var verdicts []rubyFaultResult
	var problems []string
	for _, r := range rep.Runners {
		b := r.Baseline
		if r.Runner == "" || r.IndexDigest == "" {
			problems = append(problems, fmt.Sprintf("%s: no runner identity or index digest", r.Family))
		}
		if b.Exit != 0 || len(b.Failing) != 0 || b.Skipped != 0 || b.Outside != 0 || b.Tests <= 0 {
			problems = append(problems, fmt.Sprintf("%s: the baseline did not pass completely", r.Family))
		}
		run := linecov.QualifiedRun{Runner: r.Runner, IndexDigest: r.IndexDigest, Tests: b.Tests, Selected: []int{}}
		for _, f := range r.Faults {
			if f.Suite.Tests != b.Tests || f.Suite.Skipped != 0 || f.Suite.Outside != 0 {
				problems = append(problems, fmt.Sprintf("%s: %s %s:%d did not run the complete suite", r.Family, f.Kind, f.Path, f.Line))
			}
			selected := map[[2]string]bool{}
			for _, s := range f.Selected {
				selected[s] = true
			}
			v := rubyFaultResult{Kind: f.Kind, Path: f.Path, Line: f.Line, Control: f.Control, Killed: len(f.Suite.Failing) > 0, Failing: len(f.Suite.Failing), Selected: len(selected), Fallback: f.Fallback}
			for _, failing := range f.Suite.Failing {
				if !selected[failing] {
					v.Missed = append(v.Missed, failing)
				}
			}
			if f.Control {
				run.Controls++
				v.Passed = len(selected) == 0 && !v.Killed
				if !v.Passed {
					run.ControlsFailed++
					problems = append(problems, fmt.Sprintf("%s: control %s:%d selected %d and killed %v", r.Family, f.Path, f.Line, len(selected), v.Killed))
				}
			} else {
				run.Faults++
				run.Selected = append(run.Selected, len(selected))
				if v.Killed {
					run.Killed++
				}
				if len(v.Missed) > 0 {
					run.Misses++
					problems = append(problems, fmt.Sprintf("%s: %s %s:%d missed %d failing tests", r.Family, f.Kind, f.Path, f.Line, len(v.Missed)))
				}
			}
			verdicts = append(verdicts, v)
		}
		runs = append(runs, run)
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return runs, verdicts, fmt.Errorf("ruby-qualify: %d problems: %v", len(problems), problems)
	}
	return runs, verdicts, nil
}
