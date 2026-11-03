// SPDX-License-Identifier: Apache-2.0

package rubytestprovider

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// The Ruby segment collector (docs/specs/map-coverage-join-v0.md MCJ-V0-013,
// MCJ-V0-041..044): RSpec, Capybara and SitePrism examples split into
// segments by Ruby line Coverage, built into a linecov-index/3 index whose
// blocks are source lines.

// SegmentsFormat names the collector's report.
const SegmentsFormat = "corvint-ruby-segments/0"

// CollectorRSpecSegments names the qualified Ruby collector (MCJ-V0-013).
const CollectorRSpecSegments = SegmentsFormat

// ReasonCollectorMissing is the LPCV-V0-955 reason a family without a
// qualified collector keeps the static plan under.
const ReasonCollectorMissing = "coverage-collector-missing"

// Incomplete-record reasons the collector reports (MCJ-V0-015, MCJ-V0-043).
const (
	ReasonBoundaryMissing    = "ruby-boundary-missing"
	ReasonSubprocess         = "ruby-subprocess-uninstrumented"
	ReasonDriverOutOfProcess = "ruby-driver-out-of-process"
	ReasonCoverageStopped    = "ruby-coverage-stopped"
)

var incompleteReasons = map[string]bool{
	ReasonBoundaryMissing: true, ReasonSubprocess: true, ReasonDriverOutOfProcess: true, ReasonCoverageStopped: true,
}

// maxSegmentReportBytes bounds one collector report.
const maxSegmentReportBytes = 64 << 20

//go:embed corvint_segments.rb
var segmentCollector []byte

// SegmentCollectorSource returns a copy of the collector's Ruby source.
func SegmentCollectorSource() []byte { return append([]byte(nil), segmentCollector...) }

// CollectorStatus names the collector of a Ruby runner family, or
// coverage-collector-missing for a family without a qualified one, which
// keeps the static plan (MCJ-V0-013, LPCV-V0-955). Capybara and SitePrism
// run under RSpec, so they share its collector.
func CollectorStatus(family string) (collector string, qualified bool, reason string) {
	switch strings.ToLower(family) {
	case "rspec", "capybara", "site_prism", "siteprism":
		return CollectorRSpecSegments, true, ""
	}
	return "", false, ReasonCollectorMissing
}

// SegmentReport is the collector's JSON report.
type SegmentReport struct {
	Format   string           `json:"format"`
	Root     string           `json:"root"`
	Ruby     string           `json:"ruby"`
	RSpec    string           `json:"rspec"`
	StepDirs []string         `json:"step_dirs"`
	Files    map[string][]int `json:"files"` // project path -> executable lines
	Examples []SegmentExample `json:"examples"`
}

// SegmentExample is one RSpec example's record.
type SegmentExample struct {
	ID            string                `json:"id"` // ./spec/x_spec.rb[1:2]
	File          string                `json:"file"`
	Line          int                   `json:"line"`
	Type          string                `json:"type"`
	DeclaredLevel string                `json:"declared_level"`
	Status        string                `json:"status"`
	ElapsedNS     int64                 `json:"elapsed_ns"`
	Incomplete    []string              `json:"incomplete"`
	Segments      []SegmentRun          `json:"segments"`
	Residual      map[string][][2]int64 `json:"residual"`
	// Loaded lists the project files first loaded during the example.
	Loaded []string `json:"loaded"`
}

// SegmentRun is one segment: its kind, identity, anchor, time, and the
// project lines it ran with their counts.
type SegmentRun struct {
	Kind       string                `json:"kind"`
	ID         string                `json:"id"`
	AnchorPath string                `json:"anchor_path"`
	AnchorLine int                   `json:"anchor_line"`
	ElapsedNS  int64                 `json:"elapsed_ns"`
	Lines      map[string][][2]int64 `json:"lines"`
}

