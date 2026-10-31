// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// bindRange runs the helper over the unknown map of BASE..TARGET and returns
// its summary, plan rows and uninherited rows keyed by path.
func (f fixture) bindRange(base, target string) (string, string, map[string]string) {
	f.t.Helper()
	in := &inheritor{repo: f.repo, blobs: map[string][]byte{}}
	in.emptyTree, _ = in.git(nil, "hash-object", "-t", "tree", "--stdin")
	raw, err := in.gitBytes(nil, "--attr-source="+in.emptyTree, "diff", "--full-index", "--no-color", "--no-renames",
		"--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0",
		"--src-prefix=a/", "--dst-prefix=b/", base, target, "--", ".", ":(exclude)"+changeMapPath)
	if err != nil {
		f.t.Fatal(err)
	}
	parsed, err := patch.Parse(raw)
	if err != nil {
		f.t.Fatal(err)
	}
	var rangeMap document
	for _, h := range parsed.Hunks {
		rangeMap.Hunks = append(rangeMap.Hunks, hunk{ID: h.ID, Path: h.DisplayPath})
	}
	mapRaw, _ := json.Marshal(rangeMap)
	dir := f.t.TempDir()
	mapPath, planPath, missPath := filepath.Join(dir, "map.json"), filepath.Join(dir, "plan.tsv"), filepath.Join(dir, "miss.tsv")
	if err := os.WriteFile(mapPath, mapRaw, 0o600); err != nil {
		f.t.Fatal(err)
	}
	summary, err := run(f.repo, base, target, mapPath, planPath, missPath)
	if err != nil {
		f.t.Fatal(err)
	}
	plan, _ := os.ReadFile(planPath)
	miss, _ := os.ReadFile(missPath)
	reasons := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(miss)), "\n") {
		if fields := strings.Split(line, "\t"); len(fields) == 4 {
			reasons[fields[2]] = fields[3]
		}
	}
	return summary, string(plan), reasons
}

func newFixture(t *testing.T) fixture {
	f := fixture{t: t, repo: t.TempDir()}
	f.git("init", "-q", "-b", "main")
	return f
}

// TestProposeSealedCEMRenameDCWV0829 binds a range whose only change beyond
// its lane is the seal commit moving the lane's sidecar CEM under
// .corvint/changes/: the byte-identical archive cites the DOGFOOD.md seal
// clause as one class, with no hand row, while an edited archive stays unknown.
func TestProposeSealedCEMRenameDCWV0829(t *testing.T) {
	f := newFixture(t)
	dogfood := "# Dogfood\n\nA seal commit, which " + sealClause + ".\n"
	f.write(dogfoodPath, dogfood)
	f.write("a.txt", "one\n")
	base := f.commit("base")

	f.write("a.txt", "two\n")
	lane := f.commit("lane")
	f.write(changeMapPath, `{"baseRevision":"`+base+`","hunks":[]}`+"\n")
	k := f.commit("chore: bind change evidence")
	if err := os.MkdirAll(filepath.Join(f.repo, sealedDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	f.git("mv", changeMapPath, sealedDirectory+k+".cem.json")
	target := f.commit("seal")
	_ = lane

	summary, plan, reasons := f.bindRange(base, target)
	at := strings.Index(dogfood, sealClause)
	if want := fmt.Sprintf("1\t%s\tbytes:%d:%d\tspecification\n", dogfoodPath, at, at+len(sealClause)); !strings.Contains(plan, want) {
		t.Fatalf("plan:\n%s\nwant row %q", plan, want)
	}
	if _, ok := reasons[sealedDirectory+k+".cem.json"]; ok || !strings.Contains(summary, "proposed=1 ") {
		t.Fatalf("summary %q reasons %v: the sealed rename must be proposed", summary, reasons)
	}
	if reasons["a.txt"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v, want a.txt no-constituent-hunk", reasons)
	}

	// An archive whose bytes differ from every sidecar the range wrote is an edit, not a rename.
	f.write(sealedDirectory+k+".cem.json", `{"baseRevision":"`+base+`","hunks":[],"edited":true}`+"\n")
	edited := f.commit("edit the archive")
	summary, _, reasons = f.bindRange(base, edited)
	if reasons[sealedDirectory+k+".cem.json"] != "no-constituent-hunk" || !strings.Contains(summary, "proposed=0 ") {
		t.Fatalf("summary %q reasons %v: an edited archive must stay unknown", summary, reasons)
	}
}

func copyRepoScript(f fixture, name string) {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "script", name))
	if err != nil {
		f.t.Fatal(err)
	}
	f.write("script/"+name, string(data))
}

func (f fixture) regenerateSpecIndex() {
	f.t.Helper()
	f.git("add", "-A")
	out := f.t.TempDir()
	cmd := exec.Command("sh", "script/gen-spec-requirements.sh", "--all", out)
	cmd.Dir = f.repo
	if msg, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("gen-spec-requirements: %v\n%s", err, msg)
	}
	for _, name := range []string{"REQUIREMENTS.tsv", "INDEX.json", "README.md"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			f.t.Fatal(err)
		}
		f.write("docs/specs/"+name, string(data))
	}
}

