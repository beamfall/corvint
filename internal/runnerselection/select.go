// SPDX-License-Identifier: Apache-2.0
package runnerselection

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// Select only reports a shadow. Even a fully qualified synthetic input cannot
// turn this result into a production run set (RSP-V0-040 step 1).
func Select(in Input) Plan {
	start := time.Now()
	out := Plan{Shadow: true, GoverningPlan: "static-unqualified", Soundness: shadowClaim, Runners: []RunnerPlan{}}
	if in.LoadingNS != nil && *in.LoadingNS >= 0 {
		n := *in.LoadingNS
		out.LoadingNS = &n
	}
	seen := map[string]bool{}
	duplicates := map[string]bool{}
	for _, r := range in.Runners {
		if seen[r.Name] {
			duplicates[r.Name] = true
		}
		seen[r.Name] = true
	}
	for _, r := range in.Runners {
		reason := ""
		if duplicates[r.Name] {
			reason = "runner-duplicate"
		}
		if len(in.Runners) > MaxInputs {
			reason = "runner-cap-exceeded"
		}
		out.Runners = append(out.Runners, selectRunner(in, r, reason))
	}
	sort.SliceStable(out.Runners, func(i, j int) bool { return out.Runners[i].Runner < out.Runners[j].Runner })
	elapsed := time.Since(start).Nanoseconds()
	out.SelectionNS = &elapsed
	return out
}

