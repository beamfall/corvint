// SPDX-License-Identifier: Apache-2.0

package dynamic

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// RubySelectionRunners names the Ruby runners that have a selection plugin;
// the registry's table row for this family reads it (AFP-V0-9985).
func RubySelectionRunners() []string {
	return []string{runnernames.RunnerMinitest, runnernames.RunnerRailsTest, runnernames.RunnerRspec, runnernames.RunnerTestUnit}
}

// RubySelectionInvocation extends the existing Ruby execution profiles with
// discovery. The caller stages Files and admits executable/configuration pins
// through the same core executor as Build. No process is started here.
func RubySelectionInvocation(r tr.Request, inventory bool) (tr.Invocation, error) {
	switch r.Runner {
	case "minitest", "rails-test", "rspec", "test-unit":
	default:
		return tr.Invocation{}, fmt.Errorf("ruby-runner-unsupported")
	}
	if r.HasRSpecOnlyMembers() {
		return tr.Invocation{}, fmt.Errorf("ruby-selection-runtime-mapping-unsupported")
	}
	v, err := Build(r)
	if err != nil || !inventory {
		return v, err
	}
	if r.Runner == "rspec" {
		v.Argv = append(v.Argv, "--dry-run")
		return v, nil
	}
	b, err := reporters.ReadFile("reporters/ruby_inventory.rb")
	if err != nil {
		return tr.Invocation{}, err
	}
	v.Files = map[string][]byte{"ruby_inventory.rb": b}
	v.ReportPaths = []string{"ruby-inventory.json"}
	v.Argv = []string{filepath.Join(r.ReportDir, "ruby_inventory.rb"), filepath.Join(r.ReportDir, "ruby-inventory.json"), r.Runner, r.Config, "--"}
	v.Argv = append(v.Argv, r.Selectors...)
	return v, nil
}

// RubySelectionReport reuses the existing native report parsers and their
// duplicate-key, count, status and outside-example checks. Discovery is kept
// separate from terminal observations; an empty but complete inventory is valid.
func RubySelectionReport(runner, report string) ([]tr.Test, error) {
	if !utf8.ValidString(report) {
		return nil, fmt.Errorf("ruby-report-invalid-utf8")
	}
	o := tr.Observation{Runner: runner}
	var err error
	switch runner {
	case "rspec":
		err = parseRSpec([]byte(report), &o, false)
	case "minitest", "rails-test", "test-unit":
		err = parseOwned([]byte(report), &o)
	default:
		return nil, fmt.Errorf("ruby-runner-unsupported")
	}
	// RSP-V0-003: an incomplete report still carries its failed examples, so a
	// passing FULL retry cannot erase an earlier failure.
	if err != nil {
		return rubyRetainedFailures(o.Tests), err
	}
	if len(o.Problems) != 0 {
		return rubyRetainedFailures(o.Tests), fmt.Errorf("ruby-report-incomplete:%s", o.Problems[0].Code)
	}
	if len(o.Tests) > tr.MaxTests {
		return rubyRetainedFailures(o.Tests), fmt.Errorf("ruby-report-cap-exceeded")
	}
	seen := map[string]bool{}
	for _, t := range o.Tests {
		if strings.TrimSpace(t.ID) == "" || seen[t.ID] {
			return rubyRetainedFailures(o.Tests), fmt.Errorf("ruby-report-identity-invalid")
		}
		seen[t.ID] = true
		switch t.State {
		case tr.Passed, tr.Failed, tr.Skipped:
		default:
			return rubyRetainedFailures(o.Tests), fmt.Errorf("ruby-report-nonterminal")
		}
	}
	// The stock RSpec parser intentionally does not expose pending_reason on
	// the frozen execution wire. Read it here only for SPI reconciliation.
	if runner == "rspec" {
		var reasons struct {
			Examples []struct {
				ID     string
				Reason string `json:"pending_message"`
			}
		}
		if err := decode([]byte(report), &reasons); err != nil {
			return nil, err
		}
		for i, e := range reasons.Examples {
			if o.Tests[i].State == tr.Skipped {
				o.Tests[i].Message = e.Reason
			}
		}
	}
	return o.Tests, nil
}

// rubyRetainedFailures keeps the failed examples of a report that is otherwise
// rejected: one row per non-empty identity, capped at tr.MaxTests. Only
// failures survive; passes and skips from a rejected report stay unobserved.
func rubyRetainedFailures(tests []tr.Test) []tr.Test {
	var out []tr.Test
	seen := map[string]bool{}
	for _, t := range tests {
		if t.State != tr.Failed || strings.TrimSpace(t.ID) == "" || seen[t.ID] || len(out) == tr.MaxTests {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}
