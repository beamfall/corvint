// SPDX-License-Identifier: Apache-2.0
// Package testevidencedoc projects existing RSP records without collecting,
// admitting, or persisting evidence (RSP-V0-051).
package testevidencedoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const Schema = "corvint-test-evidence-query/1"
const MaxResultBytes = 256 << 10

type Query struct {
	Path     string `json:"path,omitempty"`
	Start    int32  `json:"start,omitempty"`
	End      int32  `json:"end,omitempty"`
	Test     string `json:"test,omitempty"`
	Explain  string `json:"explain,omitempty"`
	Revision string `json:"revision"`
	Base     string `json:"base,omitempty"`
	Index    string `json:"index,omitempty"`
}

type Match struct {
	Test         string `json:"test"`
	Component    string `json:"component"`
	RecordDigest string `json:"record_digest"`
	RecordCommit string `json:"record_commit"`
}

type RecordView struct {
	Test            string              `json:"test"`
	RecordDigest    string              `json:"record_digest"`
	RecordCommit    string              `json:"record_commit"`
	Status          string              `json:"status"`
	Reason          string              `json:"reason,omitempty"`
	FreshnessReason string              `json:"freshness_reason,omitempty"`
	Record          testevidence.Record `json:"record"`
}

type Document struct {
	Schema      string                `json:"schema"`
	Query       Query                 `json:"query"`
	Status      string                `json:"status"`
	Reasons     []string              `json:"reasons"`
	Matches     []Match               `json:"matches"`
	Records     []RecordView          `json:"records"`
	Explanation *runnerselection.Plan `json:"explanation,omitempty"`
}

func Unknown(q Query, reason string) Document {
	return Document{Schema: Schema, Query: q, Status: "UNKNOWN", Reasons: []string{reason}, Matches: []Match{}, Records: []RecordView{}}
}

func validText(s string) bool {
	return s != "" && utf8.ValidString(s) && len(s) <= 4096 && !strings.ContainsFunc(s, unicode.IsControl)
}
func validPath(s string) bool {
	return validText(s) && s != "." && s != ".." && path.Clean(s) == s && !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\:") && !slices.Contains(strings.Split(s, "/"), ".git")
}
func (q Query) Validate() error {
	n := 0
	for _, s := range []string{q.Path, q.Test, q.Explain} {
		if s != "" {
			n++
			if !validText(s) {
				return errors.New("invalid query")
			}
		}
	}
	if n != 1 || q.Path != "" && !validPath(q.Path) || q.Index != "" && !validPath(q.Index) || !validText(q.Revision) || strings.HasPrefix(q.Revision, "-") || q.Base != "" && (!validText(q.Base) || strings.HasPrefix(q.Base, "-")) || (q.Explain != "") != (q.Base != "") || (q.Start != 0 || q.End != 0) && (q.Path == "" || q.Start < 1 || q.End < q.Start) {
		return errors.New("invalid query")
	}
	return nil
}

// Parse accepts flags before or after the single path operand. No root, runner,
// inventory or qualification can be supplied through the query surface.
func Parse(args []string) (q Query, format string, err error) {
	q.Revision = "HEAD"
	format = "json"
	seen := map[string]bool{}
	positional := false
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s == "--" && !positional {
			positional = true
			continue
		}
		if positional || !strings.HasPrefix(s, "-") {
			if q.Path != "" {
				return q, format, errors.New("expected one path")
			}
			q.Path = s
			continue
		}
		name, value, inline := strings.Cut(s, "=")
		if seen[name] {
			return q, format, errors.New("repeated option")
		}
		seen[name] = true
		if !inline {
			i++
			if i == len(args) {
				return q, format, errors.New("missing option value")
			}
			value = args[i]
		}
		if value == "" {
			return q, format, errors.New("empty option value")
		}
		switch name {
		case "--revision":
			q.Revision = value
		case "--base":
			q.Base = value
		case "--test":
			q.Test = value
		case "--explain":
			q.Explain = value
		case "--index":
			q.Index = value
		case "--format":
			format = value
		case "--lines":
			a, b, ok := strings.Cut(value, ":")
			x, e1 := strconv.ParseInt(a, 10, 32)
			y, e2 := strconv.ParseInt(b, 10, 32)
			if !ok || e1 != nil || e2 != nil || x < 1 || y < x {
				return q, format, errors.New("invalid line range")
			}
			q.Start = int32(x)
			q.End = int32(y)
		default:
			return q, format, errors.New("unknown option")
		}
	}
	if format != "json" && format != "text" {
		return q, format, errors.New("invalid format")
	}
	return q, format, q.Validate()
}

