package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// witnessPolicyRange commits a Go module whose base optionally holds the
// declared witness path policy, then the GitHub #1962 range: a Go change with
// Markdown, JSON and YAML changes beside it.
func witnessPolicyRange(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	write := func(files map[string]string) {
		for name, body := range files {
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	git := func(arguments ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "user.email=corvint@example.test", "-c", "user.name=Corvint Test", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	base := map[string]string{
		"go.mod":               "module example.test/cli\n\ngo 1.22\n",
		"a/a.go":               "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go":          "package a\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestA(t *testing.T) { _, _ = os.ReadFile(\"testdata/case.json\"); _ = A() }\n",
		"a/testdata/case.json": "{}\n",
		"docs/guide.md":        "# Guide\n",
		"deploy/app.yaml":      "replicas: 1\n",
	}
	if policy != "" {
		base[".corvint/witness-paths.json"] = policy
	}
	write(base)
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "initial")
	write(map[string]string{
		"a/a.go":               "package a\n\nfunc A() int { return 2 }\n",
		"a/testdata/case.json": "{\"case\": 2}\n",
		"docs/guide.md":        "# Guide, revised\n",
		"deploy/app.yaml":      "replicas: 2\n",
	})
	git("add", ".")
	git("commit", "-qm", "change")
	return root
}

// TestAGWV0045CheckAdmittedPassesOnlyWithAPolicy is the GitHub #1962 repro:
// --check=admitted exits 1 on a range touching .md, .json and .yaml paths
// without a policy, and 0 once the base tree declares them inert or data with
// a reader; the report names each declaring line.
func TestAGWV0045CheckAdmittedPassesOnlyWithAPolicy(t *testing.T) {
	t.Parallel()
	policy := "{\n  \"profile\": \"corvint-witness-paths/0\",\n  \"rules\": [\n" +
		"    {\"glob\": \"docs/**/*.md\", \"class\": \"inert\"},\n" +
		"    {\"glob\": \"**/testdata/*.json\", \"class\": \"data\"},\n" +
		"    {\"glob\": \"deploy/*.yaml\", \"class\": \"data\", \"readers\": [\"a/a.go\"]}\n  ]\n}\n"
	for _, item := range []struct {
		policy string
		exit   int
		want   []string
	}{
		{"", 1, []string{"NOT_RUN  CHANGE cov=NONE    changed-path   docs/guide.md  [LANGUAGE_COVERAGE_NONE]"}},
		{policy, 0, []string{
			"PATH POLICY\n  status=BOUND path=.corvint/witness-paths.json rules=3",
			"EXEMPT   CHANGE cov=NONE    changed-path   docs/guide.md  [DECLARED_INERT]\n        declared inert by .corvint/witness-paths.json:4 glob=docs/**/*.md",
			"declared data by .corvint/witness-paths.json:5 glob=**/testdata/*.json reader=a/a_test.go(found)",
			"declared data by .corvint/witness-paths.json:6 glob=deploy/*.yaml reader=a/a.go(declared)",
		}},
	} {
		root := witnessPolicyRange(t, item.policy)
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if code := runWitness(t.Context(), root, []string{"--base", "HEAD~1", "--check=admitted"}, stdout, stderr); code != item.exit {
			t.Fatalf("policy %t: exit = %d, want %d; stderr %s\n%s", item.policy != "", code, item.exit, stderr.String(), stdout.String())
		}
		for _, want := range item.want {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("policy %t: report lacks %q:\n%s", item.policy != "", want, stdout.String())
			}
		}
	}
}
