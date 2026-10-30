//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// verdictFixture is one dogfood-shaped change: base, change, bind (the
// producer map committed as the sidecar) and seal (the sidecar renamed to
// .corvint/changes/<bind>.cem.json). The src/app.txt hunk cites a span naming
// its path (anchored); the lib/util.txt hunk cites an unrelated span (cited).
type verdictFixture struct {
	root, base, change, bind, seal string
	sealed                         string
	portable                       string
	git                            func(args ...string) string
}

func newVerdictFixture(t *testing.T, mutate func([]byte) []byte) verdictFixture {
	t.Helper()
	stablefixture.RequireStableHost(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(path, data string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	put("docs/rule.txt", "src/app.txt renders the widget\n")
	put("docs/note.txt", "a general note\n")
	put("src/app.txt", "alpha\n")
	put("lib/util.txt", "one\n")
	run("add", ".")
	run("commit", "-qm", "base")
	f := verdictFixture{root: root, base: run("rev-parse", "HEAD"), git: run, portable: portableConsumer(t)}
	put("src/app.txt", "alpha\nwidget\n")
	put("lib/util.txt", "one\ntwo\n")
	run("commit", "-qam", "change")
	f.change = run("rev-parse", "HEAD")
	ctx := context.Background()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: f.base, Target: f.change}); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	prepared, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Hunks []struct{ ID, Path string } `json:"hunks"`
	}
	if err := json.Unmarshal(prepared, &shape); err != nil || len(shape.Hunks) != 2 {
		t.Fatalf("fixture expects two hunks: %v %s", err, prepared)
	}
	evidence := map[string]string{"src/app.txt": "docs/rule.txt", "lib/util.txt": "docs/note.txt"}
	for _, hunk := range shape.Hunks {
		if session, err = workflow.Open(root); err != nil {
			t.Fatal(err)
		}
		if _, err := session.Cite(ctx, workflow.CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: hunk.ID, EvidencePath: evidence[hunk.Path], Lines: "1:1", Relation: "specification"}); err != nil {
			t.Fatal(err)
		}
	}
	if mutate != nil {
		cited, err := os.ReadFile(sidecar)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, mutate(cited), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-f", wire.ExcludedCEMPath)
	run("commit", "-qm", "chore: bind change evidence")
	f.bind = run("rev-parse", "HEAD")
	f.sealed = sealedMapDir + f.bind + ".cem.json"
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("mv", wire.ExcludedCEMPath, f.sealed)
	run("commit", "-qm", "chore: seal change evidence")
	f.seal = run("rev-parse", "HEAD")
	return f
}

// commitAt detaches at from, applies edit to the worktree and commits it.
func (f verdictFixture) commitAt(t *testing.T, from string, edit func()) string {
	t.Helper()
	f.git("checkout", "-q", "--detach", from)
	edit()
	f.git("add", "-A", "-f", ".")
	f.git("commit", "-qm", "variant")
	return f.git("rev-parse", "HEAD")
}

func (f verdictFixture) input(predicate string, touch ...string) CEMVerdictInput {
	return CEMVerdictInput{Root: f.root, BaseCommit: f.base, CandidateCommit: f.seal, TouchPaths: touch, Predicate: predicate, PortableConsumer: f.portable}
}

type verdictView struct {
	Outcome       string                 `json:"outcome"`
	Code          *string                `json:"code"`
	Reason        *string                `json:"reason"`
	Inputs        *cemVerdictInputs      `json:"inputs"`
	CandidateTree *string                `json:"candidateTree"`
	SealedMaps    []string               `json:"sealedMaps"`
	Map           *cemVerdictMap         `json:"map"`
	Verdict       *cemVerdictPin         `json:"verdict"`
	Portable      *cemPortablePin        `json:"portable"`
	Citation      json.RawMessage        `json:"citation"`
	Governed      []cemVerdictHunk       `json:"governed"`
	Paths         []cemVerdictPath       `json:"paths"`
	Uncertainty   *cemVerdictUncertainty `json:"uncertainty"`
}

