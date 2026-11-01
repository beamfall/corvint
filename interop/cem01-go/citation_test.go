package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Citation fixtures (decision 0523 P0, CEM-V1-080..082). Each case is a
// deterministic two-commit repository; its inputs and the resulting citation
// block are retained under testdata/citation/<case>/ so the native verifier can
// be run over the same bytes. Set CEM01_CITATION_UPDATE=1 to rewrite them.

type citationTree struct {
	Base   map[string]string `json:"base"`
	Target map[string]string `json:"target"`
}

// citationCite names one evidence span: a base path and a byte range given by
// the first occurrence of a substring (empty means the whole file).
type citationCite struct{ path, text string }

// citationPlan maps a hunk's post-image path to its basis (nil = no basis) or,
// for mechanical hunks, to the reason.
type citationPlan struct {
	cites      map[string][]citationCite
	mechanical map[string]string
}

type citationFixture struct {
	repo, base, target string
	tree               citationTree
	patch              []byte
	hunks              []*patchHunk
}

func citationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-10-11T00:00:00Z",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-10-11T00:00:00Z")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return string(b)
}

func newCitationFixture(t *testing.T, tree citationTree) *citationFixture {
	t.Helper()
	repo := t.TempDir()
	citationGit(t, repo, "init", "-q")
	commit := func(files map[string]string, msg string) string {
		paths, _ := filepath.Glob(filepath.Join(repo, "*"))
		for _, p := range paths {
			if filepath.Base(p) != ".git" {
				if err := os.RemoveAll(p); err != nil {
					t.Fatal(err)
				}
			}
		}
		for p, body := range files {
			full := filepath.Join(repo, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		citationGit(t, repo, "add", "-A", ".")
		citationGit(t, repo, "commit", "-q", "--allow-empty", "-m", msg)
		return strings.TrimSpace(citationGit(t, repo, "rev-parse", "HEAD"))
	}
	f := &citationFixture{repo: repo, tree: tree}
	f.base = commit(tree.Base, "base")
	f.target = commit(tree.Target, "target")
	v := &verifier{ctx: context.Background(), repo: repo, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	patch, e := v.patch03(f.base, f.target)
	if e != nil {
		t.Fatal(e)
	}
	parsed, e := parsePatch(patch)
	if e != nil {
		t.Fatal(e)
	}
	f.patch, f.hunks = patch, parsed.hunks
	return f
}

func (f *citationFixture) evidence(t *testing.T, c citationCite) evidence {
	t.Helper()
	data := []byte(f.tree.Base[c.path])
	start, end := 0, len(data)
	if c.text != "" {
		start = bytes.Index(data, []byte(c.text))
		if start < 0 {
			t.Fatalf("span text %q not in %s", c.text, c.path)
		}
		end = start + len(c.text)
	}
	e := evidence{Path: c.path, BlobOID: strings.TrimSpace(citationGit(t, f.repo, "rev-parse", f.base+":"+c.path)), Span: span{uint64(start), uint64(end)}, SpanSHA256: shaHex(data[start:end])}
	e.ID = "evidence:sha256:" + shaHex(canonicalEvidence(e))
	return e
}

func (f *citationFixture) stableMap(t *testing.T, plan citationPlan) stableMap {
	t.Helper()
	m := stableMap{Spec: stableSpec, BaseRevision: f.base, PatchSHA256: shaHex(f.patch), ExcludedPath: sidecar03, Evidence: []evidence{}, Hunks: []hunk03{}, stableReferences: emptyStableReferences()}
	seen := map[string]bool{}
	for _, p := range f.hunks {
		path := *p.OldPath
		if p.NewPath != nil {
			path = *p.NewPath
		}
		h := mappedHunk{ID: p.ID, Path: path, OldRange: p.OldRange, NewRange: p.NewRange, Basis: []basis{}}
		if reason, ok := plan.mechanical[path]; ok {
			h.Disposition, h.Reason = "mechanical", reason
		} else if cs := plan.cites[path]; len(cs) > 0 {
			h.Disposition, h.Reason = "supported", "evidence-backed"
			for _, c := range cs {
				e := f.evidence(t, c)
				h.Basis = append(h.Basis, basis{EvidenceID: e.ID, Relation: "specification"})
				if !seen[e.ID] {
					seen[e.ID] = true
					m.Evidence = append(m.Evidence, e)
				}
			}
		} else {
			h.Disposition, h.Reason = "unknown", "no-evidence"
		}
		m.Hunks = append(m.Hunks, hunk03{mappedHunk: h})
	}
	return m
}

// run proves the map ACCEPTs on the sealed stable route (mechanical re-proof
// and span binding included), then computes the citation block directly from
// the retained map bytes, the canonical patch and the base span bytes. The
// stable result carries the same block as its citation member (CEM-V1-092).
func (f *citationFixture) run(t *testing.T, m stableMap) (stableResult, *citationBlock, []byte, []byte) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	r, n := runStable(context.Background(), f.repo, raw, f.base, f.target, t.TempDir())
	if n != 0 || r.Outcome != "ACCEPT" {
		b, _ := json.Marshal(r)
		t.Fatalf("exit=%d result=%s", n, b)
	}
	var wire stableMap
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for i := range wire.Evidence {
		e := &wire.Evidence[i]
		blob := []byte(citationGit(t, f.repo, "cat-file", "blob", e.BlobOID))
		e.data = blob[e.Span.Start:e.Span.End]
		if shaHex(e.data) != e.SpanSHA256 {
			t.Fatalf("span %s does not bind", e.ID)
		}
	}
	ph := map[string]*patchHunk{}
	for _, h := range f.hunks {
		ph[h.ID] = h
	}
	c := computeCitation(citationHunks(wire.Hunks), ph, wire.Evidence)
	block, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return r, c, raw, block
}

// citationGolden compares (or, with CEM01_CITATION_UPDATE=1, writes) the retained inputs and block.
func citationGolden(t *testing.T, name string, f *citationFixture, raw, block []byte) {
	t.Helper()
	tree, err := json.MarshalIndent(f.tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"tree.json": append(tree, '\n'), "map.json": append(append([]byte(nil), raw...), '\n'), "patch.diff": f.patch, "citation.json": append(append([]byte(nil), block...), '\n'),
		"revisions.txt": []byte("base " + f.base + "\ntarget " + f.target + "\n")}
	dir := filepath.Join("testdata", "citation", name)
	update := os.Getenv("CEM01_CITATION_UPDATE") == "1"
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"tree.json", "map.json", "patch.diff", "citation.json", "revisions.txt"} {
		p := filepath.Join(dir, n)
		if update {
			if err := os.WriteFile(p, files[n], 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(want, files[n]) {
			t.Errorf("%s differs from retained fixture (err=%v)\ngot=%s", p, err, files[n])
		}
	}
}

func unitFiles(n int, change func(i int) string) citationTree {
	tr := citationTree{Base: map[string]string{}, Target: map[string]string{}}
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("src/unit%02d.go", i)
		tr.Base[p] = fmt.Sprintf("package units\n\nfunc unit%02d(value int) int {\n\treturn value\n}\n", i)
		tr.Target[p] = fmt.Sprintf("package units\n\nfunc unit%02d(value int) int {\n%s\n}\n", i, change(i))
	}
	return tr
}

func patch24Tree() citationTree {
	tr := unitFiles(24, func(i int) string {
		if i < 8 {
			return fmt.Sprintf("\treturn compute(value) // CEM-SM-1%02d", i)
		}
		return fmt.Sprintf("\treturn compute(value) + %d", i)
	})
	agents := "# Agent guide\n\nKeep changes small and reviewed.\nPrefer clear names over clever code.\n"
	var spec, paths, notes strings.Builder
	spec.WriteString("# Requirements\n\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&spec, "- `CEM-SM-1%02d`: requirement %d computes its result.\n", i, i)
	}
	for i := 8; i < 16; i++ {
		fmt.Fprintf(&paths, "The handler in src/unit%02d.go owns step %d.\n", i, i)
	}
	for i := 16; i < 23; i++ {
		fmt.Fprintf(&notes, "Note %d: keep the retry budget bounded.\n", i)
	}
	for p, b := range map[string]string{"AGENTS.md": agents, "docs/spec.md": spec.String(), "docs/paths.md": paths.String(), "docs/notes.md": notes.String()} {
		tr.Base[p], tr.Target[p] = b, b
	}
	return tr
}

func patch24Careful() citationPlan {
	plan := citationPlan{cites: map[string][]citationCite{}}
	for i := 0; i < 24; i++ {
		p := fmt.Sprintf("src/unit%02d.go", i)
		switch {
		case i < 8:
			plan.cites[p] = []citationCite{{"docs/spec.md", fmt.Sprintf("- `CEM-SM-1%02d`: requirement %d computes its result.\n", i, i)}}
		case i < 16:
			plan.cites[p] = []citationCite{{"docs/paths.md", fmt.Sprintf("The handler in src/unit%02d.go owns step %d.\n", i, i)}}
		case i < 23:
			plan.cites[p] = []citationCite{{"docs/notes.md", fmt.Sprintf("Note %d: keep the retry budget bounded.\n", i)}}
		default:
			plan.cites[p] = []citationCite{{p, "package units\n"}}
		}
	}
	return plan
}

func patch24Rubber() citationPlan {
	plan := citationPlan{cites: map[string][]citationCite{}}
	for i := 0; i < 24; i++ {
		plan.cites[fmt.Sprintf("src/unit%02d.go", i)] = []citationCite{{"AGENTS.md", ""}}
	}
	return plan
}

func citationTiers(c *citationBlock) map[string]int {
	out := map[string]int{}
	for _, h := range c.Hunks {
		out[h.Tier]++
	}
	return out
}

// (a) rubber-stamp and careful maps of one 24-hunk patch both ACCEPT with different integers.
func TestCEMV1080RubberStampAndCarefulBothAccept(t *testing.T) {
	f := newCitationFixture(t, patch24Tree())
	if len(f.hunks) != 24 {
		t.Fatalf("hunks=%d", len(f.hunks))
	}
	rr, r, rraw, rubber := f.run(t, f.stableMap(t, patch24Rubber()))
	cr, c, craw, careful := f.run(t, f.stableMap(t, patch24Careful()))
	if r.HunksByClass != (citationClasses{Cited: 24}) || r.DistinctEvidence != 1 || r.MaxFanOut != 24 || len(r.EvidenceFanOut) != 1 || r.EvidenceFanOut[0].Hunks != 24 ||
		r.SamePathCitations != 0 || r.ConcentratedHunks != 24 || !r.Invariant || r.Specificity != 0 || r.AnnotationHunks != 0 {
		t.Fatalf("rubber-stamp block %s", rubber)
	}
	if c.HunksByClass != (citationClasses{Anchored: 16, Cited: 8}) || c.DistinctEvidence != 24 || c.MaxFanOut != 1 || len(c.EvidenceFanOut) != 24 ||
		c.SamePathCitations != 1 || c.ConcentratedHunks != 0 || c.Invariant || c.Specificity != 16 || c.AnnotationHunks != 0 {
		t.Fatalf("careful block %s", careful)
	}
	anchors := map[string]int{}
	for _, h := range c.Hunks {
		anchors[strings.Join(h.Anchors, ",")]++
	}
	if anchors["id"] != 8 || anchors["path"] != 8 || anchors[""] != 8 {
		t.Fatalf("careful anchors %v", anchors)
	}
	// The block never changes the verdict surface.
	for _, x := range []stableResult{rr, cr} {
		if x.Stage != "complete" || len(x.IssueCodes) != 0 || x.Code != nil {
			t.Fatalf("verdict surface changed %+v", x)
		}
	}
	citationGolden(t, "rubber-stamp", f, rraw, rubber)
	citationGolden(t, "careful", f, craw, careful)
}

// (b) AGENTS.md split into 20 disjoint spans, one per hunk: specificity 0, invariant false.
func TestCEMV1082DisjointUmbrellaSpansHaveZeroSpecificity(t *testing.T) {
	tr := unitFiles(20, func(i int) string { return fmt.Sprintf("\treturn compute(value) * %d", i+2) })
	var agents strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&agents, "Rule %d: keep each change reviewable.\n", i)
	}
	tr.Base["AGENTS.md"], tr.Target["AGENTS.md"] = agents.String(), agents.String()
	f := newCitationFixture(t, tr)
	plan := citationPlan{cites: map[string][]citationCite{}}
	for i := 0; i < 20; i++ {
		plan.cites[fmt.Sprintf("src/unit%02d.go", i)] = []citationCite{{"AGENTS.md", fmt.Sprintf("Rule %d: keep each change reviewable.\n", i)}}
	}
	_, c, raw, block := f.run(t, f.stableMap(t, plan))
	if c.Specificity != 0 || c.Invariant || !bytes.Contains(block, []byte(`"hunksByClass":{"held":0,"mechanical-proved":0,"anchored":0,"cited":20,"unknown":0}`)) ||
		c.DistinctEvidence != 20 || c.MaxFanOut != 1 || c.ConcentratedHunks != 0 {
		t.Fatalf("split block %s", block)
	}
	citationGolden(t, "split-20", f, raw, block)
}

