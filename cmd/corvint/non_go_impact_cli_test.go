package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NGI-V0-003 NGI-V0-004: the new path frontier is selected explicitly;
// the historical default remains discriminated by the immutable parity corpus.
func TestNonGoImpactCLIProfile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.rb":         "def run(value)\n  send(value)\nend\n",
		"b.js":         "export function run(key) { return actions[key](); }\n",
		"c.ts":         "export function run(key: string) { return actions[key](); }\n",
		"go.mod":       "module example.test/profile\n\ngo 1.27\n",
		"pkg/value.go": "package value\nfunc Value() int { return 1 }\n",
	}
	for name, body := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "config", "user.name", "fixture")
	cemGit(t, root, "config", "user.email", "fixture@example.test")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "baseline")
	invoke := func(name string, profile bool) ([]byte, map[string]any) {
		args := []string{"--root", root, "impact", name, "--limit", "1"}
		if profile {
			args = append(args, "--language-profile", "non-go-syntax-v0")
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, bytes.NewReader(nil), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit%d: %s", args, code, stderr.String())
		}
		var packet map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &packet); err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), stdout.Bytes()...), packet["context"].(map[string]any)
	}
	for _, name := range []string{"a.rb", "b.js", "c.ts"} {
		_, legacy := invoke(name, false)
		if legacy["unknowns"] != nil || legacy["language_profile"] != nil {
			t.Fatalf("historical path extended: %v", legacy)
		}
		first, syntax := invoke(name, true)
		second, _ := invoke(name, true)
		if !bytes.Equal(first, second) {
			t.Fatal("profile repeat bytes changed")
		}
		if syntax["language_profile"] != "non-go-syntax-v0" || len(syntax["unknowns"].([]any)) == 0 {
			t.Fatalf("missing versioned frontier: %v", syntax)
		}
	}
	legacy, _ := invoke("pkg/value.go", false)
	syntax, _ := invoke("pkg/value.go", true)
	if !bytes.Equal(legacy, syntax) {
		t.Fatal("Go receipt bytes changed under path selector")
	}
}

func TestNonGoImpactCLIProfileAdmission(t *testing.T) {
	for _, args := range [][]string{
		{"a.ts", "--language-profile", ""},
		{"a.ts", "--language-profile", "NON-GO-SYNTAX-V0"},
		{"a.ts", "--language-profile"},
		{"a.ts", "--language-profile", "non-go-syntax-v0", "--language-profile", "non-go-syntax-v0"},
		{"--base", strings.Repeat("0", 40), "--language-profile", "non-go-syntax-v0"},
		{"a.go", "--working-tree-untracked", "--language-profile", "non-go-syntax-v0"},
	} {
		if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, args, "darwin"); err == nil {
			t.Fatalf("accepted invalid profile argv: %v", args)
		}
	}
	for _, args := range [][]string{{"a.ts", "--language-profile=non-go-syntax-v0"}, {"--language-profile", "non-go-syntax-v0", "a.ts"}} {
		got, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, args, "darwin")
		if err != nil || got.impactLanguage != "non-go-syntax-v0" {
			t.Fatalf("valid profile refused: %v: %v", args, err)
		}
	}
}