func evaluateVerdict(t *testing.T, in CEMVerdictInput) (CEMVerdictResult, verdictView) {
	t.Helper()
	result, err := EvaluateCEMVerdictGate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document(result.Document, 1<<20); err != nil {
		t.Fatalf("gate evidence is not one canonical document: %v\n%s", err, result.Document)
	}
	var view verdictView
	if err := json.Unmarshal(result.Document, &view); err != nil {
		t.Fatal(err)
	}
	return result, view
}

func requireVerdictRefusal(t *testing.T, in CEMVerdictInput, code, reason string) verdictView {
	t.Helper()
	result, view := evaluateVerdict(t, in)
	got := ""
	if view.Reason != nil {
		got = *view.Reason
	}
	if result.Granted || result.Code != code || view.Outcome != "REFUSE" || view.Code == nil || *view.Code != code || got != reason {
		t.Fatalf("want %s/%s refusal, got granted=%v code=%q\n%s", code, reason, result.Granted, result.Code, result.Document)
	}
	return view
}

func TestTCBV0007GateInputsAreNativeAndClosed(t *testing.T) {
	f := newVerdictFixture(t, nil)
	valid := f.input(PredicateTouchedAnchored, "src/")
	for name, edit := range map[string]func(*CEMVerdictInput){
		"relative root":         func(in *CEMVerdictInput) { in.Root = "repo" },
		"unclean root":          func(in *CEMVerdictInput) { in.Root = f.root + "/../" + filepath.Base(f.root) },
		"abbreviated base":      func(in *CEMVerdictInput) { in.BaseCommit = f.base[:12] },
		"uppercase candidate":   func(in *CEMVerdictInput) { in.CandidateCommit = strings.ToUpper(f.seal) },
		"unknown predicate":     func(in *CEMVerdictInput) { in.Predicate = "held-at-least" },
		"no touch paths":        func(in *CEMVerdictInput) { in.TouchPaths = nil },
		"escaping touch path":   func(in *CEMVerdictInput) { in.TouchPaths = []string{"../src/"} },
		"absolute touch path":   func(in *CEMVerdictInput) { in.TouchPaths = []string{"/src/"} },
		"mixed object id sizes": func(in *CEMVerdictInput) { in.CandidateCommit = f.seal + strings.Repeat("0", 24) },
		"relative consumer":     func(in *CEMVerdictInput) { in.PortableConsumer = "cem01-go" },
		"no consumer":           func(in *CEMVerdictInput) { in.PortableConsumer = "" },
	} {
		in := valid
		edit(&in)
		if view := requireVerdictRefusal(t, in, CEMVerdictInputInvalid, "invalid-input"); view.Inputs != nil {
			t.Fatalf("%s: a malformed input is pinned: %+v", name, view.Inputs)
		}
	}
	unknown := valid
	unknown.CandidateCommit = strings.Repeat("e", 40)
	if view := requireVerdictRefusal(t, unknown, CEMVerdictInputInvalid, "commit-unavailable"); view.Inputs == nil || view.Inputs.CandidateCommit != unknown.CandidateCommit {
		t.Fatalf("well-formed unresolvable input is not pinned: %+v", view.Inputs)
	}
	duplicate := f.input(PredicateTouchedAnchored, "src/", "docs/", "src/")
	if _, view := evaluateVerdict(t, duplicate); strings.Join(view.Inputs.TouchPaths, ",") != "docs/,src/" {
		t.Fatalf("touch paths are not pinned sorted and unique: %v", view.Inputs.TouchPaths)
	}
}

