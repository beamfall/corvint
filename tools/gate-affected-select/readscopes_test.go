package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-050 (V1-1399): the fast tier's selector narrows an edit of the
// read-scope declaration against the plan's range.base, as the planner does:
// an added key selects only its package, and an unknown base keeps the
// AFP-V0-023 rule.
func TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050(t *testing.T) {
	if os.Getenv("CORVINT_TEST_READ_SCOPE_PLAN") == "1" {
		os.Args = []string{"gate-affected-select", os.Getenv("CORVINT_TEST_PLAN"), "example.com/fixture", os.Getenv("CORVINT_TEST_ROOT")}
		main()
		return
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	declaration := func(packages string) string {
		return `{"profile":"corvint-test-read-scopes/0","packages":{` + packages + `}}`
	}
	for name, body := range map[string]string{
		"go.mod":                         "module example.com/fixture\n",
		"alpha/alpha.go":                 "package alpha\n",
		"beta/beta.go":                   "package beta\n",
		"gamma/gamma.go":                 "package gamma\n",
		".corvint/test-read-scopes.json": declaration(`"alpha":["docs/"],"beta":["docs/"]`),
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(arguments ...string) string {
		command := exec.Command("git", append([]string{"-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, arguments...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, ".corvint", "test-read-scopes.json"), []byte(declaration(`"alpha":["docs/"],"beta":["docs/"],"gamma":["docs/"]`)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, base, revision string
		want                 []string
	}{
		{"range base", base, strings.Repeat("0", 40), []string{"gamma"}},
		{"worktree-only plan of HEAD", "", base, []string{"gamma"}},
		{"unknown base", "", "", []string{"alpha", "beta", "gamma"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := filepath.Join(t.TempDir(), "plan.json")
			body := `{"plan":{"dirty":[".corvint/test-read-scopes.json"],"unknown":[]},"provider":{"go":{"packages":[],"state":"RUNNABLE"}},"range":{"base":"` + test.base + `","paths":[]},"revision":"` + test.revision + `"}`
			if err := os.WriteFile(plan, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050$")
			command.Env = append(os.Environ(), "CORVINT_TEST_READ_SCOPE_PLAN=1", "CORVINT_TEST_PLAN="+plan, "CORVINT_TEST_ROOT="+root)
			output, err := command.Output()
			if err != nil {
				t.Fatalf("selector failed: %v\n%s", err, output)
			}
			var declared []string
			for _, line := range strings.Split(string(output), "\n") {
				if rest, ok := strings.CutPrefix(line, "declared example.com/fixture/"); ok {
					declared = append(declared, strings.Fields(rest)[0])
				}
			}
			if strings.Join(declared, " ") != strings.Join(test.want, " ") {
				t.Fatalf("declared %v, want %v\n%s", declared, test.want, output)
			}
		})
	}
}