func lexicalTree() citationTree {
	tr := citationTree{Base: map[string]string{
		"docs/ids.md": "The CEM-SM-0070 migration is tracked separately.\n" +
			"- `CEM-SM-007`: the check runs first.\n" +
			"Parsing lives in src/gamma.go today.\n",
		"src/alpha.go":  "package p\n\nvar a = 1\n",
		"src/beta.go":   "package p\n\nvar b = 1\n",
		"src/gamma.go":  "package p\n\nfunc g(value int) int {\n\treturn value\n}\n",
		"src/delta.txt": "alpha beta\n",
		"src/eps.go":    "package p\n\nfunc e() {\n}\n",
	}, Target: map[string]string{}}
	for p, b := range tr.Base {
		tr.Target[p] = b
	}
	tr.Target["src/alpha.go"] = "package p\n\nvar a = 1\nx // `CEM-SM-007`\n"
	tr.Target["src/beta.go"] = "package p\n\nvar b = 1\n// `CEM-SM-007`\n"
	tr.Target["src/gamma.go"] = "package p\n\nfunc g(value int) int {\n\treturn compute(value)\n}\n"
	tr.Target["src/delta.txt"] = "alpha  beta\n"
	tr.Target["src/eps.go"] = "package p\n\nfunc e() {\n\tcheck() // CEM-SM-007\n}\n"
	return tr
}

