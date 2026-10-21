package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	cc "github.com/Beamfall/corvint/internal/cemcandidate"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// neutralGit runs Git with the recipe identity of the public neutral case.
func neutralGit(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@invalid", "GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
	cmd.Stdin = strings.NewReader(stdin)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

// neutralPlainRequest builds the public neutral-criterion case's recipe
// repository, runs the cem/1.0 producer (prepare, then one cite per hunk)
// and writes a plain assembly request over its uncommitted producer map and
// the case's runner artifacts. It returns the request path, the request and
// a new bundle path outside the repository.
func neutralPlainRequest(t *testing.T) (string, cc.PlainRequest, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	caseRoot := filepath.Join(filepath.Dir(file), "..", "..", "protocol", "cem-1.0", "stable", "neutral-criterion")
	var recipe struct {
		Commits []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	raw, e := os.ReadFile(filepath.Join(caseRoot, "repository.recipe.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw, &recipe); e != nil {
		t.Fatal(e)
	}
	temp, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	repo := filepath.Join(temp, "repo")
	if e := os.Mkdir(repo, 0o755); e != nil {
		t.Fatal(e)
	}
	neutralGit(t, repo, "", "init", "-q", "--object-format=sha1")
	base, parent := "", ""
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + neutralGit(t, repo, commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", neutralGit(t, repo, tree.String(), "mktree"), "-m", commit.Message}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		parent = neutralGit(t, repo, "", args...)
		if commit.Name == "base" {
			base = parent
		}
	}
	neutralGit(t, repo, "", "update-ref", "refs/heads/main", parent)
	neutralGit(t, repo, "", "symbolic-ref", "HEAD", "refs/heads/main")
	session, e := workflow.Open(repo)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if _, e := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: parent}); e != nil {
		t.Fatalf("prepare: %v", e)
	}
	for _, hunk := range []string{"1", "2"} {
		if _, e := session.Cite(ctx, workflow.CiteOptions{MapPath: cw.ExcludedCEMPath, Hunk: hunk, EvidencePath: "SPEC.md", Lines: "6:6", Relation: "specification"}); e != nil {
			t.Fatalf("cite hunk %s: %v", hunk, e)
		}
	}
	produced, e := os.ReadFile(filepath.Join(repo, cw.ExcludedCEMPath))
	if e != nil {
		t.Fatal(e)
	}
	source, e := cw.ParseStable(produced)
	if e != nil {
		t.Fatal(e)
	}
	if len(source.Change.Evidence) != 1 || len(source.Change.Hunks) != 2 {
		t.Fatalf("producer map has %d evidence and %d hunks, want 1 and 2", len(source.Change.Evidence), len(source.Change.Hunks))
	}
	inputs := filepath.Join(temp, "inputs")
	if e := os.Mkdir(inputs, 0o700); e != nil {
		t.Fatal(e)
	}
	save := func(name string, b []byte) cc.FileRef {
		p := filepath.Join(inputs, name)
		if e := os.WriteFile(p, b, 0o600); e != nil {
			t.Fatal(e)
		}
		return cc.FileRef{Path: p, Sha256: tr.Digest(b)}
	}
	artifact := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(caseRoot, "artifacts", name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	evidence := source.Change.Evidence[0].ID
	r := cc.PlainRequest{Profile: cc.PlainAssemblyProfile, Repository: repo, ExpectedBase: base, Target: parent,
		SourceMap: save("source.json", produced), RunnerPlan: save("runner-plan.json", artifact("runner-plan.json")), RunnerReceipt: save("runner-receipt.json", artifact("runner-receipt.json")),
		Criteria: []cc.PlainCriterion{{CriterionEvidenceID: evidence, HunkIDs: []string{source.Change.Hunks[0].ID, source.Change.Hunks[1].ID}, EvidenceIDs: []string{evidence}}}}
	encoded, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	request := filepath.Join(inputs, "request.json")
	if e := os.WriteFile(request, encoded, 0o600); e != nil {
		t.Fatal(e)
	}
	return request, r, filepath.Join(temp, "bundle")
}

// TestCEMV1350PlainAssemblyNextActionVerifiesBundle runs plain
// assemble-stable on the neutral case, then runs its single emitted next
// action unchanged through the Core stable entry point. The action names
// cem verify-stable over the bundle's stable.json with the bundle as the
// artifact root; it ACCEPTs with exit 0 and the same verification the
// assembly reported, with no copy of the map into the repository
// (CEM-V1-350, CEM-V1-351).
func TestCEMV1350PlainAssemblyNextActionVerifiesBundle(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	request, r, bundle := neutralPlainRequest(t)
	var out, errout bytes.Buffer
	if code := command(context.Background(), []string{"assemble-stable", "--experimental", "--request", request, "--out-dir", bundle}, &out, &errout); code != 0 {
		t.Fatalf("assemble-stable code=%d err=%s", code, errout.String())
	}
	var result struct {
		OK           bool            `json:"ok"`
		StablePath   string          `json:"stablePath"`
		Verification json.RawMessage `json:"verification"`
		NextActions  [][]string      `json:"nextActions"`
	}
	if e := json.Unmarshal(out.Bytes(), &result); e != nil {
		t.Fatalf("decode %s: %v", out.String(), e)
	}
	stablePath := filepath.Join(bundle, "stable.json")
	want := [][]string{{"corvint", "cem", "verify-stable", "--repository", r.Repository, "--map", stablePath, "--expected-base", r.ExpectedBase, "--target", r.Target, "--artifacts", bundle}}
	if !result.OK || result.StablePath != stablePath || !reflect.DeepEqual(result.NextActions, want) {
		t.Fatalf("result %s\nwant nextActions %v", out.String(), want)
	}
	if !bytes.HasSuffix(out.Bytes(), []byte(`,"nextActions":`+string(mustMarshal(t, want))+"}\n")) {
		t.Fatalf("nextActions is not the trailing member: %s", out.String())
	}
	before := neutralGit(t, r.Repository, "", "status", "--porcelain", "--untracked-files=all")
	action := result.NextActions[0]
	if action[0] != "corvint" {
		t.Fatalf("action %v does not name corvint", action)
	}
	var verified, verifyErr bytes.Buffer
	code, handled := cli.RunStableInvocation(context.Background(), action[1:], &verified, &verifyErr)
	if !handled || code != 0 {
		t.Fatalf("next action %v: handled=%v code=%d out=%s err=%s", action, handled, code, verified.String(), verifyErr.String())
	}
	var got, assembled map[string]any
	if e := json.Unmarshal(verified.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(result.Verification, &assembled); e != nil {
		t.Fatal(e)
	}
	if got["outcome"] != "ACCEPT" || got["accept"] != true || !reflect.DeepEqual(got, assembled) {
		t.Fatalf("next action result %s\nassembly verification %s", verified.String(), result.Verification)
	}
	if after := neutralGit(t, r.Repository, "", "status", "--porcelain", "--untracked-files=all"); after != before {
		t.Fatalf("repository status changed:\n%s\nwant:\n%s", after, before)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