func selectRunner(in Input, r Runner, initial string) RunnerPlan {
	p := RunnerPlan{Runner: r.Name, Mode: "selected", ExecutionModel: r.Qualification.Model,
		Reasons: []string{}, Changes: []PathClass{}, Selected: []SelectedTest{}, Excluded: []ExcludedTest{}}
	full := func(reason string) {
		if reason != "" {
			p.Mode = "full"
			p.Reasons = append(p.Reasons, reason)
		}
	}
	full(initial)
	if r.Name == "" {
		full("runner-identity-missing")
	}
	if !in.DiffComplete {
		full("diff-incomplete:" + in.DiffReason)
	}
	if !oid(in.Base) || !oid(in.Head) || !in.Repositories[in.Repository] {
		full("comparison-identity-unverifiable")
	}
	if len(in.Changes) > MaxInputs || len(r.Records) > MaxInputs || len(r.Mandates) > MaxInputs || len(r.ConfigurationPaths) > MaxInputs || len(r.ServerPaths) > MaxInputs {
		full("input-cap-exceeded")
	}
	full(r.Pieces.FullReason(r.Qualification.Passed))
	if !r.Qualification.Admitted {
		full("qualification-not-admitted")
	}
	if !r.Qualification.SamplingEnabled {
		full("sampling-disabled")
	}
	if r.Qualification.Model != "traced" {
		full("execution-model-unsupported")
	}
	for _, class := range r.Qualification.Classes {
		if !slices.Contains([]Class{BodyEdit, DeclarationEdit, DataEdit, Add, DeleteRename, ExecTarget, RunnerConfig, E2EServer}, class) {
			full("qualification-class-unsupported:" + string(class))
		}
	}
	if r.Qualification.Runner != r.Identity || r.Qualification.Execution != r.Execution {
		full("execution-configuration-unqualified")
	}
	if r.PluginFailure != "" {
		full("plugin-failed:" + r.PluginFailure)
	}
	if r.InventoryDivergent {
		full("inventory-divergent")
	}
	if r.ConfigurationChanged {
		full("configuration-changed")
	}
	if r.Disabled {
		full("confirmed-miss-disabled")
	}
	if !r.IndexVerified || r.IndexBase != in.Base {
		full("index-missing-stale-or-unverifiable")
	}
	key := testrunner.InventoryKey{Checkout: in.Head, Runner: r.Name, ConfigDigest: r.Identity.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	inv, err := testrunner.CanonicalInventory(r.Inventory, key)
	if err != nil {
		full(err.Error())
	} else {
		p.Counts.InventoryKnown = true
		p.Counts.Inventoried = len(inv.Tests)
	}
	records := map[string]testevidence.Record{}
	for _, e := range r.Records {
		if !e.Admitted {
			full("evidence-not-admitted")
		}
		if _, ok := records[e.Record.Test]; ok {
			full("record-duplicate:" + e.Record.Test)
		}
		records[e.Record.Test] = e.Record
	}
	known := map[string]bool{}
	for _, t := range inv.Tests {
		known[t.ID] = true
	}
	mandates := map[string][]string{}
	for _, m := range r.Mandates {
		if !known[m.Test] || strings.TrimSpace(m.Reason) == "" {
			full("mandate-invalid:" + m.Test)
		}
		if len(in.Changes) > 0 {
			mandates[m.Test] = append(mandates[m.Test], m.Reason)
		}
	}
	for _, k := range append(append([]testevidence.PathKey{}, r.ConfigurationPaths...), r.ServerPaths...) {
		if !validPath(in, k.Repository, k.Path) {
			full("configuration-path-unverifiable")
		}
	}
	changes := expand(in.Changes)
	for _, c := range changes {
		class := Class("unknown")
		if !validChange(in, c) {
			full("change-path-or-range-unverifiable")
		} else {
			class = classify(c, r)
		}
		p.Changes = append(p.Changes, PathClass{c.Repository, c.Path, class})
		if class == RunnerConfig {
			full("rule:a")
			full("configuration-changed")
		}
		if !slices.Contains(r.Qualification.Classes, class) {
			full("class-unqualified:" + string(class))
		}
	}
	sort.Slice(p.Changes, func(i, j int) bool {
		a, b := p.Changes[i], p.Changes[j]
		if a.Repository != b.Repository {
			return a.Repository < b.Repository
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Class < b.Class
	})
	p.Reasons = unique(p.Reasons)
	for _, t := range inv.Tests {
		rules := []string{}
		record, ok := records[t.ID]
		if !ok {
			rules = append(rules, "record-missing")
		} else {
			rules = append(rules, componentReasons(record)...)
			if record.Nondeterministic {
				rules = append(rules, "nondeterministic")
			}
			if record.Repository != in.Repository || record.Commit != in.Base {
				rules = append(rules, "record-base-or-repository-mismatch")
			}
			if fresh := record.Fresh(r.Identity, r.Execution, in.Snapshot); !fresh.Fresh {
				rules = append(rules, "record-stale:"+fresh.Reason)
			}
			if !pathsKnown(in, record) {
				rules = append(rules, "ancestry-unproven")
			}
			for _, c := range changes {
				rules = append(rules, rulesFor(in, record, c)...)
			}
		}
		if len(mandates[t.ID]) > 0 {
			rules = append(rules, "project-mandated")
		}
		if p.Mode == "full" {
			rules = append(rules, p.Reasons...)
		}
		rules = unique(rules)
		if len(rules) > 0 {
			p.Selected = append(p.Selected, SelectedTest{t.ID, rules, unique(mandates[t.ID])})
			if slices.Contains(rules, "rule:b") {
				p.Counts.Coverage++
			} else {
				p.Counts.Fallback++
			}
		} else {
			data, err := testevidence.Encode(record)
			if err != nil {
				p.Selected = append(p.Selected, SelectedTest{Test: t.ID, Rules: []string{"record-malformed:" + err.Error()}})
				p.Counts.Fallback++
				continue
			}
			// RSP-V0-055: freshness and admission alone do not establish
			// repeatability. Only admitted qualification can waive recollection.
			if ready, reason := record.CollectionReady(r.Qualification.SingleCollectionQualified); !ready {
				p.Selected = append(p.Selected, SelectedTest{Test: t.ID, Rules: []string{reason}})
				p.Counts.Fallback++
				continue
			}
			digest := sha256.Sum256(data)
			p.Excluded = append(p.Excluded, ExcludedTest{t.ID, hex.EncodeToString(digest[:]), record.SelectionComponents(), append([]PathClass{}, p.Changes...), invalidation})
		}
	}
	p.Counts.Selected, p.Counts.Excluded = len(p.Selected), len(p.Excluded)
	return p
}

func oid(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && (len(b) == 20 || len(b) == 32) && strings.ToLower(s) == s
}
func unique(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return slices.Compact(out)
}
func components() []string { return (testevidence.Record{}).SelectionComponents() }

func componentReasons(r testevidence.Record) []string {
	var reasons []string
	for _, name := range r.MissingSelectionComponents() {
		reasons = append(reasons, "component-missing:"+name)
	}
	if len(r.External.Items) > 0 {
		reasons = append(reasons, "external-observed")
	}
	return reasons
}

func validPath(in Input, repo, p string) bool {
	return in.Repositories[repo] && utf8.ValidString(p) && p != "" && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/") && !strings.ContainsAny(p, "\\\x00:\"\n\r\t") && strings.IndexFunc(p, unicode.IsControl) < 0
}
func validChange(in Input, c Change) bool {
	if !c.Decoded || !validPath(in, c.Repository, c.Path) || c.Path == "." || len(c.Hunks) > MaxInputs {
		return false
	}
	if c.Kind != "modify" && c.Kind != "add" && c.Kind != "delete" {
		return false
	}
	for _, h := range c.Hunks {
		if h.Start < 0 || h.Start == 2147483647 || h.End < h.Start-1 || h.End < -1 || h.End == 2147483647 || (h.Start == 0 && h.End >= 0) {
			return false
		}
	}
	return true
}
func expand(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		if c.Kind == "rename" {
			old := c
			old.Kind, old.Path, old.OldPath = "delete", c.OldPath, ""
			out = append(out, old)
			c.Kind, c.OldPath = "add", ""
		}
		out = append(out, c)
	}
	return out
}

