package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// externalRecipeRepository builds one repository from a public case recipe
// (repository.recipe.json or external.recipe.json) with the fixed fixture
// identity and returns its root and last commit.
func externalRecipeRepository(t *testing.T, recipePath string) (root, head string) {
	t.Helper()
	var recipe struct {
		Commits []struct {
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
			// Links are mode-120000 entries (CAX-V0-024).
			Links map[string]string `json:"links"`
		} `json:"commits"`
	}
	data, err := os.ReadFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(fixture, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(stdin string, args ...string) string {
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
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
		if head != "" {
			args = append(args, "-p", head)
		}
		head = git("", args...)
	}
	git("", "update-ref", "refs/heads/main", head)
	git("", "symbolic-ref", "HEAD", "refs/heads/main")
	return root, head
}

// TestCEMV1333ExternalRepositoryOption proves the native verify-stable
// option: a bad name, a missing or duplicate value and a relative or
// unclean PATH refuse at the arguments stage (a relative PATH with the
// absolute-path fix line); an unsupplied repository abstains UNSUPPORTED
// artifact-unavailable with the CEM-V1-336 fix on stderr only; and the
// supplied repository accepts (CEM-V1-333, 334, 336).
func TestCEMV1333ExternalRepositoryOption(t *testing.T) {
	external, err := filepath.Abs("../../../protocol/cem-1.0/stable/external-criterion")
	if err != nil {
		t.Fatal(err)
	}
	head := []string{"cem", "verify-stable", "--repository", "/nonexistent-repository", "--map", filepath.Join(external, "stable.json"), "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", external}
	for _, c := range []struct {
		name  string
		extra []string
		fix   string
	}{
		{"path name", []string{"--external-repository", "../requirements=/r"}, ""},
		{"missing value", []string{"--external-repository"}, ""},
		{"no equals", []string{"--external-repository", "requirements"}, ""},
		{"duplicate name", []string{"--external-repository", "requirements=/r", "--external-repository", "requirements=/s"}, ""},
		{"unclean path", []string{"--external-repository", "requirements=/r/../s"}, ""},
		{"relative path", []string{"--external-repository", "requirements=relative/r"}, "--external-repository"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			exit, handled := RunStableInvocation(context.Background(), append(append([]string{}, head...), c.extra...), &out, &errout)
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !handled {
				t.Fatalf("handled %v, stdout %q: %v", handled, out.String(), err)
			}
			if exit != 2 || result["outcome"] != "UNSUPPORTED" || result["stage"] != "arguments" || result["code"] != "invalid-arguments" {
				t.Fatalf("exit %d result %s", exit, out.String())
			}
			requireFixLine(t, errout.String(), "corvint cem verify-stable: ", c.fix)
		})
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	repo, target := externalRecipeRepository(t, filepath.Join(external, "repository.recipe.json"))
	requirements, _ := externalRecipeRepository(t, filepath.Join(external, "external.recipe.json"))
	if target != stableArgumentTarget {
		t.Fatalf("recipe target %s, want %s", target, stableArgumentTarget)
	}
	run := []string{"cem", "verify-stable", "--repository", repo, "--map", filepath.Join(external, "stable.json"), "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", external}
	var out, errout bytes.Buffer
	exit, _ := RunStableInvocation(context.Background(), run, &out, &errout)
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || exit != 2 || result["outcome"] != "UNSUPPORTED" || result["stage"] != "artifacts" || result["code"] != "artifact-unavailable" {
		t.Fatalf("unsupplied: exit %d result %s: %v", exit, out.String(), err)
	}
	want := "corvint cem verify-stable: artifact-unavailable: external repository requirements not supplied; external criteria stay NOT_OBSERVED; fix: pass --external-repository NAME=/absolute/path for each\n"
	if errout.String() != want || strings.Contains(out.String(), "fix:") {
		t.Fatalf("stderr %q, want %q; stdout %s", errout.String(), want, out.String())
	}
	out.Reset()
	errout.Reset()
	exit, _ = RunStableInvocation(context.Background(), append(run, "--external-repository", "requirements="+requirements), &out, &errout)
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || exit != 0 || result["outcome"] != "ACCEPT" || errout.Len() != 0 {
		t.Fatalf("supplied: exit %d stderr %q result %s: %v", exit, errout.String(), out.String(), err)
	}
}
