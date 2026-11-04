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
