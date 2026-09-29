// SPDX-License-Identifier: AGPL-3.0-or-later
// Experimental offline comparison; headings confer no authority.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

type span struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}
type trial struct {
	ID          string `json:"id"`
	Query       string `json:"query"`
	Gold        []span `json:"gold"`
	AnswerNotes string `json:"answer_notes"`
}
type fixture struct {
	Profile   string   `json:"profile"`
	Revision  string   `json:"revision"`
	Timestamp string   `json:"timestamp"`
	Paths     []string `json:"paths"`
	Cases     []trial  `json:"cases"`
}
type section struct {
	start, end, level, parent int
	title                     string
}
type source struct {
	input    doccorpus.Input
	lines    []string
	sections []section
}
type segment struct {
	span
	Blob string `json:"blob"`
	Text string `json:"text"`
}
type outline struct {
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	Line       int    `json:"line"`
	Level      int    `json:"level"`
	Title      string `json:"title"`
	ParentLine int    `json:"parent_line"`
}
type packet struct {
	Profile         string    `json:"profile"`
	Arm             string    `json:"arm"`
	Question        string    `json:"question"`
	Revision        string    `json:"revision"`
	Budget          int       `json:"budget_bytes"`
	Bytes           int       `json:"packet_bytes"`
	Ranked          []string  `json:"ranked_paths"`
	Outlines        []outline `json:"outlines"`
	Segments        []segment `json:"segments"`
	OmittedLines    int       `json:"omitted_lines"`
	OmittedHeadings int       `json:"omitted_headings"`
	Limitations     []string  `json:"limitations"`
}
type result struct {
	Case              string `json:"case"`
	Arm               string `json:"arm"`
	PacketBytes       int    `json:"packet_bytes"`
	RecallNumerator   int    `json:"recall_numerator"`
	RecallDenominator int    `json:"recall_denominator"`
	RecallDefined     bool   `json:"recall_defined"`
	Misses            []span `json:"critical_misses"`
	WallNS            int64  `json:"wall_ns"`
}

var atx = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*)|$)`)
var setext = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
var fence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
var closingHashes = regexp.MustCompile(`[ \t]+#+[ \t]*$`)

