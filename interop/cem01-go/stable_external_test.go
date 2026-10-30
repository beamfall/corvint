package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type externalExpected struct {
	BaseRevision       string `json:"baseRevision"`
	TargetRevision     string `json:"targetRevision"`
	ExternalRepository string `json:"externalRepository"`
	Accept             struct {
		Exit   int            `json:"exit"`
		Result map[string]any `json:"resultExceptRuntime"`
	} `json:"accept"`
	Cases []struct {
		Name       string          `json:"name"`
		Criterion  json.RawMessage `json:"criterion"`
		RebindLink bool            `json:"rebindLink"`
		Supplied   bool            `json:"supplied"`
		Exit       int             `json:"exit"`
		Outcome    string          `json:"outcome"`
		Stage      string          `json:"stage"`
		Code       *string         `json:"code"`
		IssueCodes []string        `json:"issueCodes"`
	} `json:"cases"`
}

// externalCase loads the public external criterion case and builds both of
// its repositories: the service repository from repository.recipe.json and
// the requirements repository from external.recipe.json.
func externalCase(t *testing.T) (dir string, want externalExpected, raw []byte, repo, base, target, requirements string) {
	t.Helper()
	dir = stableCaseFiles(t, "external-criterion")
	data, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if raw, err = os.ReadFile(filepath.Join(dir, "stable.json")); err != nil {
		t.Fatal(err)
	}
	repo, base, target = neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	recipe, err := os.ReadFile(filepath.Join(dir, "external.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	staged := s0eTemp(t)
	if err := os.WriteFile(filepath.Join(staged, "repository.recipe.json"), recipe, 0o644); err != nil {
		t.Fatal(err)
	}
	requirements, _, _ = neutralRepository(t, staged)
	return dir, want, raw, repo, base, target, requirements
}

// TestCEMV1334ExternalCriterionPublicCasePortable is the portable side of the
// public external criterion case: the same exact ACCEPT when the repository
// is supplied and the same stage, code and issue codes for every recorded
// case, so an unsupplied repository is never PASS (CEM-V1-330..335).
func TestCEMV1334ExternalCriterionPublicCasePortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, want, raw, repo, base, target, requirements := externalCase(t)
	supplied := map[string]string{want.ExternalRepository: requirements}
	r, exit := runStableCanonicalExternal(context.Background(), repo, raw, base, target, dir, nil, nil, supplied)
	if got := neutralResultMap(t, r); exit != want.Accept.Exit || !reflect.DeepEqual(got, want.Accept.Result) {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonicalExternal exit=%d result=%s, want the public expected ACCEPT", exit, b)
	}
	if len(want.Cases) != 9 {
		t.Fatalf("public cases = %d, want 9", len(want.Cases))
	}
	for _, c := range want.Cases {
		t.Run(c.Name, func(t *testing.T) {
			mutated := raw
			if string(c.Criterion) != "null" {
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatal(err)
				}
				var criterion map[string]any
				if err := json.Unmarshal(c.Criterion, &criterion); err != nil {
					t.Fatal(err)
				}
				m["criterionBindings"] = []any{criterion}
				if c.RebindLink {
					m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
				}
				var err error
				if mutated, err = json.Marshal(m); err != nil {
					t.Fatal(err)
				}
			}
			roots := supplied
			if !c.Supplied {
				roots = nil
			}
			r, exit := runStableCanonicalExternal(context.Background(), repo, mutated, base, target, dir, nil, nil, roots)
			issues := append([]string{}, r.IssueCodes...)
			if exit != c.Exit || r.Outcome != c.Outcome || r.Stage != c.Stage || strOf(r.Code) != strOf(c.Code) || !reflect.DeepEqual(issues, append([]string{}, c.IssueCodes...)) {
				t.Fatalf("runStableCanonicalExternal exit=%d %s %s %s %v, want %d %s %s %s %v", exit, r.Outcome, r.Stage, strOf(r.Code), r.IssueCodes, c.Exit, c.Outcome, c.Stage, strOf(c.Code), c.IssueCodes)
			}
			if r.Accept != nil && *r.Accept {
				t.Fatal("an unobserved or mismatched external criterion accepted")
			}
		})
	}
	// verify-stable's own envelope takes the same external step.
	if r, exit := runStableExternal(context.Background(), repo, raw, base, target, dir, nil, nil); exit != 2 || r.Outcome != "UNSUPPORTED" || r.Stage != "artifacts" || strOf(r.Code) != "artifact-unavailable" {
		t.Fatalf("runStableExternal unsupplied exit=%d %s %s %s", exit, r.Outcome, r.Stage, strOf(r.Code))
	}
	if r, exit := runStableExternal(context.Background(), repo, raw, base, target, dir, nil, supplied); exit != 0 || r.Outcome != "ACCEPT" {
		t.Fatalf("runStableExternal supplied exit=%d %s %s %s", exit, r.Outcome, r.Stage, strOf(r.Code))
	}
}