func specFixture(title, id, text string) string {
	return "# " + title + "\nOwner: t\nIntent status: proposed\nDelivery status: experimental\n## Agent digest\n- Claim: " +
		strings.ToLower(title) + ".\n## Requirements\n- `" + id + "`: " + text + "\n"
}

// TestProposeGeneratedSpecIndexDCWV0828 regenerates REQUIREMENTS.tsv,
// INDEX.json and README.md with the fixture's own copy of
// script/gen-spec-requirements.sh: every hunk the generator reproduces at
// TARGET cites SRG-V0-012 and ASD-V0-033 at BASE, and a hand edit to
// REQUIREMENTS.tsv that the generator does not reproduce stays unknown.
func TestProposeGeneratedSpecIndexDCWV0828(t *testing.T) {
	f := newFixture(t)
	copyRepoScript(f, "gen-spec-requirements.sh")
	srg := specFixture("Spec Requirement Index Generator V0", "SRG-V0-012", "The generator MUST project the staged spec headers.")
	asd := specFixture("Corvint Self Development V0", "ASD-V0-033", "Batch regeneration MUST rerun every generator.")
	f.write("docs/specs/spec-requirement-index-generator-v0.md", srg)
	f.write("docs/specs/corvint-self-development-v0.md", asd)
	f.write("docs/specs/README.md", "# Specs\n\n| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n|---|---|---:|---:|---|\n"+
		"| SRG V0 | [`spec-requirement-index-generator-v0.md`](spec-requirement-index-generator-v0.md) SRG-V0-012 | proposed | experimental | It holds. |\n"+
		"| ASD V0 | [`corvint-self-development-v0.md`](corvint-self-development-v0.md) ASD-V0-033 | proposed | experimental | It holds. |\n")
	f.regenerateSpecIndex()
	base := f.commit("base")

	// The change adds a spec and regenerates the index: every index hunk is generated.
	f.write("docs/specs/demo-v0.md", specFixture("Demo V0", "DEMO-V0-001", "The demo MUST hold."))
	readme, _ := os.ReadFile(filepath.Join(f.repo, "docs/specs/README.md"))
	f.write("docs/specs/README.md", string(readme)+"| Demo V0 | [`demo-v0.md`](demo-v0.md) DEMO-V0-001 | proposed | experimental | The demo holds. |\n")
	f.regenerateSpecIndex()
	target := f.commit("demo spec")

	summary, plan, reasons := f.bindRange(base, target)
	for _, path := range []string{"docs/specs/REQUIREMENTS.tsv", "docs/specs/INDEX.json", "docs/specs/README.md"} {
		if _, ok := reasons[path]; ok {
			t.Fatalf("%s stayed unknown (%v); summary %q", path, reasons, summary)
		}
	}
	if reasons["docs/specs/demo-v0.md"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v: the hand-written spec must stay unknown", reasons)
	}
	srgAt := strings.Index(srg, "- `SRG-V0-012`:")
	asdAt := strings.Index(asd, "- `ASD-V0-033`:")
	for _, want := range []string{
		fmt.Sprintf("\tdocs/specs/spec-requirement-index-generator-v0.md\tbytes:%d:%d\tspecification\n", srgAt, len(srg)-1),
		fmt.Sprintf("\tdocs/specs/corvint-self-development-v0.md\tbytes:%d:%d\tspecification\n", asdAt, len(asd)-1),
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("plan:\n%s\nwant row %q", plan, want)
		}
	}

	// A hand edit to REQUIREMENTS.tsv is not what the generator writes, so it stays unknown;
	// INDEX.json is still byte-identical to the generator's output and stays proposed.
	tsv, _ := os.ReadFile(filepath.Join(f.repo, "docs/specs/REQUIREMENTS.tsv"))
	f.write("docs/specs/REQUIREMENTS.tsv", string(tsv)+"HAND-V0-001\tdocs/specs/demo-v0.md\n")
	hand := f.commit("hand edit")
	_, _, reasons = f.bindRange(base, hand)
	if reasons["docs/specs/REQUIREMENTS.tsv"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v: a hand-edited generated file must stay unknown", reasons)
	}
	if _, ok := reasons["docs/specs/INDEX.json"]; ok {
		t.Fatalf("reasons = %v: INDEX.json is still generated", reasons)
	}
}

