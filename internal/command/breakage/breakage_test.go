package breakage

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/breakagemap"
)

func TestBreakageCLIRejectsMissingAndAmbiguousBindings(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{}, {"--manifest", "/missing", "--api", "repo:path:Name"}, {"--manifest", "/missing", "--api", "repo:path:Name", "--repository", "r=a", "--repository", "r=b"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), ".", args, &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("code=%d out=%s diagnostic=%s", code, &out, &diagnostic)
		}
	}
}

func TestBreakageManifestRejectsIncompleteInvocations(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"manifest"}, {"manifest", "--repository", "cli=."}, {"manifest", "--source", "cli:go.mod"}, {"manifest", "--repository", "cli=.", "--source", "cli:go.mod", "extra"}, {"manifest", "--repository", "bad"}} {
		var out, diagnostic bytes.Buffer
		if code := Run(context.Background(), ".", args, &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("%v: code=%d out=%s diagnostic=%s", args, code, &out, &diagnostic)
		}
	}
}

// The explicit helper exercises the command outside central dispatch and
// permits retained source-bound CLI qualification with real pins.
func TestBreakageCommandHelperProcess(t *testing.T) {
	if os.Getenv("CORVINT_BREAKAGE_HELPER_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(Run(context.Background(), ".", os.Args[i+1:], os.Stdout, os.Stderr))
		}
	}
	os.Exit(2)
}

func TestBreakageCLIComplete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.email", "corvint@example.test")
	git("config", "user.name", "Corvint Test")
	files := map[string]string{"go.mod": "module example.test/cli\n", "api.go": "package cli\nfunc Changed() {}\n", "caller.go": "package cli\nfunc Caller() { Changed() }\n"}
	for p, b := range files {
		if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-qm", "api")
	repo := breakagemap.Repository{ID: "cli", Origin: git("rev-list", "--max-parents=0", "HEAD"), Commit: git("rev-parse", "HEAD"), Tree: git("rev-parse", "HEAD^{tree}")}
	m := breakagemap.Manifest{Schema: "corvint-breakage-manifest/0", Repositories: []breakagemap.Repository{repo}}
	for _, p := range []string{"go.mod", "api.go", "caller.go"} {
		m.Sources = append(m.Sources, breakagemap.Source{Repository: "cli", Path: p, Blob: git("rev-parse", "HEAD:"+p), Start: 1, End: len(strings.Split(files[p], "\n"))})
	}
	raw, _ := json.Marshal(m)
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if e := os.WriteFile(manifest, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var out, diagnostic bytes.Buffer
	if code := Run(context.Background(), root, []string{"--manifest", manifest, "--api", "cli:api.go:Changed", "--repository", "cli=."}, &out, &diagnostic); code != 0 {
		t.Fatalf("code %d: %s", code, &diagnostic)
	}
	var report breakagemap.Report
	if e := json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	if len(report.Edges) != 1 || report.Edges[0].Kind != "syntax-call" || report.Edges[0].From.Path != "caller.go" {
		t.Fatalf("missing caller: %s", &out)
	}
}