func matches(p testevidence.Path, c Change) bool {
	if p.Repository != c.Repository {
		return false
	}
	if p.Lexical == c.Path || p.Resolved == c.Path {
		return true
	}
	// A changed link may be an intermediate directory in either recorded
	// name. Its target change affects descendants even without child diffs.
	return (c.ModeChanged || c.LinkChanged) && (strings.HasPrefix(p.Lexical, c.Path+"/") || strings.HasPrefix(p.Resolved, c.Path+"/"))
}
func keyMatches(p testevidence.PathKey, c Change) bool {
	return p.Repository == c.Repository && p.Path == c.Path
}
func classify(c Change, r Runner) Class {
	for _, p := range r.ConfigurationPaths {
		if keyMatches(p, c) {
			return RunnerConfig
		}
	}
	for _, p := range r.ServerPaths {
		if keyMatches(p, c) {
			return E2EServer
		}
	}
	if c.Kind == "add" {
		return Add
	}
	if c.Kind == "delete" {
		return DeleteRename
	}
	for _, e := range r.Records {
		for _, x := range e.Record.Execs.Items {
			if matches(x.Blob.Path, c) {
				return ExecTarget
			}
		}
	}
	code, body := false, true
	for _, e := range r.Records {
		for _, f := range e.Record.Executed.Items {
			if matches(f.Blob.Path, c) {
				code = true
				body = body && bodyPrecondition(e.Record, f, c)
			}
		}
		for _, b := range e.Record.Loaded.Items {
			if matches(b.Path, c) {
				code = true
				if !hasExecuted(e.Record, c) {
					body = false
				}
			}
		}
	}
	if code {
		if body {
			return BodyEdit
		}
		return DeclarationEdit
	}
	return DataEdit
}
func hasExecuted(r testevidence.Record, c Change) bool {
	for _, f := range r.Executed.Items {
		if matches(f.Blob.Path, c) {
			return true
		}
	}
	return false
}

func pathsKnown(in Input, r testevidence.Record) bool {
	check := func(p testevidence.Path) bool {
		return validPath(in, p.Repository, p.Lexical) && validPath(in, p.Repository, p.Resolved) && !in.Snapshot.ChangedPins[p.Repository]
	}
	for _, f := range r.Executed.Items {
		if !check(f.Blob.Path) {
			return false
		}
	}
	for _, f := range r.Regions.Items {
		if !check(f.Blob.Path) {
			return false
		}
	}
	for _, bs := range [][]testevidence.Blob{r.Loaded.Items, r.Reads.Items} {
		for _, b := range bs {
			if !check(b.Path) {
				return false
			}
		}
	}
	for _, ps := range [][]testevidence.Path{r.Listings.Items, r.Lookups.Items} {
		for _, p := range ps {
			if !check(p) {
				return false
			}
		}
	}
	for _, e := range r.Execs.Items {
		if !check(e.Blob.Path) {
			return false
		}
	}
	return true
}

