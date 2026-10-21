package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// externalCaseRoot is the public external criterion case (CEM-V1-330..336).
const externalCaseRoot = "protocol/cem-1.0/stable/external-criterion"

type externalCaseCriterion struct {
	Name       string          `json:"name"`
	Criterion  json.RawMessage `json:"criterion"`
	RebindLink bool            `json:"rebindLink"`
	Supplied   bool            `json:"supplied"`
	Exit       int             `json:"exit"`
	Outcome    string          `json:"outcome"`
	Stage      string          `json:"stage"`
	Code       *string         `json:"code"`
	IssueCodes []string        `json:"issueCodes"`
}

type externalCase struct {
	root   string
	raw    []byte
	expect struct {
		BaseRevision       string `json:"baseRevision"`
		TargetRevision     string `json:"targetRevision"`
		ExternalRepository string `json:"externalRepository"`
		ExternalBlobOid    string `json:"externalBlobOid"`
		Accept             struct {
			Exit   int            `json:"exit"`
			Result map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		Cases []externalCaseCriterion `json:"cases"`
	}
}

func loadExternalCase(t *testing.T) *externalCase {
	t.Helper()
	top := repositoryRoot(t)
	c := &externalCase{root: filepath.Join(top, externalCaseRoot)}
	checkStableCaseManifest(t, top, "external-criterion/")
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

// materialize builds the service repository from the shared recipe and the
// external requirements repository from external.recipe.json.
func (c *externalCase) materialize(t *testing.T) (repo, base, target, external string) {
	t.Helper()
	primary := &neutralCase{root: c.root}
	primary.expect.BaseRevision, primary.expect.TargetRevision = c.expect.BaseRevision, c.expect.TargetRevision
	repo, base, target = primary.materialize(t)
	var recipe struct {
		Commits []struct {
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	data, err := os.ReadFile(filepath.Join(c.root, "external.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recipe); err != nil || len(recipe.Commits) != 1 || len(recipe.Commits[0].Files) != 1 {
		t.Fatalf("external recipe: %v", err)
	}
	fixture, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	external = filepath.Join(fixture, "requirements")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	stableRepositoryGit(t, external, "", "init", "-q", "--object-format=sha1")
	var tree strings.Builder
	for name, content := range recipe.Commits[0].Files {
		oid := stableRepositoryGit(t, external, content, "hash-object", "-w", "--stdin")
		if oid != c.expect.ExternalBlobOid {
			t.Fatalf("external blob %s, want %s", oid, c.expect.ExternalBlobOid)
		}
		tree.WriteString("100644 blob " + oid + "\t" + name + "\n")
	}
	head := stableRepositoryGit(t, external, "", "commit-tree", stableRepositoryGit(t, external, tree.String(), "mktree"), "-m", recipe.Commits[0].Message)
	stableRepositoryGit(t, external, "", "update-ref", "refs/heads/main", head)
	stableRepositoryGit(t, external, "", "symbolic-ref", "HEAD", "refs/heads/main")
	return repo, base, target, external
}

// mutate replaces the only criterion binding, and optionally the link's
// criterion ID, as each public case records.
func (c *externalCase) mutate(t *testing.T, x externalCaseCriterion) []byte {
	t.Helper()
	if string(x.Criterion) == "null" {
		return c.raw
	}
	var m map[string]any
	if err := json.Unmarshal(c.raw, &m); err != nil {
		t.Fatal(err)
	}
	var criterion map[string]any
	if err := json.Unmarshal(x.Criterion, &criterion); err != nil {
		t.Fatal(err)
	}
	m["criterionBindings"] = []any{criterion}
	if x.RebindLink {
		m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1334ExternalCriterionPublicCase proves an external criterion is
// observed only from its supplied repository: supplied and matching it
// accepts with the exact public result; unsupplied or unreadable it abstains
// UNSUPPORTED artifact-unavailable and never accepts; observed different
// bytes reject; wire and identity faults refuse at their recorded stage
// (CEM-V1-330..335).
func TestCEMV1334ExternalCriterionPublicCase(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadExternalCase(t)
	repo, base, target, external := c.materialize(t)
	supplied := map[string]string{c.expect.ExternalRepository: external}
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root, ExternalRepositories: supplied}
	result, exit := Stable(context.Background(), c.raw, options)
	if got := neutralResult(t, result); exit != c.expect.Accept.Exit || !reflect.DeepEqual(got, c.expect.Accept.Result) {
		t.Fatalf("Stable exit=%d result=%s, want the public expected ACCEPT", exit, stableResultDebug(result))
	}
	if len(c.expect.Cases) != 9 {
		t.Fatalf("public cases = %d, want 9", len(c.expect.Cases))
	}
	for _, x := range c.expect.Cases {
		t.Run(x.Name, func(t *testing.T) {
			o := options
			if !x.Supplied {
				o.ExternalRepositories = nil
			}
			result, exit := Stable(context.Background(), c.mutate(t, x), o)
			code := ""
			if result.Code != nil {
				code = *result.Code
			}
			want := ""
			if x.Code != nil {
				want = *x.Code
			}
			if exit != x.Exit || result.Outcome != x.Outcome || result.Stage != x.Stage || code != want || !reflect.DeepEqual(append([]string{}, result.IssueCodes...), append([]string{}, x.IssueCodes...)) {
				t.Fatalf("Stable exit=%d result=%s, want %d %s %s %q %v", exit, stableResultDebug(result), x.Exit, x.Outcome, x.Stage, want, x.IssueCodes)
			}
			if result.Accept != nil && *result.Accept {
				t.Fatal("an unobserved or mismatched external criterion accepted")
			}
		})
	}
	// A supplied root that is not a repository is never observed either.
	o := options
	o.ExternalRepositories = map[string]string{c.expect.ExternalRepository: t.TempDir()}
	if result, exit := Stable(context.Background(), c.raw, o); exit != 2 || result.Outcome != "UNSUPPORTED" || result.Code == nil || *result.Code != "artifact-unavailable" {
		t.Fatalf("non-repository root: exit=%d result=%s", exit, stableResultDebug(result))
	}
}

// TestCEMV1336ExternalFixLine proves the stderr fix line names exactly the
// unsupplied external repositories and is empty once all are supplied.
func TestCEMV1336ExternalFixLine(t *testing.T) {
	t.Parallel()
	c := loadExternalCase(t)
	fix := StableExternalFix(c.raw, nil)
	if !strings.Contains(fix, "external repository requirements not supplied") || !strings.Contains(fix, "--external-repository NAME=/absolute/path") {
		t.Fatalf("fix line %q", fix)
	}
	if fix := StableExternalFix(c.raw, map[string]string{"requirements": "/r"}); fix != "" {
		t.Fatalf("fix line with every repository supplied = %q", fix)
	}
	for _, value := range []string{"requirements", "=/r", "../x=/r", "requirements="} {
		if _, _, ok := ParseExternalRepository(value); ok {
			t.Fatalf("ParseExternalRepository(%q) accepted", value)
		}
	}
	if name, root, ok := ParseExternalRepository("requirements=/a=b"); !ok || name != "requirements" || root != "/a=b" {
		t.Fatalf("ParseExternalRepository = %q %q %v", name, root, ok)
	}
}