func TestTCBV0008SealedMapIsReadFromCandidateGit(t *testing.T) {
	f := newVerdictFixture(t, nil)
	for _, candidate := range []string{f.change, f.bind} {
		in := f.input(PredicateTouchedAnchored, "src/")
		in.CandidateCommit = candidate
		requireVerdictRefusal(t, in, CEMVerdictMapMissing, "")
	}
	// A worktree copy of the sealed map is not evidence: only Git objects at
	// the candidate are read.
	f.git("checkout", "-q", "--detach", f.bind)
	sealedBytes := []byte(f.git("show", f.seal+":"+f.sealed) + "\n")
	if err := os.MkdirAll(filepath.Join(f.root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(f.sealed)), sealedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	in := f.input(PredicateTouchedAnchored, "src/")
	in.CandidateCommit = f.bind
	requireVerdictRefusal(t, in, CEMVerdictMapMissing, "")
	if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(f.sealed))); err != nil {
		t.Fatal(err)
	}
	second := sealedMapDir + f.change + ".cem.json"
	twice := f.commitAt(t, f.seal, func() {
		if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(second)), sealedBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	in.CandidateCommit = twice
	view := requireVerdictRefusal(t, in, CEMVerdictMapAmbiguous, "")
	if len(view.SealedMaps) != 2 || view.Map != nil || view.Verdict != nil {
		t.Fatalf("ambiguous candidate must pin both sealed maps and no verdict: %+v", view)
	}
}

