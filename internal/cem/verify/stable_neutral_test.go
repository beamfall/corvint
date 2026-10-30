package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// neutralCaseRoot is the public no-Tasks case of CEM-V1-083.
const neutralCaseRoot = "protocol/cem-1.0/stable/neutral-criterion"

type neutralCase struct {
	root   string
	raw    []byte
	expect struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			Exit   int            `json:"exit"`
			Result map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		Refusals []struct {
			Name       string          `json:"name"`
			Criterion  json.RawMessage `json:"criterion"`
			RebindLink bool            `json:"rebindLink"`
			Exit       int             `json:"exit"`
			Outcome    string          `json:"outcome"`
			Stage      string          `json:"stage"`
			Code       string          `json:"code"`
		} `json:"refusals"`
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// loadNeutralCase reads the public case and checks every case file against
// the stable protocol manifest, so the tests and the manifest cannot diverge.
func loadNeutralCase(t *testing.T) *neutralCase {
	t.Helper()
	top := repositoryRoot(t)
	c := &neutralCase{root: filepath.Join(top, neutralCaseRoot)}
	checkStableCaseManifest(t, top, "neutral-criterion/")
	var err error
	if c.raw, err = os.ReadFile(filepath.Join(c.root, "stable.json")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(c.root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &c.expect); err != nil {
		t.Fatal(err)
	}
	return c
}

// checkStableCaseManifest checks every manifest entry under prefix against
// its case file and every case file against the manifest, so a public case
// and the stable protocol manifest cannot diverge.
func checkStableCaseManifest(t *testing.T, top, prefix string) {
	t.Helper()
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			Sha256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
		} `json:"files"`
	}
	data, err := os.ReadFile(filepath.Join(top, "protocol/cem-1.0/stable/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, f := range manifest.Files {
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		listed[f.Path] = true
		b, err := os.ReadFile(filepath.Join(top, "protocol/cem-1.0/stable", f.Path))
		if err != nil || stableDigest(b) != f.Sha256 || len(b) != f.Bytes {
			t.Fatalf("manifest entry %s does not match the case file", f.Path)
		}
	}
	err = filepath.WalkDir(filepath.Join(top, "protocol/cem-1.0/stable", prefix), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(top, "protocol/cem-1.0/stable"), p)
		if !listed[filepath.ToSlash(rel)] {
			t.Errorf("case file %s is not in the stable manifest", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// materialize builds the recipe repository with Git plumbing and the fixed
// identity of stableRepositoryGit, and returns it with the target commit.
func (c *neutralCase) materialize(t *testing.T) (repo, base, target string) {
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
	data, err := os.ReadFile(filepath.Join(c.root, "repository.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	if recipe.Identity.Name != "Fixture" || recipe.Identity.Email != "fixture@invalid" || recipe.Identity.Date != "2026-10-01T00:00:00Z" {
		t.Fatalf("recipe identity %+v differs from the stableRepositoryGit identity", recipe.Identity)
	}
	fixture, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(fixture, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	stableRepositoryGit(t, repo, "", "init", "-q", "--object-format=sha1")
	parent := ""
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
			tree.WriteString(mode + " blob " + stableRepositoryGit(t, repo, content, "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", stableRepositoryGit(t, repo, tree.String(), "mktree"), "-m", commit.Message}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		parent = stableRepositoryGit(t, repo, "", args...)
		if commit.Name == "base" {
			base = parent
		}
	}
	stableRepositoryGit(t, repo, "", "update-ref", "refs/heads/main", parent)
	stableRepositoryGit(t, repo, "", "symbolic-ref", "HEAD", "refs/heads/main")
	if base != c.expect.BaseRevision || parent != c.expect.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, parent, c.expect.BaseRevision, c.expect.TargetRevision)
	}
	return repo, base, parent
}

func neutralResult(t *testing.T, result StableResult) map[string]any {
	t.Helper()
	b, err := json.Marshal(result)
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

// TestCEMV1083NeutralCriterionPublicCase proves a plain criterion bound to a
// Git blob span verifies on a repository with no .taskman/ and no Tasks
// fields, with the exact public expected result, and that each public
// refusal mutation fails at its recorded stage and code (CEM-V1-083).
func TestCEMV1083NeutralCriterionPublicCase(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadNeutralCase(t)
	repo, base, target := c.materialize(t)
	if listing := stableRepositoryGit(t, repo, "", "ls-tree", "-r", "--name-only", target); strings.Contains(listing, ".taskman") || strings.Contains(listing, ".corvint") {
		t.Fatalf("public case repository carries Corvint state:\n%s", listing)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".taskman")); !os.IsNotExist(err) {
		t.Fatalf(".taskman lstat = %v, want absent", err)
	}
	if bytes.Contains(c.raw, []byte("ticketId")) {
		t.Fatal("public plain map carries a Tasks field")
	}
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root}
	result, exit := Stable(context.Background(), c.raw, options)
	if got := neutralResult(t, result); exit != c.expect.Accept.Exit || !reflect.DeepEqual(got, c.expect.Accept.Result) {
		t.Fatalf("Stable exit=%d result=%s, want the public expected ACCEPT", exit, stableResultDebug(result))
	}
	checkResultBytes(t, filepath.Join(c.root, "results", "accept.json"), result)
	if len(c.expect.Refusals) != 7 {
		t.Fatalf("public refusals = %d, want 7", len(c.expect.Refusals))
	}
	for _, r := range c.expect.Refusals {
		t.Run(r.Name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(c.raw, &m); err != nil {
				t.Fatal(err)
			}
			var criterion map[string]any
			if err := json.Unmarshal(r.Criterion, &criterion); err != nil {
				t.Fatal(err)
			}
			m["criterionBindings"] = []any{criterion}
			if r.RebindLink {
				m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
			}
			raw, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			result, exit := Stable(context.Background(), raw, options)
			if exit != r.Exit || result.Outcome != r.Outcome || result.Stage != r.Stage || result.Code == nil || *result.Code != r.Code {
				t.Fatalf("Stable exit=%d result=%s, want %d %s %s %s", exit, stableResultDebug(result), r.Exit, r.Outcome, r.Stage, r.Code)
			}
		})
	}
}

// TestCEMV1083TasksFormUnchanged proves every Tasks-bound criterion of the
// frozen S0E maps still decodes as the Tasks form and re-encodes to exactly
// its original eleven keys and values (CEM-V1-083: Tasks form unchanged).
func TestCEMV1083TasksFormUnchanged(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	seen := 0
	for digest, raw := range packet.maps {
		d, err := wire.ParseStable(raw)
		if err != nil {
			t.Fatalf("S0E map %s: %v", digest, err)
		}
		var original struct {
			CriterionBindings []map[string]any `json:"criterionBindings"`
		}
		if err := json.Unmarshal(raw, &original); err != nil {
			t.Fatal(err)
		}
		if len(d.Criteria) != len(original.CriterionBindings) {
			t.Fatalf("S0E map %s criteria = %d, want %d", digest, len(d.Criteria), len(original.CriterionBindings))
		}
		for i, criterion := range d.Criteria {
			if criterion.Plain() {
				t.Fatalf("S0E map %s criterion %d decoded as plain", digest, i)
			}
			encoded, err := json.Marshal(criterion)
			if err != nil {
				t.Fatal(err)
			}
			var again map[string]any
			if err := json.Unmarshal(encoded, &again); err != nil {
				t.Fatal(err)
			}
			if len(again) != 11 || !reflect.DeepEqual(again, original.CriterionBindings[i]) {
				t.Fatalf("S0E map %s criterion %d re-encodes as %s", digest, i, encoded)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("S0E maps carry no Tasks criterion")
	}
}