// (c)/(d) a boundary-failing ID mention is not a definition; annotation flags.
func TestCEMV1081IDBoundaryAndAnnotation(t *testing.T) {
	f := newCitationFixture(t, lexicalTree())
	plan := citationPlan{cites: map[string][]citationCite{
		"src/alpha.go": {{"docs/ids.md", "The CEM-SM-0070 migration is tracked separately.\n"}},
		"src/gamma.go": {{"docs/ids.md", "Parsing lives in src/gamma.go today.\n"}},
		"src/eps.go":   {{"docs/ids.md", "- `CEM-SM-007`: the check runs first.\n"}},
	}, mechanical: map[string]string{"src/delta.txt": "whitespace-only"}}
	r, c, raw, block := f.run(t, f.stableMap(t, plan))
	byPath := map[string]citationHunk{}
	for _, h := range c.Hunks {
		for _, m := range r.Hunks {
			if m.ID == h.HunkID {
				byPath[m.Path] = h
			}
		}
	}
	want := map[string]string{
		"src/alpha.go":  `{"hunkId":"%s","tier":"cited","anchors":[],"annotation":true}`,
		"src/beta.go":   `{"hunkId":"%s","tier":"unknown","anchors":[],"annotation":true}`,
		"src/gamma.go":  `{"hunkId":"%s","tier":"anchored","anchors":["path"],"annotation":false}`,
		"src/delta.txt": `{"hunkId":"%s","tier":"mechanical-proved","anchors":[],"annotation":false}`,
		"src/eps.go":    `{"hunkId":"%s","tier":"anchored","anchors":["id"],"annotation":false}`,
	}
	for p, w := range want {
		got, _ := json.Marshal(byPath[p])
		if string(got) != fmt.Sprintf(w, byPath[p].HunkID) {
			t.Errorf("%s: got %s", p, got)
		}
	}
	if c.AnnotationHunks != 2 || c.HunksByClass != (citationClasses{MechanicalProved: 1, Anchored: 2, Cited: 1, Unknown: 1}) {
		t.Fatalf("lexical block %s", block)
	}
	citationGolden(t, "lexical", f, raw, block)
}