func rulesFor(in Input, r testevidence.Record, c Change) []string {
	if !validChange(in, c) {
		return []string{"ancestry-unproven"}
	}
	var rules []string
	coarse := false
	for _, f := range r.Executed.Items {
		if !matches(f.Blob.Path, c) {
			continue
		}
		if bodyPrecondition(r, f, c) {
			if intersects(r, f, c) {
				rules = append(rules, "rule:b")
			}
		} else {
			coarse = true
		}
	}
	for _, b := range r.Loaded.Items {
		if matches(b.Path, c) {
			body := false
			for _, f := range r.Executed.Items {
				if matches(f.Blob.Path, c) && bodyPrecondition(r, f, c) {
					body = true
				}
			}
			if !body {
				coarse = true
			}
		}
	}
	if coarse {
		rules = append(rules, "rule:c")
	}
	read := false
	for _, b := range r.Reads.Items {
		read = read || matches(b.Path, c)
	}
	for _, e := range r.Execs.Items {
		read = read || matches(e.Blob.Path, c)
	}
	if read {
		rules = append(rules, "rule:d")
	}
	if c.Kind == "add" {
		if !in.DirectoriesComplete || !in.BaseDirectories[testevidence.PathKey{Repository: c.Repository, Path: "."}] {
			rules = append(rules, "ancestry-unproven")
		} else {
			ancestor := path.Dir(c.Path)
			missing := []string{c.Path}
			for !in.BaseDirectories[testevidence.PathKey{Repository: c.Repository, Path: ancestor}] && ancestor != "." {
				missing = append(missing, ancestor)
				ancestor = path.Dir(ancestor)
			}
			for _, p := range r.Listings.Items {
				if p.Repository == c.Repository && (p.Lexical == ancestor || p.Resolved == ancestor) {
					rules = append(rules, "rule:e")
				}
			}
			for _, p := range r.Lookups.Items {
				if p.Repository == c.Repository && (slices.Contains(missing, p.Lexical) || slices.Contains(missing, p.Resolved)) {
					rules = append(rules, "rule:e")
				}
			}
		}
	}
	if c.Kind == "delete" {
		listing := false
		for a := path.Dir(c.Path); ; a = path.Dir(a) {
			for _, p := range r.Listings.Items {
				if p.Repository == c.Repository && (p.Lexical == a || p.Resolved == a) {
					listing = true
				}
			}
			if a == "." {
				break
			}
		}
		if listing || coarse || read {
			rules = append(rules, "rule:f")
		}
	}
	if c.ModeChanged || c.LinkChanged {
		dependent := coarse || read
		// RSP-V0-012(g): directory enumeration observes entry types too.
		// No mode transition is certified invisible to a parent listing.
		for _, p := range r.Listings.Items {
			dependent = dependent || p.Repository == c.Repository && (p.Lexical == path.Dir(c.Path) || p.Resolved == path.Dir(c.Path))
		}
		for _, ps := range [][]testevidence.Path{r.Listings.Items, r.Lookups.Items} {
			for _, p := range ps {
				dependent = dependent || matches(p, c)
			}
		}
		if dependent {
			rules = append(rules, "rule:g")
		}
	}
	return rules
}

func hunkLines(h Hunk) (int32, int32) {
	if h.End < h.Start {
		return max(1, h.Start), h.Start + 1
	}
	return h.Start, h.End
}
func rangeEnd(r testevidence.Range) int32 {
	if r.EndColumn == 1 && r.End > r.Start {
		return r.End - 1
	}
	return r.End
}
func containsLines(r testevidence.Range, start, end int32) bool {
	// A line-only hunk cannot distinguish a signature or trailing declaration
	// from a body on the same line. Certify only strict interior lines.
	return r.Start < start && r.End > end
}
func bodyPrecondition(r testevidence.Record, f testevidence.ExecutedFile, c Change) bool {
	if c.Kind != "modify" || c.ModeChanged || c.LinkChanged || len(c.Hunks) == 0 || r.Executed.State != testevidence.Present || r.Regions.State != testevidence.Present {
		return false
	}
	for _, h := range c.Hunks {
		a, b := hunkLines(h)
		inside := false
		for _, region := range r.Regions.Items {
			if region.Blob == f.Blob {
				for _, body := range region.Bodies {
					inside = inside || containsLines(body.Range, a, b)
				}
			}
		}
		if !inside {
			return false
		}
	}
	return true
}
func intersects(r testevidence.Record, f testevidence.ExecutedFile, c Change) bool {
	for _, h := range c.Hunks {
		a, b := hunkLines(h)
		for _, e := range f.Executed {
			if e.Start <= b && rangeEnd(e) >= a {
				return true
			}
		}
		// LPCV-V0-952: only whole lines inside coverage blocks certify
		// coverage. Boundary columns may leave a case expression or an unknown
		// construct outside a block; those gaps depend on the enclosing body.
		// Merge certified intervals instead of iterating line numbers.
		blocks := append(append([]testevidence.Range{}, f.Executed...), f.Unexecuted...)
		sort.Slice(blocks, func(i, j int) bool {
			if blocks[i].Start != blocks[j].Start {
				return blocks[i].Start < blocks[j].Start
			}
			return blocks[i].StartColumn < blocks[j].StartColumn
		})
		next := a
		for _, block := range blocks {
			start, end := block.Start, block.End-1
			if block.StartColumn != 1 {
				start++
			}
			if start > next {
				continue
			}
			if end >= next {
				next = end + 1
			}
			if next > b {
				break
			}
		}
		if next <= b {
			for _, region := range r.Regions.Items {
				if region.Blob == f.Blob {
					for _, body := range region.Bodies {
						if containsLines(body.Range, a, b) {
							for _, e := range f.Executed {
								if e.Start <= rangeEnd(body.Range) && rangeEnd(e) >= body.Range.Start {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}
