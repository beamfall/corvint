// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"context"
	"fmt"
	"path/filepath"
)

// Collect compares each ordinary-order result with exactly one isolated test
// process and unions both reaches. Missing, duplicate and mismatching outcomes
// preserve whole-file fallback (LPCV-V0-962, LPCV-V0-7605).
func Collect(ctx context.Context, o Options) (*Run, error) {
	run, err := collectOnce(ctx, o)
	if err != nil {
		return nil, err
	}
	run.isolated = true
	records := map[string]*record{}
	for i := range run.records {
		r := &run.records[i]
		records[r.pkg+"\x00"+r.name] = r
	}
	for i := range run.Outcomes {
		out := &run.Outcomes[i]
		if out.Status == "pending" {
			continue
		}
		one := o
		if o.IsolatedTimeout > 0 {
			one.Timeout = o.IsolatedTimeout
		}
		one.Work = filepath.Join(o.Work, fmt.Sprintf("isolated-%d", i))
		one.onlyName, one.onlyPackage = out.Name, out.Package
		isolated, err := collectOnce(ctx, one)
		divergent := err != nil || isolated == nil || len(isolated.Outcomes) != 1
		if !divergent {
			a := isolated.Outcomes[0]
			divergent = a.Package != out.Package || a.Name != out.Name || a.Status != out.Status || a.Recorded != out.Recorded
		}
		if divergent {
			out.Recorded = false
			out.Reason = "isolation-divergent"
			continue
		}
		if !out.Recorded {
			continue
		}
		target := records[out.Package+"\x00"+out.Name]
		if target == nil {
			out.Recorded = false
			out.Reason = "isolation-divergent: missing record"
			continue
		}
		// A test whose ordinary-order reach exceeds its isolated reach depends on
		// earlier tests (shared state): selecting it alone would not reach the
		// changed code, so the file keeps whole-file fallback.
		if !reachCovered(target, isolated.records, out.Package, out.Name) {
			out.Recorded = false
			out.Reason = "isolation-divergent: ordinary-order reach exceeds isolated reach"
			continue
		}
		for _, r := range isolated.records {
			if r.pkg == out.Package && r.name == out.Name {
				for p, ls := range r.lines {
					target.lines[p] = append(target.lines[p], ls...)
				}
				for p, ms := range r.methods {
					target.methods[p] = append(target.methods[p], ms...)
				}
			}
		}
		mergeShapes(run.files, isolated.files)
	}
	complete := map[string]bool{}
	for _, o := range run.Outcomes {
		if o.Recorded {
			complete[o.Package+"\x00"+o.Name] = true
		}
	}
	var kept []record
	for _, r := range run.records {
		if r.pkg == Load || complete[r.pkg+"\x00"+r.name] {
			kept = append(kept, r)
		}
	}
	run.records = kept
	return run, nil
}

// reachCovered reports whether every line and method the ordinary-order record
// reached was also reached by the isolated run of the same test.
func reachCovered(ordinary *record, isolated []record, pkg, name string) bool {
	lines := map[string]map[int32]bool{}
	methods := map[string]map[span]bool{}
	for _, r := range isolated {
		if r.pkg != pkg || r.name != name {
			continue
		}
		for p, ls := range r.lines {
			if lines[p] == nil {
				lines[p] = map[int32]bool{}
			}
			for _, l := range ls {
				lines[p][l] = true
			}
		}
		for p, ms := range r.methods {
			if methods[p] == nil {
				methods[p] = map[span]bool{}
			}
			for _, m := range ms {
				methods[p][m] = true
			}
		}
	}
	for p, ls := range ordinary.lines {
		for _, l := range ls {
			if !lines[p][l] {
				return false
			}
		}
	}
	for p, ms := range ordinary.methods {
		for _, m := range ms {
			if !methods[p][m] {
				return false
			}
		}
	}
	return true
}
