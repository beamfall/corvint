package main

// CEM-V1-083 portable conformance: the public no-Tasks case under
// protocol/cem-1.0/stable/neutral-criterion verifies with the exact public
// expected result, its refusal mutations fail at their recorded stage and
// code, and the Tasks-bound S0E criteria keep their exact form.

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
)

type neutralRefusal struct {
	Name       string          `json:"name"`
	Criterion  json.RawMessage `json:"criterion"`
	RebindLink bool            `json:"rebindLink"`
	Exit       int             `json:"exit"`
	Outcome    string          `json:"outcome"`
	Stage      string          `json:"stage"`
	Code       string          `json:"code"`
}

type neutralExpected struct {
	BaseRevision   string `json:"baseRevision"`
	TargetRevision string `json:"targetRevision"`
	Accept         struct {
		Exit   int            `json:"exit"`
		Result map[string]any `json:"resultExceptRuntime"`
	} `json:"accept"`
	Refusals []neutralRefusal `json:"refusals"`
}

func neutralStableRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate stable_neutral_test.go")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "protocol", "cem-1.0", "stable"))
}

// stableCaseFiles checks every file of one public case against the stable
// protocol manifest and returns the case directory.
func stableCaseFiles(t *testing.T, name string) string {
	t.Helper()
	stable := neutralStableRoot(t)
	raw, err := os.ReadFile(filepath.Join(stable, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, f := range manifest.Files {
		if strings.HasPrefix(f.Path, name+"/") {
			b, err := os.ReadFile(filepath.Join(stable, filepath.FromSlash(f.Path)))
			if err != nil || len(b) != f.Bytes || shaHex(b) != f.SHA256 {
				t.Fatalf("manifest entry %s does not match the case file", f.Path)
			}
			listed[f.Path] = true
		}
	}
	dir := filepath.Join(stable, name)
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(stable, p)
		if !listed[filepath.ToSlash(rel)] {
			t.Errorf("case file %s is not in the stable manifest", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func neutralCaseFiles(t *testing.T) string {
	t.Helper()
	return stableCaseFiles(t, "neutral-criterion")
}

// neutralRepository builds the recipe repository with Git plumbing and the
// recipe's fixed identity and returns its directory and commits.
func neutralRepository(t *testing.T, dir string) (repo, base, target string) {
	t.Helper()
	var recipe struct {
		Identity struct{ Name, Email, Date string } `json:"identity"`
		Commits  []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
			// Links are mode-120000 entries whose blob is the link target
			// (the cem-non-regular/1 vector, CAX-V0-024).
			Links map[string]string `json:"links"`
		} `json:"commits"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "repository.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &recipe); err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(s0eTemp(t), "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(stdin string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir = repo
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent-cem01-home", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C",
			"GIT_AUTHOR_NAME=" + recipe.Identity.Name, "GIT_AUTHOR_EMAIL=" + recipe.Identity.Email, "GIT_AUTHOR_DATE=" + recipe.Identity.Date,
			"GIT_COMMITTER_NAME=" + recipe.Identity.Name, "GIT_COMMITTER_EMAIL=" + recipe.Identity.Email, "GIT_COMMITTER_DATE=" + recipe.Identity.Date}
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("", "init", "-q", "--object-format=sha1")
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files)+len(commit.Links))
		for name := range commit.Files {
			names = append(names, name)
		}
		for name := range commit.Links {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			mode, content := "100644", commit.Files[name]
			if link, ok := commit.Links[name]; ok {
				mode, content = "120000", link
			}
			tree.WriteString(mode + " blob " + git(content, "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", git(tree.String(), "mktree"), "-m", commit.Message}
		if target != "" {
			args = append(args, "-p", target)
		}
		target = git("", args...)
		if commit.Name == "base" {
			base = target
		}
	}
	git("", "update-ref", "refs/heads/main", target)
	git("", "symbolic-ref", "HEAD", "refs/heads/main")
	if listing := git("", "ls-tree", "-r", "--name-only", target); strings.Contains(listing, ".taskman") || strings.Contains(listing, ".corvint") {
		t.Fatalf("public case repository carries Corvint state:\n%s", listing)
	}
	return repo, base, target
}

func neutralResultMap(t *testing.T, r stableResult) map[string]any {
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

// TestCEMV1083NeutralCriterionPublicCase is the portable side of the public
// no-Tasks case; the native verifier asserts the same EXPECTED.json.
func TestCEMV1083NeutralCriterionPublicCase(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := neutralCaseFiles(t)
	var want neutralExpected
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".taskman")); !os.IsNotExist(err) {
		t.Fatalf(".taskman lstat = %v, want absent", err)
	}
	mapBytes, err := os.ReadFile(filepath.Join(dir, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, dir, nil)
	if got := neutralResultMap(t, r); exit != want.Accept.Exit || !reflect.DeepEqual(got, want.Accept.Result) {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonical exit=%d result=%s, want the public expected ACCEPT", exit, b)
	}
	checkResultBytes(t, filepath.Join(dir, "results", "accept.json"), r)
	if len(want.Refusals) != 7 {
		t.Fatalf("public refusals = %d, want 7", len(want.Refusals))
	}
	for _, refusal := range want.Refusals {
		t.Run(refusal.Name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(mapBytes, &m); err != nil {
				t.Fatal(err)
			}
			var criterion map[string]any
			if err := json.Unmarshal(refusal.Criterion, &criterion); err != nil {
				t.Fatal(err)
			}
			m["criterionBindings"] = []any{criterion}
			if refusal.RebindLink {
				m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
			}
			mutated, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			r, exit := runStableCanonical(context.Background(), repo, mutated, base, target, dir, nil)
			if exit != refusal.Exit || r.Outcome != refusal.Outcome || r.Stage != refusal.Stage || strOf(r.Code) != refusal.Code {
				t.Fatalf("runStableCanonical exit=%d %s %s %s, want %d %s %s %s", exit, r.Outcome, r.Stage, strOf(r.Code), refusal.Exit, refusal.Outcome, refusal.Stage, refusal.Code)
			}
		})
	}
}

// TestCEMV1083PortablePlainDuplicateEvidence proves two plain criteria on one
// evidence span are a duplicate reference in the portable consumer.
func TestCEMV1083PortablePlainDuplicateEvidence(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(neutralCaseFiles(t), "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	first := m["criterionBindings"].([]any)[0].(map[string]any)
	second := map[string]any{"criterionIndex": 1, "criterionSha256": first["criterionSha256"], "evidenceId": first["evidenceId"]}
	identity, err := json.Marshal(second) // encoding/json sorts map keys: canonical for these ASCII values
	if err != nil {
		t.Fatal(err)
	}
	second["id"] = "criterion:sha256:" + shaHex(identity)
	m["criterionBindings"] = append(m["criterionBindings"].([]any), second)
	mutated, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	decoded, e := decodeStable(mutated)
	if e != nil {
		t.Fatalf("decodeStable error %v", e)
	}
	if e := validateStableReferences(&decoded); e == nil || e.code != "duplicate-reference" {
		t.Fatalf("validateStableReferences error %v, want duplicate-reference", e)
	}
}

// TestCEMV1083PortableTasksFormUnchanged proves every Tasks-bound criterion of
// the frozen S0E maps still decodes as the Tasks form and re-encodes to its
// original eleven keys and values.
func TestCEMV1083PortableTasksFormUnchanged(t *testing.T) {
	packet, err := s0eDefaultPacket()
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, name := range []string{
		"inherited/fixtures/maps/sha1-positive-sealed.json", "inherited/fixtures/maps/sha1-positive-sidecarless.json",
		"sha1-ledger-336-evidence.json", "sha1-ledger-337-evidence.json", "sha1-nonstructural-sidecarless.json",
	} {
		raw, err := os.ReadFile(filepath.Join(packet, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		m, e := decodeStable(raw)
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		var original struct {
			CriterionBindings []map[string]any `json:"criterionBindings"`
		}
		if err := json.Unmarshal(raw, &original); err != nil {
			t.Fatal(err)
		}
		if len(m.CriterionBindings) != len(original.CriterionBindings) {
			t.Fatalf("%s criteria = %d, want %d", name, len(m.CriterionBindings), len(original.CriterionBindings))
		}
		for i, c := range m.CriterionBindings {
			if c.EvidenceID != "" {
				t.Fatalf("%s criterion %d decoded as plain", name, i)
			}
			encoded, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			var again map[string]any
			if err := json.Unmarshal(encoded, &again); err != nil {
				t.Fatal(err)
			}
			if len(again) != 11 || !reflect.DeepEqual(again, original.CriterionBindings[i]) {
				t.Fatalf("%s criterion %d re-encodes as %s", name, i, encoded)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("S0E maps carry no Tasks criterion")
	}
}