func TestTCBV0009ProducerFieldsDisagreeingRefuse(t *testing.T) {
	f := newVerdictFixture(t, nil)
	sealedPath := filepath.Join(f.root, filepath.FromSlash(f.sealed))
	// The map's baseRevision is a producer field: the attempt base outranks it.
	wrongBase := f.input(PredicateTouchedAnchored, "src/")
	wrongBase.BaseCommit = f.change
	if view := requireVerdictRefusal(t, wrongBase, CEMVerdictProducerMismatch, "sealed-base-mismatch"); view.Verdict != nil {
		t.Fatalf("a base mismatch reached the consumer: %+v", view.Verdict)
	}
	// A later code change is not covered by the sealed verdict.
	drifted := f.commitAt(t, f.seal, func() {
		if err := os.WriteFile(filepath.Join(f.root, "src", "app.txt"), []byte("alpha\nwidget\ngadget\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	in := f.input(PredicateTouchedAnchored, "src/")
	in.CandidateCommit = drifted
	requireVerdictRefusal(t, in, CEMVerdictProducerMismatch, "sealed-tree-mismatch")
	// Tamper: the sealed bytes differ from the bind commit's sidecar.
	tampered := f.commitAt(t, f.seal, func() {
		raw, err := os.ReadFile(sealedPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sealedPath, bytes.Replace(raw, []byte(`"specification"`), []byte(`"implementation"`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	in.CandidateCommit = tampered
	requireVerdictRefusal(t, in, CEMVerdictProducerMismatch, "sealed-tree-mismatch")
	// The sealed name names a target that is no commit.
	sealedBytes := []byte(f.git("show", f.seal+":"+f.sealed) + "\n")
	ghost := f.commitAt(t, f.change, func() {
		path := filepath.Join(f.root, ".corvint", "changes", strings.Repeat("d", 40)+".cem.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, sealedBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	in.CandidateCommit = ghost
	requireVerdictRefusal(t, in, CEMVerdictProducerMismatch, "sealed-target-unavailable")
}

func TestTCBV0010UnverifiableMapRefuses(t *testing.T) {
	// The producer's patchSha256 disagrees with the consumer's recomputed
	// patch; bind and seal carry the same wrong bytes, so only the in-process
	// stable consumer can refuse it.
	f := newVerdictFixture(t, func(raw []byte) []byte {
		var shape struct {
			PatchSha256 string `json:"patchSha256"`
		}
		if err := json.Unmarshal(raw, &shape); err != nil || len(shape.PatchSha256) != 64 {
			t.Fatalf("patchSha256: %v", err)
		}
		wrong := strings.Repeat("0", 64)
		if shape.PatchSha256 == wrong {
			wrong = strings.Repeat("1", 64)
		}
		return bytes.Replace(raw, []byte(shape.PatchSha256), []byte(wrong), 1)
	})
	// A root that is no repository is a read failure, not an absent commit.
	unreadable := f.input(PredicateTouchedAnchored, "src/")
	unreadable.Root = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(unreadable.Root); err == nil {
		unreadable.Root = resolved
	}
	requireVerdictRefusal(t, unreadable, CEMVerdictUnverifiable, "git-read-failed")
	view := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, "src/"), CEMVerdictUnverifiable, "stable-not-accepted")
	if view.Verdict == nil || view.Verdict.Outcome == "ACCEPT" || len(view.Verdict.VerdictSha256) != 64 || string(view.Citation) != "null" {
		t.Fatalf("an unverifiable map must pin the consumer's non-ACCEPT verdict and no citation: %+v %s", view.Verdict, view.Citation)
	}
}

func TestTCBV0011PredicateIsClosedAndNeverRaisesATier(t *testing.T) {
	f := newVerdictFixture(t, nil)
	cited := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, "lib/util.txt"), CEMVerdictPredicateFailed, "tier-below-anchored")
	if len(cited.Governed) != 1 || cited.Governed[0].Path != "lib/util.txt" || cited.Governed[0].Tier != verify.TierCited || cited.Governed[0].Satisfied {
		t.Fatalf("the cited hunk must stay cited and unsatisfied: %+v", cited.Governed)
	}
	all := requireVerdictRefusal(t, f.input(PredicateAllAnchored), CEMVerdictPredicateFailed, "tier-below-anchored")
	if len(all.Governed) != 2 {
		t.Fatalf("all-anchored governs every hunk: %+v", all.Governed)
	}
	var citation verify.Citation
	if err := json.Unmarshal(all.Citation, &citation); err != nil {
		t.Fatal(err)
	}
	for i, row := range citation.Hunks {
		if all.Governed[i].HunkID != row.HunkID || all.Governed[i].Tier != row.Tier {
			t.Fatalf("governed tier differs from the recomputed citation: %+v %+v", all.Governed[i], row)
		}
	}
	requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, "src/", "lib/"), CEMVerdictPredicateFailed, "tier-below-anchored")
	if view := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, "docs/"), CEMVerdictPredicateFailed, "no-governed-hunk"); len(view.Governed) != 0 {
		t.Fatalf("no hunk lies under docs/: %+v", view.Governed)
	}
	// A key without the trailing slash governs only that exact path, and one
	// naming a directory refuses (TCB-V0-023).
	requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, "src"), CEMVerdictInputInvalid, "touch-key-names-directory")
}

func TestTCBV0012GrantPinsVerdictAndCitation(t *testing.T) {
	f := newVerdictFixture(t, nil)
	for _, touch := range [][]string{{"src/"}, {"src/app.txt"}} {
		in := f.input(PredicateTouchedAnchored, touch...)
		result, view := evaluateVerdict(t, in)
		if !result.Granted || result.Code != "" || view.Outcome != "GRANT" || view.Code != nil || view.Reason != nil {
			t.Fatalf("anchored touched hunk must grant: %s", result.Document)
		}
		raw := []byte(f.git("show", f.seal+":"+f.sealed) + "\n")
		independent, _ := verify.Stable(context.Background(), raw, verify.StableOptions{Repository: f.root, ExpectedBase: f.base, Target: f.bind, ArtifactRoot: f.root})
		verdict, err := independent.VerdictDigest()
		if err != nil {
			t.Fatal(err)
		}
		citation, err := json.Marshal(independent.Citation)
		if err != nil {
			t.Fatal(err)
		}
		if view.Verdict.Outcome != "ACCEPT" || view.Verdict.Sidecar != "EXACT" || view.Verdict.VerdictSha256 != verdict {
			t.Fatalf("pinned verdict differs from an independent consumer: %+v want %s", view.Verdict, verdict)
		}
		if !bytes.Equal(canonical(mustParse(t, view.Citation)), canonical(mustParse(t, citation))) {
			t.Fatalf("pinned citation differs from the recomputed block:\n%s\n%s", view.Citation, citation)
		}
		if view.Map == nil || view.Map.Path != f.sealed || view.Map.Target != f.bind || *view.Map.MapSha256 != sum(raw) || *view.CandidateTree != f.git("rev-parse", f.seal+"^{tree}") {
			t.Fatalf("sealed map pins: %+v %v", view.Map, *view.CandidateTree)
		}
		if len(view.Governed) != 1 || view.Governed[0].Path != "src/app.txt" || view.Governed[0].Tier != verify.TierAnchored || !view.Governed[0].Satisfied {
			t.Fatalf("governed: %+v", view.Governed)
		}
		again, _ := evaluateVerdict(t, in)
		if !bytes.Equal(again.Document, result.Document) || bytes.Contains(result.Document, []byte(f.root)) {
			t.Fatal("gate evidence must be deterministic and free of host paths")
		}
	}
}

func TestTCBV0013TamperedEvidenceRefusesOnReverify(t *testing.T) {
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	pinned, view := evaluateVerdict(t, in)
	if !pinned.Granted {
		t.Fatalf("fixture must grant: %s", pinned.Document)
	}
	// A consumer with no Tasks store reads the inputs from the pinned document.
	storeless := CEMVerdictInput{Root: f.root, BaseCommit: view.Inputs.BaseCommit, CandidateCommit: view.Inputs.CandidateCommit, TouchPaths: view.Inputs.TouchPaths, Predicate: view.Inputs.Predicate, PortableConsumer: f.portable}
	if again, err := ReverifyCEMVerdictGate(context.Background(), storeless, pinned.Document); err != nil || !again.Granted || again.Code != "" {
		t.Fatalf("untampered evidence must re-verify: %v %+v", err, again)
	}
	verdict := view.Verdict.VerdictSha256
	flipped := "0"
	if verdict[0] == '0' {
		flipped = "1"
	}
	for name, tampered := range map[string][]byte{
		"edited verdictSha256": bytes.Replace(pinned.Document, []byte(verdict), []byte(flipped+verdict[1:]), 1),
		"raised tier":          bytes.Replace(pinned.Document, []byte(`"tier":"cited"`), []byte(`"tier":"held"`), 1),
		"dropped citation":     bytes.Replace(pinned.Document, []byte(`"citation":{`), []byte(`"citation":null,"x":{`), 1),
	} {
		if bytes.Equal(tampered, pinned.Document) {
			t.Fatalf("%s: tamper did not change the evidence", name)
		}
		again, err := ReverifyCEMVerdictGate(context.Background(), in, tampered)
		if err != nil || again.Granted || again.Code != CEMVerdictProducerMismatch || !bytes.Equal(again.Document, pinned.Document) {
			t.Fatalf("%s: tampered evidence must refuse as a producer mismatch: %v %+v", name, err, again)
		}
	}
	// Evidence pinned for other native facts does not re-verify for these.
	other := in
	other.TouchPaths = []string{"src/app.txt"}
	if again, _ := ReverifyCEMVerdictGate(context.Background(), other, pinned.Document); again.Granted || again.Code != CEMVerdictProducerMismatch {
		t.Fatalf("evidence must bind its native inputs: %+v", again)
	}
}

func TestTCBV0014GateIsReadOnly(t *testing.T) {
	f := newVerdictFixture(t, nil)
	state := func() string {
		return strings.Join([]string{
			f.git("status", "--porcelain=v1", "--ignored", "--untracked-files=all"),
			f.git("for-each-ref"), f.git("rev-parse", "HEAD"), f.git("count-objects", "-v"),
		}, "\n")
	}
	before := state()
	for _, in := range []CEMVerdictInput{f.input(PredicateTouchedAnchored, "src/"), f.input(PredicateAllAnchored)} {
		if _, err := EvaluateCEMVerdictGate(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	if after := state(); after != before {
		t.Fatalf("the gate mutated the repository:\n%s\n---\n%s", before, after)
	}
}

func mustParse(t *testing.T, raw []byte) wire.Value {
	t.Helper()
	value, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