// ParseSegmentReport reads a report and refuses another format or a body
// whose kinds, paths or lines cannot be trusted, so a damaged report
// decides nothing.
func ParseSegmentReport(data []byte) (SegmentReport, error) {
	var r SegmentReport
	if len(data) > maxSegmentReportBytes {
		return r, errors.New("rubytestprovider: segment report too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("rubytestprovider: segment report: %w", err)
	}
	if r.Format != SegmentsFormat {
		return r, fmt.Errorf("rubytestprovider: segment report format %q, want %q", r.Format, SegmentsFormat)
	}
	checkLines := func(lines map[string][][2]int64) error {
		for p, ls := range lines {
			if !relPath(p) {
				return fmt.Errorf("rubytestprovider: segment path %q is not project-relative", p)
			}
			for _, l := range ls {
				if l[0] < 1 || l[0] > math.MaxInt32 || l[1] < 1 {
					return fmt.Errorf("rubytestprovider: segment line %v of %s", l, p)
				}
			}
		}
		return nil
	}
	for p := range r.Files {
		if !relPath(p) {
			return r, fmt.Errorf("rubytestprovider: segment path %q is not project-relative", p)
		}
	}
	seen := map[string]bool{}
	for _, e := range r.Examples {
		if e.ID == "" || seen[e.ID] {
			return r, fmt.Errorf("rubytestprovider: example id %q empty or repeated", e.ID)
		}
		seen[e.ID] = true
		if _, _, ok := splitExampleID(e.ID); !ok {
			return r, fmt.Errorf("rubytestprovider: example id %q", e.ID)
		}
		for _, s := range e.Segments {
			switch linecov.SegmentKind(s.Kind) {
			case linecov.KindStep, linecov.KindMethod, linecov.KindSetup, linecov.KindTest:
			default:
				return r, fmt.Errorf("rubytestprovider: segment kind %q", s.Kind)
			}
			if s.ID == "" {
				return r, errors.New("rubytestprovider: segment without identity")
			}
			if err := checkLines(s.Lines); err != nil {
				return r, err
			}
		}
		if err := checkLines(e.Residual); err != nil {
			return r, err
		}
		for _, p := range e.Loaded {
			if !relPath(p) {
				return r, fmt.Errorf("rubytestprovider: loaded path %q is not project-relative", p)
			}
		}
	}
	return r, nil
}

func relPath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}

// splitExampleID splits "./spec/x_spec.rb[1:2]" into its file and scoped id.
func splitExampleID(id string) (file, scoped string, ok bool) {
	i := strings.LastIndexByte(id, '[')
	if i <= 0 || !strings.HasSuffix(id, "]") {
		return "", "", false
	}
	file = strings.TrimPrefix(id[:i], "./")
	return file, id[i:], relPath(file)
}

// SegmentConfig is one collector run.
type SegmentConfig struct {
	Dir      string   // absolute project directory
	RSpec    []string // default: bundle exec rspec
	Specs    []string // project-relative selectors; empty runs the suite
	StepDirs []string // project-relative step directories; default spec/support/steps
	Timeout  time.Duration
	// Env, when set, replaces the inherited environment.
	Env []string
}

// RunSegments runs RSpec once with the collector and returns its report.
// Failed examples stay in the report with their status; BuildSegmentIndex
// records passing ones only (LPCV-V0-962).
func RunSegments(ctx context.Context, c SegmentConfig) (SegmentReport, []byte, error) {
	if !filepath.IsAbs(c.Dir) {
		return SegmentReport{}, nil, errors.New("rubytestprovider: segment run needs an absolute project directory")
	}
	argv := c.RSpec
	if len(argv) == 0 {
		argv = []string{"bundle", "exec", "rspec"}
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	stepDirs := c.StepDirs
	if len(stepDirs) == 0 {
		stepDirs = []string{"spec/support/steps"}
	}
	dir := filepath.Join(c.Dir, reportDirectory)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return SegmentReport{}, nil, err
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return SegmentReport{}, nil, err
	}
	tag := hex.EncodeToString(nonce[:])
	collector := filepath.Join(dir, "corvint_segments-"+tag+".rb")
	out := filepath.Join(dir, "segments-"+tag+".json")
	defer os.Remove(collector)
	defer os.Remove(out)
	if err := os.WriteFile(collector, segmentCollector, 0o600); err != nil {
		return SegmentReport{}, nil, err
	}
	env := c.Env
	if env == nil {
		env = os.Environ()
	}
	env = SetEnv(env, "CORVINT_RUBY_SEGMENTS_OUT", out)
	env = SetEnv(env, "CORVINT_RUBY_SEGMENTS_ROOT", c.Dir)
	env = SetEnv(env, "CORVINT_RUBY_STEP_DIRS", strings.Join(stepDirs, ":"))
	full := append(append(append([]string(nil), argv...), "-r", collector), c.Specs...)
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(full), Dir: c.Dir, Env: env, Timeout: timeout,
		OutputLimit: maxRunnerOutputBytes, OverflowPolicy: procgroup.OverflowTruncate})
	data, err := os.ReadFile(out)
	if err != nil {
		return SegmentReport{}, nil, fmt.Errorf("rubytestprovider: segment run left no report (exit %d, %v): %s", obs.ExitStatus, obs.Err, tail(obs.Stderr, obs.Stdout))
	}
	r, err := ParseSegmentReport(data)
	return r, data, err
}

