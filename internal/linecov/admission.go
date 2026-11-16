// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
)

var QualifiedSources = []string{
	"cmd/corvint/affected_linecov.go",
	"internal/linecov/admission.go",
	"internal/linecov/audit.go",
	"internal/linecov/collect.go",
	"internal/linecov/compact.go",
	"internal/linecov/decide.go",
	"internal/linecov/envaudit.go",
	"internal/linecov/evidence.go",
	"internal/linecov/git.go",
	"internal/linecov/go_regions.go",
	"internal/linecov/heads.go",
	"internal/linecov/index.go",
	"internal/linecov/lookup.go",
	"internal/linecov/runner.go",
	"internal/linecov/select.go",
	"internal/linecov/pgroup.go",
	"internal/linecov/pgroup_unix.go",
	"internal/linecov/pgroup_other.go",
	"internal/linecov/pgroup_rss_linux.go",
	"internal/linecov/pgroup_rss_other.go",
	"internal/linecov/segments.go",
	"internal/linecov/qualify.go",
	"internal/linecov/windows.go",
	"internal/linecov/syscallenv.go",
	"internal/linecov/redundancy.go",
	"internal/linecov/shadow.go",
	"internal/linecov/worktree.go",
	"internal/liveverify/affected/readers.go",
	"tools/linecov/main.go",
	"tools/linecov/qualify.go",
	"tools/linecov/ci.go",
}

// RubyQualificationRecord is the Ruby family's qualification record, read
// at the selector executable's own revision (LPCV-V0-7608).
const RubyQualificationRecord = ".github/linecov/ruby-qualification.json"

// RubyQualifiedSources are the Ruby selector and collector sources a Ruby
// record binds: every source of this package (the index, diff and narrowing
// code the Ruby path shares), the Ruby collector and analysis, the lexer the
// structure guard uses, and the commands (LPCV-V0-7608).
var RubyQualifiedSources = append(rubyOwnSources(), []string{
	"internal/rubydialect/rubydialect.go",
	"internal/rubydialect/scanner.go",
	"internal/rubylinecov/artifacts.go",
	"internal/rubylinecov/collect.go",
	"internal/rubylinecov/isolate.go",
	"internal/rubylinecov/linecov.rb",
	"internal/rubylinecov/lines.go",
	"internal/rubylinecov/select.go",
	"internal/rubylinecov/selection.go",
	"tools/linecov/languages.go",
	"tools/linecov/main.go",
	"tools/linecov/rubyqualify.go",
}...)

func rubyOwnSources() []string {
	var own []string
	for _, p := range QualifiedSources {
		if strings.HasPrefix(p, "internal/linecov/") {
			own = append(own, p)
		}
	}
	return own
}

type Qualified struct {
	Profile     string            `json:"profile"`
	Family      string            `json:"family"`
	Runner      string            `json:"runner"`
	IndexCommit string            `json:"indexCommit"`
	Report      QualifiedReport   `json:"report"`
	Sources     map[string]string `json:"sources"`
	// Runners and Corpus describe a family qualified on a public corpus with
	// several runners (Ruby: Minitest and RSpec), each named by its index
	// runner identity (LPCV-V0-7608).
	Runners []string         `json:"runners,omitempty"`
	Corpus  *QualifiedCorpus `json:"corpus,omitempty"`
}

// QualifiedCorpus identifies the pinned public repository a record stands on.
type QualifiedCorpus struct {
	URL           string `json:"url"`
	Commit        string `json:"commit"`
	License       string `json:"license"`
	LicenseSHA256 string `json:"licenseSha256"`
}

// QualifiedRun restates one runner's part of a multi-runner report: its
// suite size, kills, the selected-set size of each fault in seeded order,
// its controls and its misses.
type QualifiedRun struct {
	Runner         string `json:"runner"`
	IndexDigest    string `json:"indexDigest"`
	Tests          int    `json:"tests"`
	Faults         int    `json:"faults"`
	Killed         int    `json:"killed"`
	Selected       []int  `json:"selected"`
	Controls       int    `json:"controls"`
	ControlsFailed int    `json:"controlsFailed"`
	Misses         int    `json:"misses"`
}

