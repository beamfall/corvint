package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTaskContextInvocation(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	options, isContext, err := parseTaskContextInvocation([]string{"--root", root, "context", "--task", "find the demux", "--subject", "cache/demux.go", "--limit", "7"})
	if err != nil || !isContext {
		t.Fatalf("parse: isContext=%v err=%v", isContext, err)
	}
	if options.task != "find the demux" || options.subject != "cache/demux.go" || options.limit != 7 {
		t.Fatalf("options = %+v", options)
	}
	if _, isContext, err := parseTaskContextInvocation([]string{"--root", root, "context", "--subject", "x.go"}); !isContext || err == nil {
		t.Fatalf("missing --task: isContext=%v err=%v", isContext, err)
	}
	if _, isContext, err := parseTaskContextInvocation([]string{"--root", root, "context", "--task", "t", "--mutate"}); !isContext || err == nil {
		t.Fatalf("unknown flag: isContext=%v err=%v", isContext, err)
	}
	if _, isContext, _ := parseTaskContextInvocation([]string{"query", "--task", "t"}); isContext {
		t.Fatal("query parsed as context")
	}
}

func taskContextRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                "module example.test/ctx\n\ngo 1.27.0\n",
		"cache/demux.go":        "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go":   "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"testing/features.yaml": "features: []\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func TestRunTaskContextIsReadOnlyAndKeepsTheSubjectOut(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	before := repositoryListing(t, root)
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "context", "--task", "does `Split` keep empty keys", "--subject", "cache/demux.go"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var packet map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet["state"] != "READY" || packet["subject"].(map[string]any)["path"] != "cache/demux.go" {
		t.Fatalf("packet = %s", stdout.String())
	}
	for _, item := range packet["results"].([]any) {
		if item.(map[string]any)["id"] == "cache/demux.go" {
			t.Fatalf("subject listed as a result: %s", stdout.String())
		}
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatalf("context wrote to the repository:\n%s\n%s", before, after)
	}
}

func repositoryListing(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "status", "--porcelain", "--ignored")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