// SetEnv returns env with exactly one name=value entry for name; a process
// spec refuses an environment naming one variable twice.
func SetEnv(env []string, name, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != name {
			out = append(out, kv)
		}
	}
	return append(out, name+"="+value)
}

func tail(outs ...[]byte) string {
	s := strings.TrimSpace(string(bytes.Join(outs, []byte("\n"))))
	if len(s) > 2000 {
		s = s[len(s)-2000:]
	}
	return s
}

// SegmentIndexOptions bind a report to the commit it ran at.
type SegmentIndexOptions struct {
	Module string // index identity; the repository or project name
	Commit string
	Runner string // Ruby, RSpec and lockfile identity; a different runner decides nothing
	// Prefix is the project directory relative to the repository root ("" at
	// the root); report paths are joined to it.
	Prefix string
	// OIDs maps every repository path at Commit to its blob OID.
	OIDs map[string]string
	// Dirty lists repository paths whose worktree content differs from
	// Commit; executing one refuses the build, and no segment anchored in
	// one binds.
	Dirty map[string]bool
	// Rules are the repository's declared level path rules (MCJ-V0-007).
	Rules []linecov.LevelRule
}

// BuildSegmentIndex builds the linecov-index/3 index of a report
// (MCJ-V0-041, MCJ-V0-042). A test is "<spec file>" + "[scoped id]"; each
// block is one executable source line. A segment binds when its anchor file
// is committed, unchanged in the worktree, and holds the anchor line
// (MCJ-V0-042); lines an example ran outside every segment form one trailing
// unbound segment. A failed example has no record.
func BuildSegmentIndex(r SegmentReport, o SegmentIndexOptions) (*linecov.Index, error) {
	if o.Module == "" || o.Commit == "" || o.Runner == "" {
		return nil, errors.New("rubytestprovider: index identity incomplete")
	}
	repo := func(p string) string {
		if o.Prefix == "" || o.Prefix == "." {
			return p
		}
		return path.Join(o.Prefix, p)
	}
	type example struct {
		e     SegmentExample
		test  linecov.Test
		level string
	}
	var exs []example
	for _, e := range r.Examples {
		if e.Status != "passed" {
			continue
		}
		file, scoped, _ := splitExampleID(e.ID)
		level, _, err := linecov.ResolveLevel(linecov.LevelInput{Declared: e.DeclaredLevel, Rules: o.Rules, Family: "rspec",
			RunnerType: e.Type, Path: repo(file)})
		if err != nil {
			return nil, fmt.Errorf("rubytestprovider: %s: %w", e.ID, err)
		}
		incomplete := false
		for _, why := range e.Incomplete {
			if !incompleteReasons[why] {
				return nil, fmt.Errorf("rubytestprovider: %s: unnamed incomplete reason %q", e.ID, why)
			}
			incomplete = true
		}
		exs = append(exs, example{e: e, test: linecov.Test{Package: repo(file), Name: scoped, Elapsed: time.Duration(e.ElapsedNS),
			Level: level, Incomplete: incomplete}})
	}
	sort.Slice(exs, func(a, b int) bool {
		if exs[a].test.Package != exs[b].test.Package {
			return exs[a].test.Package < exs[b].test.Package
		}
		return exs[a].test.Name < exs[b].test.Name
	})
	ix := &linecov.Index{Format: linecov.Format, Module: o.Module, Commit: o.Commit, Runner: o.Runner}
	type lineKey struct {
		path string
		line int32
	}
	// A file first loaded inside an example ran its load-time lines there,
	// so which tests depend on them is unknown: it is left out of the index
	// and a change to it keeps the static plan (MCJ-V0-042).
	unmapped := map[string]bool{}
	for _, x := range exs {
		for _, p := range x.e.Loaded {
			unmapped[repo(p)] = true
		}
	}
	segsOf := map[lineKey][]int32{}
	testsOf := map[lineKey]map[int32]bool{}
	add := func(t int32, s int32, lines map[string][][2]int64) error {
		for p, ls := range lines {
			rp := repo(p)
			if o.OIDs[rp] == "" {
				return fmt.Errorf("rubytestprovider: executed file %s has no blob at %s", rp, o.Commit)
			}
			if o.Dirty[rp] {
				return fmt.Errorf("rubytestprovider: executed file %s differs from %s in the worktree", rp, o.Commit)
			}
			if unmapped[rp] {
				continue
			}
			for _, l := range ls {
				k := lineKey{rp, int32(l[0])}
				segsOf[k] = append(segsOf[k], s)
				if testsOf[k] == nil {
					testsOf[k] = map[int32]bool{}
				}
				testsOf[k][t] = true
			}
		}
		return nil
	}
	for i, x := range exs {
		t := int32(i)
		ix.Tests = append(ix.Tests, x.test)
		for _, s := range x.e.Segments {
			anchor := repo(s.AnchorPath)
			bound := s.AnchorPath != "" && relPath(s.AnchorPath) && o.OIDs[anchor] != "" && !o.Dirty[anchor] && s.AnchorLine > 0 &&
				executableOrAnchored(r.Files[s.AnchorPath], s.AnchorLine)
			seg := linecov.Segment{Test: t, Kind: linecov.SegmentKind(s.Kind), ID: s.ID, Bound: bound, Elapsed: time.Duration(s.ElapsedNS)}
			id := int32(len(ix.Segments))
			ix.Segments = append(ix.Segments, seg)
			if err := add(t, id, s.Lines); err != nil {
				return nil, err
			}
		}
		if len(x.e.Residual) > 0 || len(x.e.Segments) == 0 {
			id := int32(len(ix.Segments))
			kind := linecov.SegmentKind("")
			if len(x.e.Segments) == 0 {
				kind = linecov.KindTest // no boundary at all: the whole test
			}
			ix.Segments = append(ix.Segments, linecov.Segment{Test: t, Kind: kind})
			if err := add(t, id, x.e.Residual); err != nil {
				return nil, err
			}
		}
	}
	// Every executable line of every executed project file is a block, run
	// or not, so an unrun line reads as instrumented and uncovered.
	exec := map[string]map[int32]bool{}
	for p, ls := range r.Files {
		rp := repo(p)
		if o.OIDs[rp] == "" || unmapped[rp] {
			continue // not committed, or loaded inside an example: not indexed
		}
		exec[rp] = map[int32]bool{}
		for _, l := range ls {
			if l > 0 && l <= math.MaxInt32 {
				exec[rp][int32(l)] = true
			}
		}
	}
	for k := range testsOf {
		if exec[k.path] == nil {
			exec[k.path] = map[int32]bool{}
		}
		exec[k.path][k.line] = true
	}
	paths := make([]string, 0, len(exec))
	for p := range exec {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		f := linecov.File{Path: p, OID: o.OIDs[p]}
		lines := make([]int32, 0, len(exec[p]))
		for l := range exec[p] {
			lines = append(lines, l)
		}
		sort.Slice(lines, func(a, b int) bool { return lines[a] < lines[b] })
		for _, l := range lines {
			b := linecov.Block{Start: l, StartCol: 1, End: l, EndCol: math.MaxInt32}
			k := lineKey{p, l}
			for t := range testsOf[k] {
				b.Tests = append(b.Tests, t)
			}
			sort.Slice(b.Tests, func(a, c int) bool { return b.Tests[a] < b.Tests[c] })
			if len(b.Tests) > 0 {
				b.Segs = uniqueSegs(segsOf[k])
			}
			f.Blocks = append(f.Blocks, b)
		}
		ix.Files = append(ix.Files, f)
	}
	// Round-trip so the index is checked and indexed exactly as a reader
	// sees it.
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		return nil, err
	}
	return linecov.Decode(&buf)
}

// executableOrAnchored accepts an anchor line the report saw as part of its
// file: a def or hook line is executable in Ruby coverage.
func executableOrAnchored(lines []int, line int) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func uniqueSegs(ids []int32) []int32 {
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	out := ids[:0:0]
	for i, id := range ids {
		if i == 0 || id != ids[i-1] {
			out = append(out, id)
		}
	}
	return out
}