// (e) byte-identical output across runs and across map hunk and evidence order.
func TestCEMV1082CitationIsOrderIndependent(t *testing.T) {
	f := newCitationFixture(t, patch24Tree())
	m := f.stableMap(t, patch24Careful())
	_, _, _, first := f.run(t, m)
	_, _, _, again := f.run(t, m)
	// Evidence order is free. Hunk order is fixed by CEM-V1-750, so a map whose
	// hunks are reordered is refused before any citation block is computed.
	rev := m
	rev.Evidence = append([]evidence(nil), m.Evidence...)
	sort.Slice(rev.Evidence, func(i, j int) bool { return rev.Evidence[i].ID > rev.Evidence[j].ID })
	_, _, _, reordered := f.run(t, rev)
	if !bytes.Equal(first, again) || !bytes.Equal(first, reordered) {
		t.Fatalf("citation bytes differ\n%s\n%s\n%s", first, again, reordered)
	}
	rev.Hunks = append([]hunk03(nil), m.Hunks...)
	sort.Slice(rev.Hunks, func(i, j int) bool { return rev.Hunks[i].ID > rev.Hunks[j].ID })
	raw, err := json.Marshal(rev)
	if err != nil {
		t.Fatal(err)
	}
	r, n := runStable(context.Background(), f.repo, raw, f.base, f.target, t.TempDir())
	if n != 1 || r.Outcome != "REJECT" || r.Stage != "verification" || len(r.IssueCodes) != 1 || r.IssueCodes[0] != "invalid-field" {
		b, _ := json.Marshal(r)
		t.Fatalf("hunk-reordered map: exit=%d result=%s, want a verification REJECT with invalid-field", n, b)
	}
}

