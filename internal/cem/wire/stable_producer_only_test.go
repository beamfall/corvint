package wire_test

// Native side of the CEM-V1-094 producer-only acceptance: the public neutral
// map with its four producer-only members emitted empty verifies ACCEPT with
// an EMPTY_REFERENCE_SET reference axis and every authority axis NOT_OBSERVED,
// and adding provenance and role (CEM-V1-093, CEM-V1-095) changes nothing but
// the map digest. It lives here so the verifier package needs no edit.

import (
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

	"github.com/Beamfall/corvint/internal/cem/verify"
)

func producerOnlyGit(t *testing.T, repo, stdin string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@invalid",
		"GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
	command.Stdin = strings.NewReader(stdin)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// producerOnlyRepository builds the neutral recipe repository.
func producerOnlyRepository(t *testing.T, root string) (repo, base, target string) {
	t.Helper()
	var recipe struct {
		Identity struct{ Name, Email, Date string } `json:"identity"`
		Commits  []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	data, err := os.ReadFile(filepath.Join(root, "repository.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	if recipe.Identity.Name != "Fixture" || recipe.Identity.Email != "fixture@invalid" || recipe.Identity.Date != "2026-10-01T00:00:00Z" {
		t.Fatalf("recipe identity %+v differs from the helper identity", recipe.Identity)
	}
	fixture, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(fixture, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	producerOnlyGit(t, repo, "", "init", "-q", "--object-format=sha1")
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + producerOnlyGit(t, repo, commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", producerOnlyGit(t, repo, tree.String(), "mktree"), "-m", commit.Message}
		if target != "" {
			args = append(args, "-p", target)
		}
		target = producerOnlyGit(t, repo, "", args...)
		if commit.Name == "base" {
			base = target
		}
	}
	producerOnlyGit(t, repo, "", "update-ref", "refs/heads/main", target)
	producerOnlyGit(t, repo, "", "symbolic-ref", "HEAD", "refs/heads/main")
	return repo, base, target
}

func producerOnlyResult(t *testing.T, r verify.StableResult) map[string]any {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "runtime")
	return m
}

func producerOnlyMap(t *testing.T, base []byte, set map[string]string) []byte {
	t.Helper()
	var root map[string]json.RawMessage
	if err := json.Unmarshal(base, &root); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		root[k] = json.RawMessage(v)
	}
	out, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCEMV1094ProducerOnlyEmptyVerifyAccept(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "stable", "neutral-criterion"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	repo, base, target := producerOnlyRepository(t, root)
	plain := producerOnlyMap(t, raw, map[string]string{"criterionBindings": `[]`, "runnerReceipts": `[]`, "criterionLinks": `[]`, "artifacts": `[]`})
	options := verify.StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: root}
	r, exit := verify.Stable(context.Background(), plain, options)
	if exit != 0 || r.Outcome != "ACCEPT" || r.Axes["referenceIntegrity"] != "EMPTY_REFERENCE_SET" || r.Axes["changeIntegrity"] != "VERIFIED" {
		b, _ := json.Marshal(r)
		t.Fatalf("Stable exit=%d result=%s, want ACCEPT with EMPTY_REFERENCE_SET", exit, b)
	}
	checked := map[string]bool{"changeIntegrity": true, "referenceIntegrity": true, "structuralProof": true, "coverageValidation": true, "discriminationValidation": true}
	for axis, value := range r.Axes {
		if !checked[axis] && value != "NOT_OBSERVED" {
			t.Fatalf("axis %s = %s, want NOT_OBSERVED", axis, value)
		}
	}
	marked := producerOnlyMap(t, plain, map[string]string{"role": `"seal-chore"`, "provenance": `{"sourceSpec":"cem/0.2","sourceBlobOid":"` + base + `","sourceSha256":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","bindCommit":"` + target + `","converter":"corvint-cem-convert/1"}`})
	again, exit2 := verify.Stable(context.Background(), marked, options)
	first, second := producerOnlyResult(t, r), producerOnlyResult(t, again)
	if first["mapSha256"] == second["mapSha256"] {
		t.Fatal("marked map has the same digest")
	}
	if first["verdictSha256"] == second["verdictSha256"] {
		t.Fatal("verdict digest did not move with mapSha256 (CEM-V1-066)")
	}
	for _, member := range []string{"mapSha256", "verdictSha256"} {
		delete(first, member)
		delete(second, member)
	}
	if exit2 != exit || !reflect.DeepEqual(first, second) {
		b, _ := json.Marshal(again)
		t.Fatalf("provenance and role changed the result: exit=%d %s", exit2, b)
	}
}