// Project consumes the selector's existing core inputs. Callers must independently
// resolve expected identities; record identities never create a run context.
func Project(q Query, in runnerselection.Input, records []testevidence.Record, loadReason string) Document {
	d := Unknown(q, "inventory-unavailable")
	if q.Validate() != nil {
		return Unknown(q, "invalid-query")
	}
	if len(records) > runnerselection.MaxInputs {
		return Unknown(q, "record-cap-exceeded")
	}
	known := len(in.Runners) > 0
	for _, r := range in.Runners {
		key := testrunner.InventoryKey{Checkout: in.Head, Runner: r.Name, ConfigDigest: r.Identity.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
		if _, err := testrunner.CanonicalInventory(r.Inventory, key); err != nil {
			known = false
		}
	}
	if known {
		d.Status = "OBSERVED"
		d.Reasons = []string{}
	}
	add := func(reason string) {
		d.Status = "UNKNOWN"
		if !slices.Contains(d.Reasons, reason) {
			d.Reasons = append(d.Reasons, reason)
		}
	}
	if loadReason != "" {
		add(loadReason)
		in.DiffComplete, in.DiffReason = false, loadReason
	}
	fresh := 0
	freshTests := map[string]bool{}
	for _, r := range records {
		if q.Test != "" && r.Test != q.Test || q.Explain != "" && r.Test != q.Explain {
			continue
		}
		data, err := json.Marshal(r)
		if err != nil {
			add("malformed-record")
			continue
		}
		digest := sha256.Sum256(data)
		v := RecordView{Test: r.Test, RecordDigest: hex.EncodeToString(digest[:]), RecordCommit: r.Commit, Status: "UNKNOWN", Record: r}
		var expected *runnerselection.Runner
		for i := range in.Runners {
			rr := &in.Runners[i]
			if rr.Name == r.Runner.Plugin && rr.Identity.ExecutableDigest != "" && rr.Execution.EnvironmentDigest != "" {
				expected = rr
				break
			}
		}
		switch {
		case r.Runner.Plugin == "" || r.Execution.OS == "":
			v.Reason = "identity-missing"
		case r.Validate() != nil:
			v.Reason = "malformed-record"
		case expected == nil:
			v.Reason = "run-context-unresolved"
		default:
			f := r.Fresh(expected.Identity, expected.Execution, in.Snapshot)
			if !f.Fresh {
				v.Reason = "stale-record"
				v.FreshnessReason = f.Reason
			} else if !known || !slices.ContainsFunc(expected.Inventory.Tests, func(t testrunner.InventoryTest) bool { return t.ID == r.Test }) {
				v.Reason = "inventory-unavailable"
			} else {
				v.Status = "FRESH"
				fresh++
				freshTests[expected.Name+"\x00"+r.Test] = true
			}
		}
		components := reaches(r, q)
		if v.Reason != "" {
			add(v.Reason)
		}
		if q.Path != "" && len(r.MissingSelectionComponents()) > 0 {
			add("component-unavailable")
		}
		if q.Path != "" && len(components) == 0 {
			continue
		}
		d.Records = append(d.Records, v)
		if v.Reason != "" {
			add(v.Reason)
			continue
		}
		for _, c := range components {
			d.Matches = append(d.Matches, Match{r.Test, c, v.RecordDigest, r.Commit})
		}
	}
	if fresh == 0 {
		add("no-fresh-record")
	}
	if known {
		for _, runner := range in.Runners {
			for _, test := range runner.Inventory.Tests {
				if q.Test != "" && q.Test != test.ID || q.Explain != "" && q.Explain != test.ID {
					continue
				}
				if !freshTests[runner.Name+"\x00"+test.ID] {
					add("no-fresh-record")
				}
			}
		}
	}
	if q.Explain != "" {
		plan := runnerselection.Select(in)
		plan.LoadingNS = nil
		plan.SelectionNS = nil
		// Keep runner reasons and only the requested test's chain. Empty inventory
		// remains empty; the query identity is never manufactured as inventory.
		for i := range plan.Runners {
			p := &plan.Runners[i]
			p.Selected = slices.DeleteFunc(p.Selected, func(t runnerselection.SelectedTest) bool { return t.Test != q.Explain })
			p.Excluded = slices.DeleteFunc(p.Excluded, func(t runnerselection.ExcludedTest) bool { return t.Test != q.Explain })
			if p.Mode == "full" {
				add("unqualified")
			}
		}
		d.Explanation = &plan
	}
	slices.Sort(d.Reasons)
	slices.SortFunc(d.Records, func(a, b RecordView) int {
		if c := strings.Compare(a.Test, b.Test); c != 0 {
			return c
		}
		return strings.Compare(a.RecordDigest, b.RecordDigest)
	})
	slices.SortFunc(d.Matches, func(a, b Match) int {
		return strings.Compare(a.Test+"\x00"+a.Component+"\x00"+a.RecordDigest, b.Test+"\x00"+b.Component+"\x00"+b.RecordDigest)
	})
	d.Records = slices.CompactFunc(d.Records, func(a, b RecordView) bool { return a.RecordDigest == b.RecordDigest })
	d.Matches = slices.Compact(d.Matches)
	return bounded(d)
}

func reaches(r testevidence.Record, q Query) []string {
	out := []string{}
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	match := func(p testevidence.Path) bool {
		return p.Repository == r.Repository && (p.Lexical == q.Path || p.Resolved == q.Path)
	}
	if q.Path == "" {
		return out
	}
	for _, f := range r.Executed.Items {
		if r.Executed.State != testevidence.Present || !match(f.Blob.Path) {
			continue
		}
		for _, span := range f.Executed {
			end := span.End
			if span.EndColumn == 1 && span.End > span.Start {
				end--
			}
			if q.Start == 0 || span.Start <= q.End && end >= q.Start {
				add("executed")
			}
		}
	}
	// Whole-file and existence dependencies also reach any requested line range.
	for _, c := range []struct {
		name  string
		value testevidence.Component[testevidence.Blob]
	}{{"loaded", r.Loaded}, {"reads", r.Reads}} {
		if c.value.State == testevidence.Present {
			for _, b := range c.value.Items {
				if match(b.Path) {
					add(c.name)
				}
			}
		}
	}
	for _, c := range []struct {
		name  string
		value testevidence.Component[testevidence.Path]
	}{{"listings", r.Listings}, {"lookups", r.Lookups}} {
		if c.value.State == testevidence.Present {
			for _, p := range c.value.Items {
				if match(p) || c.name == "listings" && p.Repository == r.Repository && (p.Lexical == "." || strings.HasPrefix(q.Path, p.Lexical+"/") || p.Resolved == "." || strings.HasPrefix(q.Path, p.Resolved+"/")) {
					add(c.name)
				}
			}
		}
	}
	if r.Writes.State == testevidence.Present {
		for _, w := range r.Writes.Items {
			if match(w.Path) {
				add("writes")
			}
		}
	}
	if r.Execs.State == testevidence.Present {
		for _, e := range r.Execs.Items {
			if match(e.Blob.Path) {
				add("execs")
			}
		}
	}
	slices.Sort(out)
	return out
}

func bounded(d Document) Document {
	b, err := json.Marshal(d)
	if err != nil || len(b) > MaxResultBytes {
		return Unknown(d.Query, "result-cap-exceeded")
	}
	return d
}
func Write(w io.Writer, d Document, format string) error {
	d = bounded(d)
	if format == "json" {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(d)
	}
	var b strings.Builder
	fmt.Fprintln(&b, strings.TrimSpace(d.Status+" "+strings.Join(d.Reasons, ",")))
	for _, m := range d.Matches {
		fmt.Fprintf(&b, "%s %s %s %s\n", m.Test, m.Component, m.RecordDigest, m.RecordCommit)
	}
	for _, r := range d.Records {
		fmt.Fprintf(&b, "%s %s %s %s %s %s\n", r.Test, r.Status, r.Reason, r.FreshnessReason, r.RecordDigest, r.RecordCommit)
		data, _ := json.Marshal(r.Record)
		fmt.Fprintln(&b, string(data))
	}
	if d.Explanation != nil {
		data, _ := json.Marshal(d.Explanation)
		fmt.Fprintln(&b, string(data))
	}
	_, err := io.WriteString(w, b.String())
	return err
}