func TestCEMV1081RequirementIDGrammar(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"x // `CEM-SM-007`", []string{"CEM-SM-007"}},
		{"CEM-SM-0070", nil},
		{"xCEM-SM-007", nil},
		{"-CEM-SM-007", nil},
		{"_CEM-SM-007", nil},
		{"CEM-SM-007-", nil},
		{"CEM-SM-007a", nil},
		{"(CEM-SM-007)", []string{"CEM-SM-007"}},
		{"AB-123", nil},
		{"ABC-123", []string{"ABC-123"}},
		{"ABC-123-456", []string{"ABC-123-456"}},
		{"A" + strings.Repeat("B", 31) + "-123", []string{"A" + strings.Repeat("B", 31) + "-123"}},
		{"A" + strings.Repeat("B", 32) + "-123", nil},
		{"V1-1262 and CEM-V1-080, LRF-001.", []string{"CEM-V1-080", "LRF-001"}},
	} {
		ids, _ := citationIDs([]byte(tc.in))
		var got []string
		for id := range ids {
			got = append(got, id)
		}
		sort.Strings(got)
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
	defs := citationDefined([]byte("  - `CEM-SM-007`: yes\n- `CEM-SM-008` : no\n-  `CEM-SM-009`: no\n\t- `CEM-SM-010`:\nx - `CEM-SM-011`: no\n"))
	if len(defs) != 2 || !defs["CEM-SM-007"] || !defs["CEM-SM-010"] {
		t.Fatalf("definitions %v", defs)
	}
	for in, want := range map[string]bool{"// `CEM-SM-007`\n": true, "x // `CEM-SM-007`\n": true, "return compute(value)\n": false, "return value\n": false, "007\n": false, "if ok {}\n": true} {
		_, rest := citationIDs([]byte(in))
		if got := citationNeutral(rest); got != want {
			t.Errorf("neutral(%q)=%v want %v", in, got, want)
		}
	}
}

// The accepted cem/1.0 stable result carries the computed block as its
// citation member, byte for byte (CEM-V1-082 placement, CEM-V1-092).
func TestCEMV1082StableResultCarriesTheBlock(t *testing.T) {
	f := newCitationFixture(t, patch24Tree())
	accepted, _, _, block := f.run(t, f.stableMap(t, patch24Careful()))
	b, _ := json.Marshal(accepted)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil || len(obj) != 23 || !bytes.Equal(obj["citation"], block) {
		t.Fatalf("accepted citation = %s, want %s", obj["citation"], block)
	}
}

