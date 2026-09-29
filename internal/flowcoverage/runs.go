package flowcoverage

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"reflect"
	"sort"
	"strings"
)

type boundRun struct {
	inputs  map[string]string
	binding Run
	receipt jstestprovider.Receipt
	reasons []string
}

func testKey(t ExactTest) string {
	p := "<missing>"
	if t.Project != nil {
		p = *t.Project
	}
	return t.File + "\x00" + t.FullTitle + "\x00" + p
}
func (s *source) loadRuns(in Runs) ([]boundRun, error) {
	names, e := s.files(in.Revision, in.Directory)
	if e != nil {
		return nil, e
	}
	wanted := map[string]bool{}
	for _, p := range names {
		if strings.HasSuffix(p, ".json") {
			wanted[p] = true
		}
	}
	if len(in.Runs) > 128 || len(wanted) != len(in.Runs) {
		return nil, fmt.Errorf("receipt directory inventory mismatch")
	}
	out := []boundRun{}
	total := 0
	for _, b := range in.Runs {
		if !wanted[b.Path] || b.Revision != in.Revision || (b.Kind != "planned" && b.Kind != "manual") {
			return nil, fmt.Errorf("unknown/duplicate receipt or run kind")
		}
		delete(wanted, b.Path)
		raw, e := s.ref(b.Ref, 4<<20)
		if e != nil {
			return nil, e
		}
		total += len(raw)
		if total > 64<<20 {
			return nil, fmt.Errorf("aggregate receipt bound exceeded")
		}
		r, e := jstestprovider.DecodeAttemptReceipt(raw)
		if e != nil {
			return nil, e
		}
		if _, e = s.auth.CommitTree(s.ctx, b.SourceRevision); e != nil {
			return nil, e
		}
		if _, e = s.auth.CommitTree(s.ctx, b.TestRevision); e != nil {
			return nil, e
		}
		bound := boundRun{binding: b, receipt: r, reasons: []string{}, inputs: map[string]string{}}
		used := map[string]bool{}
		check := func(label, hash string) error {
			p, ok := b.Paths[label]
			if !ok {
				return fmt.Errorf("missing exact reporter path mapping")
			}
			data, _, e := s.blob(b.TestRevision, p, 4<<20)
			if e != nil {
				return e
			}
			if hash == "" || digest(data) != hash {
				return fmt.Errorf("reporter digest differs from immutable bytes")
			}
			used[label] = true
			bound.inputs[p] = hash
			if !s.unchanged(b.TestRevision, p) {
				bound.reasons = append(bound.reasons, "stale: test/config/package "+p)
			}
			return nil
		}
		if e = check(r.Identity.ConfigFile, r.Identity.ConfigDigest); e != nil {
			return nil, e
		}
		for p, h := range r.Identity.ConfigInputDigests {
			if e = check(p, h); e != nil {
				return nil, e
			}
		}
		for p, h := range r.Identity.TestFileDigests {
			if e = check(p, h); e != nil {
				return nil, e
			}
		}
		if len(b.PackagePaths) != 2 {
			bound.reasons = append(bound.reasons, "package and lock binding unavailable")
		} else {
			var combined strings.Builder
			for _, label := range b.PackagePaths {
				p, ok := b.Paths[label]
				if !ok {
					return nil, fmt.Errorf("missing package path mapping")
				}
				data, _, e := s.blob(b.TestRevision, p, 4<<20)
				if e != nil {
					return nil, e
				}
				h := digest(data)
				if e = check(label, h); e != nil {
					return nil, e
				}
				fmt.Fprintf(&combined, "%s=%s\n", label, h)
			}
			if digest([]byte(combined.String())) != r.Identity.PackageDigest {
				return nil, fmt.Errorf("package identity mismatch")
			}
		}
		if len(used) != len(b.Paths) {
			return nil, fmt.Errorf("unused reporter path mapping")
		}
		if !reflect.DeepEqual(r.Identity.Environment, b.Environment) {
			return nil, fmt.Errorf("execution environment differs from binding")
		}
		if b.BuildScope == "" || b.ApplicationIdentity == "" {
			bound.reasons = append(bound.reasons, "committed served-build provenance missing")
		} else {
			h, fresh, e := s.build(b.SourceRevision, b.BuildScope)
			if e != nil {
				return nil, e
			}
			if !fresh {
				bound.reasons = append(bound.reasons, "stale: served-build scope")
			}
			if h == "" || r.AppBuildAtStart.Unknown || r.AppBuildAtPublish.Unknown || r.AppBuildAtStart.Digest != h || r.AppBuildAtPublish.Digest != h || r.External == nil || r.External.DeclaredAppIdentity != b.ApplicationIdentity {
				bound.reasons = append(bound.reasons, "served-build/application byte binding missing")
			}
		}
		seenTests := map[string]bool{}
		seenIDs := map[string]bool{}
		if len(r.Tests) > 8192 {
			return nil, fmt.Errorf("receipt test bound exceeded")
		}
		for _, t := range r.Tests {
			if t.Anchor == nil || t.Project == nil || t.ID == "" {
				continue
			}
			key := t.Anchor.File + "\x00" + t.FullName + "\x00" + t.Project.Name
			if seenTests[key] || seenIDs[t.ID] {
				return nil, fmt.Errorf("ambiguous duplicate test identity")
			}
			seenTests[key] = true
			seenIDs[t.ID] = true
		}
		// Every start must join one retained test and attempt, not merely the selected favorable test.
		if r.Schedule == nil {
			bound.reasons = append(bound.reasons, "reporter schedule missing")
		} else {
			counts := map[string]int{}
			for _, t := range r.Tests {
				if t.Anchor == nil || t.Project == nil {
					continue
				}
				key := t.Anchor.File + "\x00" + t.FullName + "\x00" + t.Project.Name
				for _, a := range t.Attempts {
					counts[fmt.Sprintf("%s\x00%d", key, a.Retry)]++
				}
			}
			starts := map[string]int{}
			for _, x := range r.Schedule.Starts {
				key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", x.File, x.FullName, x.Project, x.Retry)
				starts[key]++
				if x.Retries < 0 || x.Retry < 0 || counts[key] != 1 {
					bound.reasons = append(bound.reasons, "ambiguous or unjoined schedule")
				}
			}
			if !reflect.DeepEqual(counts, starts) {
				bound.reasons = append(bound.reasons, "schedule does not cover all attempts exactly")
			}
		}
		sort.Strings(bound.reasons)
		out = append(out, bound)
	}
	return out, nil
}
func (s *source) observe(exact ExactTest, runs []boundRun) []Observation {
	out := []Observation{}
	for _, b := range runs {
		for _, t := range b.receipt.Tests {
			if t.Anchor == nil || t.Project == nil || exact.Project == nil || b.binding.Paths[t.Anchor.File] != exact.File || t.FullName != exact.FullTitle || t.Project.Name != *exact.Project {
				continue
			}
			o := Observation{Receipt: b.binding.SHA256, Kind: b.binding.Kind, Test: exact, State: string(t.State), Attempts: len(t.Attempts), Reasons: append([]string{}, b.reasons...), ConfiguredRetries: []int{}, SourceRevision: b.binding.SourceRevision}
			// Commit and caller-label changes preserve comparable history when the
			// actual bound bytes remain identical. Proof pairing still requires
			// the same declared app identity; history cannot be reset by relabeling it.
			identity := struct {
				Build, Config, Node, Runner string
				Inputs, Environment         map[string]string
				Project                     *jstestprovider.ProjectIdentity
			}{b.receipt.AppBuildAtStart.Digest, b.binding.Paths[b.receipt.Identity.ConfigFile], b.receipt.Identity.NodeVersion, b.receipt.Identity.RunnerVersion, b.inputs, b.receipt.Identity.Environment, t.Project}
			history, _ := Encode(identity)
			o.HistoryGroup = digest(history)
			group, _ := Encode(struct{ History, Application string }{o.HistoryGroup, b.binding.ApplicationIdentity})
			o.BindingGroup = digest(group)

			if t.Anchor.Line != exact.Anchor.Start || b.receipt.Identity.TestFileDigests[t.Anchor.File] != exact.Anchor.SHA256 {
				o.Reasons = append(o.Reasons, "stale: test declaration/digest does not match accepted anchor")
			}
			if !jstestprovider.QualifiedReceiptBindingReady(b.receipt, t) {
				o.Reasons = append(o.Reasons, "provider lifecycle or identity unqualified")
			}
			if len(t.Attempts) != 1 || t.Retries != 0 {
				o.Reasons = append(o.Reasons, "actual retries present")
			}
			for _, a := range t.Attempts {
				if a.Retry != 0 || (a.State != jstestprovider.StatePassed && a.State != jstestprovider.StateFailed) || a.State == jstestprovider.StateFailed && a.FailureKind != "assertion-or-test" {
					o.Reasons = append(o.Reasons, "nonassertion/infrastructure attempt cannot qualify")
				}
			}
			starts := 0
			if b.receipt.Schedule != nil {
				for _, x := range b.receipt.Schedule.Starts {
					if x.File == t.Anchor.File && x.FullName == t.FullName && x.Project == t.Project.Name {
						starts++
						o.ConfiguredRetries = append(o.ConfiguredRetries, x.Retries)
						if x.Retry != 0 || x.Retries != 0 || x.Line != t.Anchor.Line {
							o.Reasons = append(o.Reasons, "configured retries or schedule location unqualified")
						}
					}
				}
			}
			if starts != 1 {
				o.Reasons = append(o.Reasons, "exactly one retry-zero start required")
			}
			for _, why := range o.Reasons {
				if strings.HasPrefix(why, "stale:") {
					o.Historical = true
				}
			}
			sort.Strings(o.Reasons)
			o.Qualified = len(o.Reasons) == 0
			out = append(out, o)
		}
	}
	return out
}
