// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

type fixture struct {
	t    *testing.T
	repo string
}

func (f fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid", "GIT_CONFIG_NOSYSTEM=1", "HOME="+f.repo)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f fixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.repo, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f fixture) commit(message string) string {
	f.git("add", "-A")
	f.git("commit", "-qm", message)
	return f.git("rev-parse", "HEAD")
}

func numbered(prefix string, n int, edits map[int]string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		line, ok := edits[i]
		if !ok {
			line = fmt.Sprintf("%s line %d", prefix, i)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// cemFor writes the lane CEM of BASE..TARGET in which each hunk on a path
// named in cites cites that text of doc.md at BASE; other hunks stay unknown.
func (f fixture) cemFor(base, target string, cites map[string]string) string {
	f.t.Helper()
	in := &inheritor{repo: f.repo, blobs: map[string][]byte{}}
	in.emptyTree, _ = in.git(nil, "hash-object", "-t", "tree", "--stdin")
	diff := in.canonicalDiff(base, target)
	docOid := f.git("rev-parse", base+":doc.md")
	doc := f.git("show", base+":doc.md") + "\n"
	out := document{BaseRevision: base}
	var ordered []string
	for id := range diff {
		ordered = append(ordered, id)
	}
	for _, id := range sortedByPath(diff, ordered) {
		h := diff[id]
		entry := hunk{ID: id, Path: h.DisplayPath, NewRange: lineRange{h.NewRange.Start, h.NewRange.Count}}
		if text, ok := cites[h.DisplayPath]; ok {
			start := strings.Index(doc, text)
			if start < 0 {
				f.t.Fatalf("%q not in doc.md at %s", text, base)
			}
			evidenceID := fmt.Sprintf("evidence:%s:%d", h.DisplayPath, start)
			out.Evidence = append(out.Evidence, evidence{ID: evidenceID, Path: "doc.md", BlobOid: docOid,
				Span: &span{int64(start), int64(start + len(text))}})
			entry.Basis = []basis{{EvidenceID: evidenceID, Relation: "specification"}}
		}
		out.Hunks = append(out.Hunks, entry)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(raw)
}

func sortedByPath(diff map[string]*patch.Hunk, ids []string) []string {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && diff[ids[j]].DisplayPath+ids[j] < diff[ids[j-1]].DisplayPath+ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

// TestInheritCitationsDCWV0824TwoLaneRange binds a range holding two sealed
// lanes and one unbound commit: an unchanged hunk inherits by hunk ID, a hunk
// the two lanes merged into one inherits from both by added-line overlap, each
// span is moved to BASE's offsets, and the rest stay visible unknowns.
func TestInheritCitationsDCWV0824TwoLaneRange(t *testing.T) {
	f := fixture{t: t, repo: t.TempDir()}
	f.git("init", "-q", "-b", "main")
	f.write("doc.md", "rule alpha holds\nrule beta holds\nrule gamma holds\n")
	f.write("a.txt", numbered("a", 12, nil))
	f.write("b.txt", numbered("b", 12, nil))
	base := f.commit("base")

	// Lane 1 edits a.txt line 2 and prepends a doc.md line; its CEM cites only a.txt.
	f.write("a.txt", numbered("a", 12, map[int]string{2: "alpha lane edit"}))
	f.write("doc.md", "preface\nrule alpha holds\nrule beta holds\nrule gamma holds\n")
	f.commit("lane 1")
	l1 := f.git("rev-parse", "HEAD")
	f.write(changeMapPath, f.cemFor(base, l1, map[string]string{"a.txt": "rule alpha holds"}))
	k1 := f.commit("chore: bind change evidence")

	// Lane 2, based on lane 1's binding, edits a.txt line 9 and b.txt line 5, cites both at its own
	// base (where doc.md is shifted by the preface), then seals its CEM under .corvint/changes/.
	f.write("a.txt", numbered("a", 12, map[int]string{2: "alpha lane edit", 9: "second lane edit"}))
	f.write("b.txt", numbered("b", 12, map[int]string{5: "beta lane edit"}))
	f.commit("lane 2")
	l2 := f.git("rev-parse", "HEAD")
	f.write(changeMapPath, f.cemFor(k1, l2, map[string]string{"a.txt": "rule beta holds", "b.txt": "rule gamma holds"}))
	k2 := f.commit("chore: bind change evidence")
	if err := os.MkdirAll(filepath.Join(f.repo, sealedDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	f.git("mv", changeMapPath, sealedDirectory+k2+".cem.json")
	f.commit("seal")

	// An unbound commit adds a file no constituent covers.
	f.write("c.txt", "unbound content line\n")
	target := f.commit("unbound")

	// The range map: every hunk of BASE..TARGET in patch order, all unknown.
	in := &inheritor{repo: f.repo, blobs: map[string][]byte{}}
	in.emptyTree, _ = in.git(nil, "hash-object", "-t", "tree", "--stdin")
	raw, err := in.gitBytes(nil, "--attr-source="+in.emptyTree, "diff", "--full-index", "--no-color", "--no-renames",
		"--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0",
		"--src-prefix=a/", "--dst-prefix=b/", base, target, "--", ".", ":(exclude)"+changeMapPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := patch.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var rangeMap document
	ordinal := map[string]int{}
	for i, h := range parsed.Hunks {
		rangeMap.Hunks = append(rangeMap.Hunks, hunk{ID: h.ID, Path: h.DisplayPath})
		ordinal[h.DisplayPath] = i + 1
	}
	mapRaw, _ := json.Marshal(rangeMap)
	dir := t.TempDir()
	mapPath, planPath, missPath := filepath.Join(dir, "map.json"), filepath.Join(dir, "plan.tsv"), filepath.Join(dir, "miss.tsv")
	if err := os.WriteFile(mapPath, mapRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	summary, err := run(f.repo, base, target, mapPath, planPath, missPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("hunks=%d inherited=2 proposed=0 rows=3 uninherited=%d constituents=2", len(parsed.Hunks), len(parsed.Hunks)-2); summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
	}
	plan, _ := os.ReadFile(planPath)
	// BASE's doc.md: "rule alpha holds" is 0:16, "rule beta holds" 17:32, "rule gamma holds" 33:49.
	// Sealed constituents are read before bind commits, so lane 2's row leads.
	wantPlan := fmt.Sprintf("%d\tdoc.md\tbytes:17:32\tspecification\n%d\tdoc.md\tbytes:0:16\tspecification\n%d\tdoc.md\tbytes:33:49\tspecification\n",
		ordinal["a.txt"], ordinal["a.txt"], ordinal["b.txt"])
	if string(plan) != wantPlan {
		t.Fatalf("plan:\n%s\nwant:\n%s", plan, wantPlan)
	}
	miss, _ := os.ReadFile(missPath)
	reasons := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(miss)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("uninherited row %q", line)
		}
		reasons[fields[2]] = fields[3]
	}
	want := map[string]string{
		"c.txt":                            "no-constituent-hunk",
		"doc.md":                           "constituent-hunk-unknown",
		sealedDirectory + k2 + ".cem.json": "no-constituent-hunk",
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Fatalf("uninherited = %v, want %v", reasons, want)
	}
}