// TestCEMV1092PortableCitationOnlyOnAccept proves the portable consumer emits
// citation only on ACCEPT: a REJECT (patch digest), an evidence-drift REJECT
// and an argument refusal all carry null in the full closed result.
func TestCEMV1092PortableCitationOnlyOnAccept(t *testing.T) {
	f := newCitationFixture(t, patch24Tree())
	m := f.stableMap(t, patch24Careful())
	accepted, _, _, _ := f.run(t, m)
	bad := m
	bad.PatchSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(bad)
	rejected, rejectExit := runStable(context.Background(), f.repo, raw, f.base, f.target, t.TempDir())
	raw, _ = json.Marshal(m)
	refused, refuseExit := runStable(context.Background(), "relative/repository", raw, f.base, f.target, t.TempDir())

	// Drift: the target drops the cited note, so its evidence span goes stale.
	tree := patch24Tree()
	tree.Target["docs/notes.md"] = strings.Replace(tree.Target["docs/notes.md"], "Note 16: keep the retry budget bounded.\n", "", 1)
	g := newCitationFixture(t, tree)
	raw, _ = json.Marshal(g.stableMap(t, patch24Careful()))
	drifted, driftExit := runStable(context.Background(), g.repo, raw, g.base, g.target, t.TempDir())

	for _, c := range []struct {
		name           string
		r              stableResult
		exit           int
		outcome, stage string
	}{
		{"accept", accepted, 0, "ACCEPT", "complete"},
		{"reject", rejected, rejectExit, "REJECT", rejected.Stage},
		{"drift", drifted, driftExit, "REJECT", "drift"},
		{"refusal", refused, refuseExit, "UNSUPPORTED", refused.Stage},
	} {
		b, _ := json.Marshal(c.r)
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil || len(obj) != 23 {
			t.Fatalf("%s: result shape %s", c.name, b)
		}
		if c.r.Outcome != c.outcome || c.r.Stage != c.stage {
			t.Fatalf("%s: outcome %s stage %s, want %s %s", c.name, c.r.Outcome, c.r.Stage, c.outcome, c.stage)
		}
		if got := string(obj["citation"]); (c.outcome == "ACCEPT") == (got == "null") {
			t.Errorf("%s: citation = %s", c.name, got)
		}
	}
	if rejectExit != 1 || driftExit != 1 || refuseExit != 2 {
		t.Fatalf("exits reject=%d drift=%d refusal=%d", rejectExit, driftExit, refuseExit)
	}
}

// Pinned counting rules: samePathCitations counts every basis entry (one
// evidence under two relations counts twice) while fan-out, distinctEvidence
// and the shift sets deduplicate; unknown is a non-mechanical hunk with no
// bases and stays outside the shift null.
func TestCEMV1082CountingRules(t *testing.T) {
	ev := []evidence{{ID: "evidence:e1", Path: "a.go", data: []byte("x")}, {ID: "evidence:e2", Path: "b.go", data: []byte("y")}}
	hunks := []mappedHunk{
		{ID: "hunk:1", Path: "a.go", Disposition: "supported", Basis: []basis{{"evidence:e1", "specification"}, {"evidence:e1", "constraint"}}},
		{ID: "hunk:2", Path: "b.go", Disposition: "supported", Basis: []basis{{"evidence:e1", "specification"}, {"evidence:e2", "specification"}}},
		{ID: "hunk:3", Path: "c.go", Disposition: "unknown", Basis: []basis{}},
	}
	got, _ := json.Marshal(computeCitation(hunks, map[string]*patchHunk{}, ev))
	want := `{"profile":"cem-citation-support/0","hunks":[` +
		`{"hunkId":"hunk:1","tier":"cited","anchors":[],"annotation":false},` +
		`{"hunkId":"hunk:2","tier":"cited","anchors":[],"annotation":false},` +
		`{"hunkId":"hunk:3","tier":"unknown","anchors":[],"annotation":false}],` +
		`"hunksByClass":{"held":0,"mechanical-proved":0,"anchored":0,"cited":2,"unknown":1},"annotationHunks":0,` +
		`"distinctEvidence":2,"evidenceFanOut":[["evidence:e1",2],["evidence:e2",1]],"maxFanOut":2,` +
		`"samePathCitations":3,"concentratedHunks":1,"invariant":false,"specificity":0}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
