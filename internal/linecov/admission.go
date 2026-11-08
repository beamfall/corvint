// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
)

var QualifiedSources = []string{
	"cmd/corvint/affected_linecov.go",
	"internal/linecov/admission.go",
	"internal/linecov/audit.go",
	"internal/linecov/collect.go",
	"internal/linecov/compact.go",
	"internal/linecov/decide.go",
	"internal/linecov/envaudit.go",
	"internal/linecov/git.go",
	"internal/linecov/heads.go",
	"internal/linecov/index.go",
	"internal/linecov/lookup.go",
	"internal/linecov/runner.go",
	"internal/linecov/select.go",
	"internal/linecov/pgroup_unix.go",
	"internal/linecov/pgroup_other.go",
	"internal/linecov/segments.go",
	"internal/linecov/qualify.go",
	"internal/linecov/windows.go",
	"tools/linecov/main.go",
	"tools/linecov/qualify.go",
	"tools/linecov/ci.go",
}

type Qualified struct {
	Profile     string            `json:"profile"`
	Family      string            `json:"family"`
	Runner      string            `json:"runner"`
	IndexCommit string            `json:"indexCommit"`
	Report      QualifiedReport   `json:"report"`
	Sources     map[string]string `json:"sources"`
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
	if q.Profile != "linecov-qualified/1" || q.Family != "go" || q.Runner != runner || q.Report.Misses != 0 || q.Report.Mutants <= 0 || q.Report.Killed <= 0 || q.Report.Controls <= 0 || q.Report.Digest == "" {
		return fmt.Errorf("qualification is not a passing record for this runner")
	}
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
		return fmt.Errorf("selector executable has no clean VCS provenance")
	}
	for _, at := range []string{base, rev} {
		oids, err := OIDs(ctx, root, at)
		if err != nil {
			return fmt.Errorf("qualification sources unavailable at %s: %w", at, err)
		}
		for _, p := range QualifiedSources {
			if oids[p] == "" || q.Sources[p] != oids[p] {
				return fmt.Errorf("qualification source mismatch: %s at %s", p, at)
			}
		}
	}
	return nil
}
