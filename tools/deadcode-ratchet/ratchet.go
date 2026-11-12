// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// baselineProfile names the format of baseline.json (DCR-V0-004).
const baselineProfile = "corvint-deadcode-baseline/0"

// config is one GOOS/GOARCH pair the analyzer runs under (DCR-V0-002).
type config struct{ GOOS, GOARCH string }

func (c config) String() string { return c.GOOS + "/" + c.GOARCH }

// file is the name of the per-configuration analyzer output.
func (c config) file() string { return c.GOOS + "-" + c.GOARCH + ".json" }

// configs is the fixed set of six configurations. Both architectures of each
// operating system run because the tree has files constrained to one
// architecture.
var configs = []config{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
}

// identity is a function's package import path and the analyzer's name for
// it ("F", "T.M"). It carries no position, so moving a function inside its
// package does not change it (DCR-V0-003).
type identity struct{ Package, Func string }

func (id identity) String() string { return id.Package + " " + id.Func }

func (id identity) less(o identity) bool {
	if id.Package != o.Package {
		return id.Package < o.Package
	}
	return id.Func < o.Func
}

// The analyzer's -json records (golang.org/x/tools/cmd/deadcode).
type analyzerPkg struct {
	Name  string
	Path  string
	Funcs []analyzerFunc
}

type analyzerFunc struct {
	Name      string
	Position  analyzerPosition
	Generated bool
	Marker    bool
}

type analyzerPosition struct {
	File      string
	Line, Col int
}

// run maps each function one analyzer run reports to its declaration
// position.
type run map[identity]string

// parseRun reads one analyzer -json output. Functions in generated files and
// marker methods are dropped even if the analyzer was asked to keep them.
func parseRun(data []byte) (run, error) {
	var pkgs []analyzerPkg
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&pkgs); err != nil {
		return nil, fmt.Errorf("analyzer output is not a JSON package list: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("analyzer output has data after the package list")
	}
	out := run{}
	for _, p := range pkgs {
		if p.Path == "" {
			return nil, fmt.Errorf("analyzer output has a package with no path")
		}
		for _, f := range p.Funcs {
			if f.Name == "" {
				return nil, fmt.Errorf("analyzer output has a function with no name in %s", p.Path)
			}
			if f.Generated || f.Marker {
				continue
			}
			id := identity{p.Path, f.Name}
			if _, dup := out[id]; dup {
				continue
			}
			out[id] = fmt.Sprintf("%s:%d:%d", f.Position.File, f.Position.Line, f.Position.Col)
		}
	}
	return out, nil
}

// intersect returns the identities every one of the six runs reports, with
// the position from the first configuration. It refuses to compute a report
// from fewer than six runs, because a missing run would shrink the report
// (DCR-V0-002).
func intersect(runs map[config]run) (run, error) {
	for _, c := range configs {
		if _, ok := runs[c]; !ok {
			return nil, fmt.Errorf("no analyzer run for %s; refusing to compute a report from fewer than %d runs", c, len(configs))
		}
	}
	for c := range runs {
		if !known(c) {
			return nil, fmt.Errorf("analyzer run for unknown configuration %s", c)
		}
	}
	out := run{}
	for id, pos := range runs[configs[0]] {
		all := true
		for _, c := range configs[1:] {
			if _, ok := runs[c][id]; !ok {
				all = false
				break
			}
		}
		if all {
			out[id] = pos
		}
	}
	return out, nil
}

func known(c config) bool {
	for _, k := range configs {
		if k == c {
			return true
		}
	}
	return false
}

type baselineEntry struct {
	Package string `json:"package"`
	Func    string `json:"func"`
	Reason  string `json:"reason"`
}

func (e baselineEntry) id() identity { return identity{e.Package, e.Func} }

type baselineFile struct {
	Profile   string          `json:"profile"`
	Functions []baselineEntry `json:"functions"`
}

// decodeBaseline reads baseline.json strictly: an unknown field, a wrong
// profile, an empty package or function, a duplicate or an entry out of
// order is refused (DCR-V0-004).
func decodeBaseline(data []byte) ([]baselineEntry, error) {
	var b baselineFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("baseline: data after the top-level object")
	}
	if b.Profile != baselineProfile {
		return nil, fmt.Errorf("baseline: profile %q, want %q", b.Profile, baselineProfile)
	}
	if b.Functions == nil {
		return nil, fmt.Errorf("baseline: missing functions list")
	}
	for i, e := range b.Functions {
		if e.Package == "" || e.Func == "" {
			return nil, fmt.Errorf("baseline: entry %d has an empty package or func", i+1)
		}
		if i == 0 {
			continue
		}
		prev := b.Functions[i-1].id()
		switch {
		case prev == e.id():
			return nil, fmt.Errorf("baseline: duplicate entry %d: %s", i+1, e.id())
		case e.id().less(prev):
			return nil, fmt.Errorf("baseline: entry %d is out of order: %s sorts before %s; sort by package then func", i+1, e.id(), prev)
		}
	}
	return b.Functions, nil
}

// encodeBaseline writes the report as a baseline, one entry per line,
// keeping the reason of each entry that is still reported.
func encodeBaseline(report run, old []baselineEntry) []byte {
	reasons := map[identity]string{}
	for _, e := range old {
		reasons[e.id()] = e.Reason
	}
	ids := sortedIDs(report)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{\n  \"profile\": %q,\n  \"functions\": [", baselineProfile)
	for i, id := range ids {
		line, err := json.Marshal(baselineEntry{id.Package, id.Func, reasons[id]})
		if err != nil {
			panic(err) // strings always marshal
		}
		sep := ","
		if i == len(ids)-1 {
			sep = ""
		}
		fmt.Fprintf(&buf, "\n    %s%s", line, sep)
	}
	if len(ids) > 0 {
		buf.WriteString("\n  ")
	}
	buf.WriteString("]\n}\n")
	return buf.Bytes()
}

// verdict compares the report with the baseline. The baseline is
// shrink-only: a reported function it lacks and an entry the report lacks
// both fail (DCR-V0-005). The returned lines are sorted.
func verdict(report run, base []baselineEntry, baselinePath string) []string {
	inBase := map[identity]bool{}
	for _, e := range base {
		inBase[e.id()] = true
	}
	var out []string
	for _, id := range sortedIDs(report) {
		if !inBase[id] {
			out = append(out, fmt.Sprintf("NEW_DEAD_FUNCTION %s.%s at %s: no binary or test reaches it in any of the %d configurations; delete it or give it a caller (a new baseline entry needs review)",
				id.Package, id.Func, report[id], len(configs)))
		}
	}
	for _, e := range base {
		if _, ok := report[e.id()]; !ok {
			out = append(out, fmt.Sprintf("STALE_BASELINE_ENTRY %s.%s: no longer reported; delete its entry from %s and lower the asserted count in baseline_test.go",
				e.Package, e.Func, baselinePath))
		}
	}
	return out
}

func sortedIDs(r run) []identity {
	ids := make([]identity, 0, len(r))
	for id := range r {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].less(ids[j]) })
	return ids
}

// validVersion accepts only an exact module version: no "latest", branch,
// query or empty string (DCR-V0-001).
func validVersion(v string) bool {
	if !strings.HasPrefix(v, "v") {
		return false
	}
	core, _, _ := strings.Cut(v[1:], "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return false
		}
	}
	return !strings.ContainsAny(v, " <>=@/")
}