// TestCEMV1333ExternalRepositoryOptionPortable proves both portable CLIs
// refuse a bad --external-repository value at the arguments stage, and that
// verify-stable-canonical abstains with the CEM-V1-336 fix when the
// repository is not supplied and accepts when it is (CEM-V1-333, 336).
func TestCEMV1333ExternalRepositoryOptionPortable(t *testing.T) {
	dir := stableCaseFiles(t, "external-criterion")
	head := []string{"--repository", "/nonexistent-repository", "--map", filepath.Join(dir, "stable.json"), "--expected-base", "467cbb73c1102cd7df0299cd74a6bef96ee3a9a4", "--target", "1f181cd0843318b72bb6679cd19878c46eb95ada", "--artifacts", dir}
	for _, bad := range [][]string{
		{"--external-repository", "../requirements=/r"},
		{"--external-repository"},
		{"--external-repository", "requirements"},
		{"--external-repository", "requirements=/r", "--external-repository", "requirements=/s"},
		{"--external-repository", "requirements=/r/../s"},
		{"--external-repository", "requirements=relative/r"},
	} {
		argv := append(append([]string{}, head...), bad...)
		for name, execute := range map[string]func(context.Context, []string) (stableResult, int, []byte, map[string]string){"verify-stable": executeStableCLIExternal, "verify-stable-canonical": executeStableCanonicalCLIExternal} {
			if r, exit, _, _ := execute(context.Background(), argv); exit != 2 || r.Outcome != "UNSUPPORTED" || r.Stage != "arguments" || strOf(r.Code) != "invalid-arguments" {
				t.Fatalf("%s %v: exit=%d %s %s %s", name, bad, exit, r.Outcome, r.Stage, strOf(r.Code))
			}
		}
	}
	code := "invalid-arguments"
	if fix := stableRelativePathFix(stableResult{Code: &code}, append(append([]string{}, head...), "--external-repository", "requirements=relative/r")); fix == "" {
		t.Fatal("a relative --external-repository PATH printed no fix line")
	}
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	_, _, _, repo, _, _, requirements := externalCase(t)
	head[1] = repo
	r, exit, raw, externals := executeStableCanonicalCLIExternal(context.Background(), head)
	want := "artifact-unavailable: external repository requirements not supplied; external criteria stay NOT_OBSERVED; fix: pass --external-repository NAME=/absolute/path for each"
	if exit != 2 || r.Outcome != "UNSUPPORTED" || r.Stage != "artifacts" || strOf(r.Code) != "artifact-unavailable" || stableExternalFix(r, raw, externals) != want {
		t.Fatalf("unsupplied: exit=%d %s %s %s fix %q", exit, r.Outcome, r.Stage, strOf(r.Code), stableExternalFix(r, raw, externals))
	}
	r, exit, raw, externals = executeStableCanonicalCLIExternal(context.Background(), append(head, "--external-repository", "requirements="+requirements))
	if exit != 0 || r.Outcome != "ACCEPT" || stableExternalFix(r, raw, externals) != "" {
		t.Fatalf("supplied: exit=%d %s %s %s", exit, r.Outcome, r.Stage, strOf(r.Code))
	}
}

// TestCEMV1335ExternalReadsChargePrimaryBudgets proves both portable external
// readers charge the verification-wide Git-operation and distinct-blob
// budgets: a primary budget already at its limit abstains instead of reading
// the external blob for free, and a fresh budget carries the read back.
func TestCEMV1335ExternalReadsChargePrimaryBudgets(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	_, want, raw, _, _, _, requirements := externalCase(t)
	m, e := decodeStable(raw)
	if e != nil {
		t.Fatal(e.code)
	}
	externals := map[string]string{want.ExternalRepository: requirements}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	canonical := func(calls int, bytes int64) (*stableSession, *cemError) {
		s := &stableSession{ctx: context.Background(), exe: exe, git: git, calls: calls, charged: map[string]bool{}, bytes: bytes}
		return s, checkStableExternalCanonical(context.Background(), s, m.CriterionBindings, externals)
	}
	plain := func(ops int, bytes int64) (*verifier, *cemError) {
		v := &verifier{ctx: context.Background(), gitOps: ops, blobBytes: bytes, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
		return v, checkStableExternal(context.Background(), v, m.CriterionBindings, externals)
	}
	for name, e := range map[string]*cemError{
		"canonical calls":   func() *cemError { _, e := canonical(stableCallLimit, 0); return e }(),
		"canonical bytes":   func() *cemError { _, e := canonical(0, stableBlobAggregate); return e }(),
		"noncanonical ops":  func() *cemError { _, e := plain(maxGitOps, 0); return e }(),
		"noncanonical blob": func() *cemError { _, e := plain(0, maxTotalBlobs); return e }(),
	} {
		if e == nil || e.code != "artifact-unavailable" {
			t.Fatalf("%s: exhausted primary budget = %v, want artifact-unavailable", name, e)
		}
	}
	if s, e := canonical(0, 0); e != nil || s.calls == 0 || s.bytes == 0 {
		t.Fatalf("canonical fresh budget: e=%v calls=%d bytes=%d", e, s.calls, s.bytes)
	}
	if v, e := plain(0, 0); e != nil || v.gitOps == 0 || v.blobBytes == 0 {
		t.Fatalf("noncanonical fresh budget: e=%v ops=%d bytes=%d", e, v.gitOps, v.blobBytes)
	}
}