// Partition all lines, including preamble and each parent's introductory body.
// CommonMark container blocks are deliberately outside this flat parser's scope.
func parse(lines []string) []section {
	out := []section{{start: 1, end: len(lines), parent: -1}}
	var fenceChar byte
	fenceLen := 0
	for i, raw := range lines {
		line := strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r")
		if fenceLen > 0 {
			if m := fence.FindStringSubmatch(line); m != nil && m[1][0] == fenceChar && len(m[1]) >= fenceLen && strings.TrimSpace(m[2]) == "" {
				fenceLen = 0
			}
			continue
		}
		if m := fence.FindStringSubmatch(line); m != nil && (m[1][0] != '`' || !strings.Contains(m[2], "`")) {
			fenceChar = m[1][0]
			fenceLen = len(m[1])
			continue
		}
		level, title, start := 0, "", i+1
		if m := atx.FindStringSubmatch(line); m != nil {
			level = len(m[1])
			title = strings.TrimSpace(closingHashes.ReplaceAllString(m[2], ""))
		} else if m := setext.FindStringSubmatch(line); m != nil && i > 0 && out[len(out)-1].start < i+1 {
			prev := strings.TrimSuffix(strings.TrimSuffix(lines[i-1], "\n"), "\r")
			// Conservative one-line setext titles only, excluding container/list/code syntax.
			if strings.TrimSpace(prev) != "" && !strings.HasPrefix(prev, "    ") && !strings.ContainsAny(strings.TrimSpace(prev)[:1], ">#-*+`~") && fence.FindStringSubmatch(prev) == nil {
				level = 2
				if m[1][0] == '=' {
					level = 1
				}
				title = strings.TrimSpace(prev)
				start = i
			}
		}
		if level == 0 {
			continue
		}
		parent := 0
		for j := len(out) - 1; j > 0; j-- {
			if out[j].level < level {
				parent = j
				break
			}
		}
		out[len(out)-1].end = start - 1
		out = append(out, section{start: start, end: len(lines), level: level, parent: parent, title: title})
	}
	return out
}
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	return c.Output()
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func load(ctx context.Context, root string, in doccorpus.Input) (source, error) {
	oid, err := git(ctx, root, "rev-parse", in.Revision+":"+in.Path)
	if err != nil {
		return source{}, err
	}
	if strings.TrimSpace(string(oid)) != in.Blob {
		return source{}, errors.New("manifest blob drift: " + in.Path)
	}
	raw, err := git(ctx, root, "cat-file", "blob", in.Blob)
	if err != nil {
		return source{}, err
	}
	if digest(raw) != in.SHA256 {
		return source{}, errors.New("manifest digest drift: " + in.Path)
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return source{in, lines, parse(lines)}, nil
}
func ranked(a *doccorpus.Artifact, q string) ([]string, error) {
	r, err := doccorpus.Query(a, doccorpus.Request{Operation: "search", Query: q, Limit: doccorpus.MaxResults}, "pinned", nil)
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for _, v := range r.Results {
		s, ok := v.(doccorpus.Subject)
		if !ok {
			return nil, errors.New("unexpected corpus result")
		}
		for _, a := range s.Evidence.Anchors {
			if !seen[a.Path] {
				seen[a.Path] = true
				out = append(out, a.Path)
			}
		}
	}
	return out, nil
}
func encode(p *packet) ([]byte, error) {
	// packet_bytes includes itself and every serialized metadata byte.
	for i := 0; i < 12; i++ {
		b, e := json.Marshal(p)
		if e != nil {
			return nil, e
		}
		if p.Bytes == len(b)+1 {
			return append(b, '\n'), nil
		}
		p.Bytes = len(b) + 1
	}
	return nil, errors.New("packet size did not converge")
}
func fits(p *packet) bool { b, e := encode(p); return e == nil && len(b) <= p.Budget }
func ordered(s source, q string) []int {
	terms := contextindex.EvidenceTerms(q)
	ids := make([]int, len(s.sections))
	scores := make([]int, len(ids))
	for i, x := range s.sections {
		ids[i] = i
		text := x.title + " " + strings.Join(s.lines[x.start-1:x.end], "")
		for j := x.parent; j > 0; j = s.sections[j].parent {
			text += " " + s.sections[j].title
		}
		for w := range contextindex.EvidenceTerms(text) {
			if _, ok := terms[w]; ok {
				scores[i]++
			}
		}
	}
	sort.SliceStable(ids, func(i, j int) bool { return scores[ids[i]] > scores[ids[j]] })
	out := []int{}
	seen := map[int]bool{}
	var add func(int)
	add = func(i int) {
		if i < 0 || seen[i] {
			return
		}
		add(s.sections[i].parent)
		seen[i] = true
		out = append(out, i)
	}
	for _, i := range ids {
		add(i)
	}
	return out
}
func makePacket(rev, q, arm string, budget int, paths []string, all map[string]source) (packet, []byte, error) {
	p := packet{Profile: "heading-navigation-packet/0", Arm: arm, Question: q, Revision: rev, Budget: budget, Ranked: paths, Outlines: []outline{}, Segments: []segment{}, Limitations: []string{"experimental deterministic lexical navigation; headings grant no authority", "flat ATX and conservative single-line setext only; container headings unsupported", "omissions include unranked documents; search bounded to 256 results; only body spans count as read"}}
	for _, s := range all {
		p.OmittedLines += len(s.lines)
		p.OmittedHeadings += len(s.sections) - 1
	}
	if !fits(&p) {
		return p, nil, errors.New("minimal packet envelope exceeds budget")
	}
	for _, path := range paths {
		s, ok := all[path]
		if !ok {
			return p, nil, fmt.Errorf("search returned undeclared path %s", path)
		}
		ids := []int{-1}
		if arm == "heading" {
			ids = ordered(s, q)
		}
		for _, id := range ids {
			start, end := 1, len(s.lines)
			if id >= 0 {
				start, end = s.sections[id].start, s.sections[id].end
			}
			// Add selected ancestry labels, capped to one quarter of the packet budget.
			if id > 0 {
				x := s.sections[id]
				parentLine := 0
				if x.parent > 0 {
					parentLine = s.sections[x.parent].start
				}
				old := p.Bytes
				p.Outlines = append(p.Outlines, outline{path, s.input.Blob, x.start, x.level, x.title, parentLine})
				p.OmittedHeadings--
				ob, _ := json.Marshal(p.Outlines)
				if !fits(&p) || len(ob) > budget/4 {
					p.Outlines = p.Outlines[:len(p.Outlines)-1]
					p.OmittedHeadings++
					p.Bytes = old
				}
			}
			for line := start; line <= end; line++ {
				old := append([]segment(nil), p.Segments...)
				n := len(p.Segments)
				if n > 0 && p.Segments[n-1].Path == path && p.Segments[n-1].End == line-1 {
					p.Segments[n-1].End = line
					p.Segments[n-1].Text += s.lines[line-1]
				} else {
					p.Segments = append(p.Segments, segment{span{path, line, line}, s.input.Blob, s.lines[line-1]})
				}
				p.OmittedLines--
				if !fits(&p) {
					p.Segments = old
					p.OmittedLines++
					break
				}
			}
		}
	}
	b, e := encode(&p)
	return p, b, e
}
func recall(gold []span, p packet) (int, []span) {
	n := 0
	misses := []span{}
	for _, g := range gold {
		covered := map[int]bool{}
		for _, s := range p.Segments {
			if s.Path == g.Path {
				for i := s.Start; i <= s.End; i++ {
					covered[i] = true
				}
			}
		}
		ok := true
		for i := g.Start; i <= g.End; i++ {
			if !covered[i] {
				ok = false
				break
			}
		}
		if ok {
			n++
		} else {
			misses = append(misses, g)
		}
	}
	return n, misses
}
func validate(f fixture) error {
	if f.Profile != "heading-navigation-fixture/0" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(f.Revision) {
		return errors.New("invalid fixture profile/revision")
	}
	if _, e := time.Parse(time.RFC3339, f.Timestamp); e != nil {
		return e
	}
	if len(f.Paths) == 0 || len(f.Paths) > 32 || len(f.Cases) == 0 || len(f.Cases) > 100 {
		return errors.New("fixture bounds exceeded")
	}
	for i, p := range f.Paths {
		if !strings.HasSuffix(p, ".md") || filepath.IsAbs(p) || filepath.Clean(p) != p || strings.HasPrefix(p, "../") || (i > 0 && p <= f.Paths[i-1]) {
			return errors.New("paths must be sorted unique relative Markdown files")
		}
	}
	seen := map[string]bool{}
	for _, c := range f.Cases {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(c.ID) || seen[c.ID] || c.Query == "" || len(c.Query) > 1024 {
			return errors.New("invalid case")
		}
		seen[c.ID] = true
		for _, g := range c.Gold {
			if !sort.StringsAreSorted(f.Paths) || !contains(f.Paths, g.Path) || g.Start < 1 || g.End < g.Start {
				return errors.New("invalid gold")
			}
		}
	}
	return nil
}
func contains(xs []string, s string) bool {
	i := sort.SearchStrings(xs, s)
	return i < len(xs) && xs[i] == s
}
func run(ctx context.Context, fpath, out string, budget int) error {
	start := time.Now()
	raw, e := os.ReadFile(fpath)
	if e != nil {
		return e
	}
	var f fixture
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&f); e != nil {
		return e
	}
	if e = dec.Decode(new(any)); e != io.EOF {
		return errors.New("fixture has trailing data")
	}
	if e = validate(f); e != nil {
		return e
	}
	if budget < 1 || budget > 1<<20 {
		return errors.New("budget must be 1..1048576")
	}
	rootRaw, e := git(ctx, ".", "rev-parse", "--show-toplevel")
	if e != nil {
		return e
	}
	root := strings.TrimSpace(string(rootRaw))
	var m doccorpus.Manifest
	for i, p := range f.Paths {
		part, err := doccorpus.Inventory(ctx, root, f.Revision, p, f.Timestamp)
		if err != nil {
			return err
		}
		if len(part.Inputs) != 1 || part.Inputs[0].Path != p {
			return errors.New("fixture scope is not a file")
		}
		if i == 0 {
			m = part
		} else {
			m.Inputs = append(m.Inputs, part.Inputs...)
			m.Scopes = append(m.Scopes, part.Scopes...)
		}
	}
	a, e := doccorpus.Build(ctx, root, m)
	if e != nil {
		return e
	}
	all := map[string]source{}
	for _, in := range m.Inputs {
		s, err := load(ctx, root, in)
		if err != nil {
			return err
		}
		all[in.Path] = s
	}
	for _, c := range f.Cases {
		for _, g := range c.Gold {
			if g.End > len(all[g.Path].lines) {
				return errors.New("gold outside immutable source")
			}
		}
	}
	buildNS := time.Since(start).Nanoseconds()
	rows := []result{}
	// Refuse overwrite: artifact paths form the frozen trial record.
	if e = os.Mkdir(out, 0700); e != nil {
		return e
	}
	for _, c := range f.Cases {
		for _, arm := range []string{"baseline", "heading"} {
			began := time.Now()
			paths, err := ranked(a, c.Query)
			if err != nil {
				return err
			}
			p, b, err := makePacket(f.Revision, c.Query, arm, budget, paths, all)
			if err != nil {
				return err
			}
			elapsed := time.Since(began).Nanoseconds()
			if e = os.WriteFile(filepath.Join(out, c.ID+"-"+arm+".json"), b, 0600); e != nil {
				return e
			}
			n, misses := recall(c.Gold, p)
			rows = append(rows, result{c.ID, arm, len(b), n, len(c.Gold), len(c.Gold) > 0, misses, elapsed})
		}
	}
	report := map[string]any{"profile": "heading-navigation-report/0", "fixture_sha256": digest(raw), "revision": f.Revision, "corpus_sha256": a.SHA256, "builder": a.Builder, "builder_limitation": "experimental harness source may be uncommitted; no immutable harness attestation; corpus builder identity does not attest harness", "build_wall_ns": buildNS, "wall_ns": time.Since(start).Nanoseconds(), "budget_bytes": budget, "results": rows, "authority_errors": "NOT_OBSERVED", "unsupported_answers": "NOT_OBSERVED", "answer_correctness": "NOT_OBSERVED", "limitations": []string{"gold is used only for validation and post-retrieval scoring", "all gold spans are treated as critical; recall requires full-span union coverage; zero denominator is undefined", "flat Markdown parser is deliberately incomplete; heading labels alone are not read evidence", "lexical policy is deterministic and is not an LLM navigation trial"}}
	b, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(out, "report.json"), append(b, '\n'), 0600)
}
func main() {
	f := flag.String("fixture", "", "frozen fixture JSON")
	b := flag.Int("budget", 6000, "serialized bytes per packet")
	o := flag.String("output", "", "new output directory")
	flag.Parse()
	if *f == "" || *o == "" {
		fmt.Fprintln(os.Stderr, "--fixture and --output required")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, *f, *o, *b); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