// QualifiedReport restates the summarized report a record stands on.
type QualifiedReport struct {
	Digest   string   `json:"digest"`
	Seed     int64    `json:"seed"`
	Sites    int      `json:"sites"`
	Mutants  int      `json:"mutants"`
	Killed   int      `json:"killed"`
	Invalid  int      `json:"invalid"`
	Controls int      `json:"controls"`
	Misses   int      `json:"misses"`
	Packages []string `json:"packages"`
	Suite    []string `json:"suite"`
	// Runs is set for a multi-runner family (LPCV-V0-7608).
	Runs []QualifiedRun `json:"runs,omitempty"`
}

// CheckQualification admits exclusion only for a passing record whose runner
// and source blobs match both the planning base and the executing selector.
func CheckQualification(ctx context.Context, root, base, runner string, info *debug.BuildInfo) error {
	data, err := gitOut(ctx, root, "show", base+":.github/linecov/qualification.json")
	if err != nil {
		return fmt.Errorf("qualification record unavailable: %w", err)
	}
	var q Qualified
	if err := json.Unmarshal(data, &q); err != nil {
		return fmt.Errorf("qualification record invalid: %w", err)
	}
	if !q.passing("go") || q.Runner != runner {
		return fmt.Errorf("qualification is not a passing record for this runner")
	}
	rev, err := cleanRevision(info)
	if err != nil {
		return err
	}
	return q.bound(ctx, root, QualifiedSources, base, rev)
}

// CheckRubyQualification admits the Ruby family for an index collected by
// runner only for a passing Ruby record committed at the selector
// executable's clean VCS revision in selectorRoot, whose bound sources match
// that revision and which qualified runner with zero misses and passing
// controls (LPCV-V0-7608).
func CheckRubyQualification(ctx context.Context, selectorRoot, runner string, info *debug.BuildInfo) error {
	rev, err := cleanRevision(info)
	if err != nil {
		return err
	}
	if selectorRoot == "" {
		return fmt.Errorf("ruby qualification record unavailable: no selector repository")
	}
	data, err := gitOut(ctx, selectorRoot, "show", rev+":"+RubyQualificationRecord)
	if err != nil {
		return fmt.Errorf("ruby qualification record unavailable at %s: %w", rev, err)
	}
	var q Qualified
	if err := json.Unmarshal(data, &q); err != nil {
		return fmt.Errorf("ruby qualification record invalid: %w", err)
	}
	if !q.passing("ruby") || q.Corpus == nil || q.Corpus.Commit == "" || len(q.Report.Runs) == 0 || len(q.Report.Runs) != len(q.Runners) {
		return fmt.Errorf("ruby qualification is not a passing record")
	}
	found := false
	for i, r := range q.Report.Runs {
		if r.Runner != q.Runners[i] || r.Misses != 0 || r.ControlsFailed != 0 || r.Controls <= 0 || r.Killed <= 0 || r.Faults <= 0 || len(r.Selected) != r.Faults || r.Tests <= 0 || r.IndexDigest == "" {
			return fmt.Errorf("ruby qualification run %q is not passing", r.Runner)
		}
		found = found || r.Runner == runner
	}
	if !found {
		return fmt.Errorf("ruby qualification names no runner %q", runner)
	}
	return q.bound(ctx, selectorRoot, RubyQualifiedSources, rev)
}

// passing reports whether q is a passing record of family.
func (q Qualified) passing(family string) bool {
	return q.Profile == "linecov-qualified/1" && q.Family == family && q.Report.Misses == 0 && q.Report.Mutants > 0 && q.Report.Killed > 0 && q.Report.Controls > 0 && q.Report.Digest != ""
}

// cleanRevision is the VCS revision of a clean selector executable.
func cleanRevision(info *debug.BuildInfo) (string, error) {
	rev, clean := "", false
	if info != nil {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
			if s.Key == "vcs.modified" {
				clean = s.Value == "false"
			}
		}
	}
	if rev == "" || !clean {
		return "", fmt.Errorf("selector executable has no clean VCS provenance")
	}
	return rev, nil
}

// bound checks every source of sources against q at each revision.
func (q Qualified) bound(ctx context.Context, root string, sources []string, revs ...string) error {
	for _, at := range revs {
		oids, err := OIDs(ctx, root, at)
		if err != nil {
			return fmt.Errorf("qualification sources unavailable at %s: %w", at, err)
		}
		for _, p := range sources {
			if oids[p] == "" || q.Sources[p] != oids[p] {
				return fmt.Errorf("qualification source mismatch: %s at %s", p, at)
			}
		}
	}
	return nil
}