// TestProposeUseCaseReceiptRepinDCWV0828 proposes UCV0-004 and UCV0-005 for a
// receipt hunk that only repins a digest to the SHA-256 of the TARGET file the
// receipt names, and leaves a digest naming no such file unknown.
func TestProposeUseCaseReceiptRepinDCWV0828(t *testing.T) {
	f := newFixture(t)
	ucv := "# Use Case Conformance V0\n\n- `UCV0-004`: A receipt MUST pin its inputs.\n- `UCV0-005`: The ledger MUST pin each receipt.\n\n## Next\n"
	f.write("docs/specs/use-case-conformance-v0.md", ucv)
	receipt := func(content string) string {
		sum := sha256.Sum256([]byte(content))
		return "{\n  \"path\": \"cmd/demo/main.go\",\n  \"sha256\": \"" + hex.EncodeToString(sum[:]) + "\"\n}\n"
	}
	f.write("cmd/demo/main.go", "package main\n")
	f.write("conformance/use-cases-v0/receipts/demo.json", receipt("package main\n"))
	base := f.commit("base")

	f.write("cmd/demo/main.go", "package main\n\nfunc main() {}\n")
	f.write("conformance/use-cases-v0/receipts/demo.json", receipt("package main\n\nfunc main() {}\n"))
	target := f.commit("edit and repin")
	summary, plan, reasons := f.bindRange(base, target)
	if _, ok := reasons["conformance/use-cases-v0/receipts/demo.json"]; ok || !strings.Contains(summary, "proposed=1 ") {
		t.Fatalf("summary %q reasons %v: the repinned receipt must be proposed", summary, reasons)
	}
	at4, at5 := strings.Index(ucv, "- `UCV0-004`:"), strings.Index(ucv, "- `UCV0-005`:")
	want := fmt.Sprintf("\tdocs/specs/use-case-conformance-v0.md\tbytes:%d:%d\tspecification\n", at4, at5-1) +
		fmt.Sprintf("%s\tdocs/specs/use-case-conformance-v0.md\tbytes:%d:%d\tspecification\n",
			strings.SplitN(plan, "\t", 2)[0], at5, strings.Index(ucv, "\n\n## Next"))
	if !strings.Contains(plan, want) {
		t.Fatalf("plan:\n%s\nwant rows %q", plan, want)
	}

	f.write("conformance/use-cases-v0/receipts/demo.json", receipt("not the tracked content\n"))
	wrong := f.commit("wrong pin")
	_, _, reasons = f.bindRange(base, wrong)
	if reasons["conformance/use-cases-v0/receipts/demo.json"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v: a digest that pins no tracked file must stay unknown", reasons)
	}
}

// TestProposeLineCitationRepinDCWV0828 proposes DCG-V0-007 for a document hunk
// that only moves a pinned `path:N@anchor` citation, when TARGET's own copy of
// script/check-line-citations.sh passes; a repin the gate refuses stays unknown.
func TestProposeLineCitationRepinDCWV0828(t *testing.T) {
	f := newFixture(t)
	copyRepoScript(f, "check-line-citations.sh")
	f.write("script/line-citation-legacy-documents.txt", "")
	f.write("script/line-citation-unpinned-inventory.txt", "")
	dcg := "# Documentation Citation Gate V0\n\n- `DCG-V0-007`: A moved citation MUST be repinned.\n"
	f.write("docs/specs/documentation-citation-gate-v0.md", dcg)
	f.write("src/a.go", "package a\n\nfunc A() {}\n")
	f.git("add", "-A")
	anchor := func(citation string) string {
		cmd := exec.Command("sh", "script/check-line-citations.sh", "--hash", citation)
		cmd.Dir = f.repo
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--hash %s: %v", citation, err)
		}
		return strings.TrimSpace(string(out))
	}
	doc := func(citation string) string { return "# Notes\n\nSee `" + citation + "` for A.\n" }
	f.write("docs/specs/notes-v0.md", doc(anchor("src/a.go:3")))
	base := f.commit("base")

	f.write("src/a.go", "package a\n\n// A is the entry point.\nfunc A() {}\n")
	f.git("add", "-A")
	f.write("docs/specs/notes-v0.md", doc(anchor("src/a.go:4")))
	target := f.commit("move and repin")
	summary, plan, reasons := f.bindRange(base, target)
	if _, ok := reasons["docs/specs/notes-v0.md"]; ok || !strings.Contains(summary, "proposed=1 ") {
		t.Fatalf("summary %q reasons %v: the repinned citation must be proposed", summary, reasons)
	}
	at := strings.Index(dcg, "- `DCG-V0-007`:")
	if want := fmt.Sprintf("\tdocs/specs/documentation-citation-gate-v0.md\tbytes:%d:%d\tspecification\n", at, len(dcg)-1); !strings.Contains(plan, want) {
		t.Fatalf("plan:\n%s\nwant row %q", plan, want)
	}
	if reasons["src/a.go"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v: the code edit must stay unknown", reasons)
	}

	// A repin onto the blank line is refused by the gate, so it is not a generated repin.
	f.write("docs/specs/notes-v0.md", doc("src/a.go:2"))
	refused := f.commit("bad repin")
	_, _, reasons = f.bindRange(base, refused)
	if reasons["docs/specs/notes-v0.md"] != "no-constituent-hunk" {
		t.Fatalf("reasons = %v: a repin the gate refuses must stay unknown", reasons)
	}
}
