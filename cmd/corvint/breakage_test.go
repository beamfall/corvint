package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Handler-level breakage tests live in internal/command/breakage. These
// remain here because they exercise root --root resolution, dispatch and help.

func TestBreakageManifestCLIPinsNamedSources(t *testing.T) {
	root := cliRepository(t)
	files := map[string]string{"go.mod": "module example.test/cli\n", "api.go": "package cli\nfunc Changed() {}\n", "caller.go": "package cli\nfunc Caller() { Changed() }\n"}
	for p, b := range files {
		writeFixtureFile(t, root, p, b)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "api")
	var out, diagnostic bytes.Buffer
	args := []string{"--root", root, "breakage", "manifest", "--repository", "cli=.", "--source", "cli:go.mod", "--source", "cli:api.go", "--source", "cli:caller.go"}
	if code := runContext(context.Background(), args, strings.NewReader(""), &out, &diagnostic); code != 0 {
		t.Fatalf("code %d: %s", code, &diagnostic)
	}
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if e := os.WriteFile(manifest, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var report bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "breakage", "--manifest", manifest, "--api", "cli:api.go:Changed", "--repository", "cli=."}, strings.NewReader(""), &report, &diagnostic); code != 0 || !strings.Contains(report.String(), "syntax-call") {
		t.Fatalf("built manifest did not compile: %d %s %s", code, &report, &diagnostic)
	}
	for _, bad := range [][]string{
		{"breakage", "manifest"},
		{"breakage", "manifest", "--repository", "cli=."},
		{"breakage", "manifest", "--repository", "cli=.", "--source", "other:go.mod"},
		{"breakage", "manifest", "--repository", "cli=.", "--source", "cli:absent.go"},
		{"breakage", "manifest", "--repository", "cli=.", "--source", "cli:go.mod", "extra"},
	} {
		var out, diagnostic bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root}, bad...), strings.NewReader(""), &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("%v: code=%d out=%s diagnostic=%s", bad, code, &out, &diagnostic)
		}
	}
}

func TestWorkflowHelpKeepsBooleanFlagsScoped(t *testing.T) {
	for _, topic := range []string{"prove", "query"} {
		_, requested, _ := parseHelpInvocation([]string{topic, "--attack-tests", "--help"})
		if requested != (topic == "prove") {
			t.Fatalf("%s: experimental attack flag changed unrelated help scan", topic)
		}
	}
	for _, args := range [][]string{{"breakage", "--help"}, {"help", "breakage"}, {"--root", "/missing", "breakage", "--help"}} {
		code, out, diagnostic := runCLI(t, args...)
		if code != 0 || diagnostic != "" || !strings.Contains(out, "BKM-V0") || !strings.Contains(out, "INCOMPLETE") {
			t.Fatalf("%v: %d %s %s", args, code, out, diagnostic)
		}
	}
}
