package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func generateRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(arguments ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-08-15T10:00:00+0000", "GIT_COMMITTER_DATE=2026-08-15T10:00:00+0000")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	write := func(name, content string) {
		t.Helper()
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "t")
	write("go.mod", "module example.test/gen\n\ngo 1.27.0\n")
	write("cache/cache.go", "package cache\n\nfunc Demux() string { return \"demux\" }\n")
	write("cache/cache_test.go", "package cache\n\nfunc TestDemux() {}\n")
	write("logo.png", "\x89PNG\x00\x00")
	run("add", "-A")
	run("commit", "-qm", "base")
	write("cache/cache.go", "package cache\n\nfunc Demux() string { return \"demux\" }\n\nfunc Split(key string) string { return key }\n")
	write("server/server.go", "package server\n\nimport \"example.test/gen/cache\"\n\nfunc Run() string { return cache.Split(\"k\") }\n")
	run("add", "-A")
	run("commit", "-qm", "feat(cache): add Split and wire the server")
	write("README.md", "# docs only\n")
	run("add", "-A")
	run("commit", "-qm", "docs: readme")
	return root
}

func TestGenerateBuildsCoChangeTasksFromLocalHistory(t *testing.T) {
	root := generateRepository(t)
	output := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := generate([]string{"--repo", root, "--name", "example/gen", "--since", "2026-08-01", "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(output, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document generatedManifest
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Population != 1 || len(document.Tasks) != 1 || document.Partition != "unseen" {
		t.Fatalf("manifest = %s", raw)
	}
	item := document.Tasks[0]
	if item.Kind != "change" || item.ChangedFile != "cache/cache.go" || item.Repo != "example/gen" {
		t.Fatalf("task = %+v", item)
	}
	if got := item.Gold["test-file"]; len(got) != 1 || got[0] != "cache/cache_test.go" {
		t.Fatalf("test gold = %v, want the untouched counterpart present in the parent tree", got)
	}
	if document.Counterparts != 1 || item.Source.(map[string]any)["counterpart_gold_paths"] != 1.0 {
		t.Fatalf("counterpart gold must be counted: manifest %d task %v", document.Counterparts, item.Source)
	}
	if got := item.Gold["source-file"]; len(got) != 1 || got[0] != "server/server.go" {
		t.Fatalf("source gold = %v", got)
	}
	if document.NovelGold != 1 || !strings.Contains(item.Text, "Intent:\nfeat(cache): add Split") || !strings.Contains(item.Text, "@@") {
		t.Fatalf("novel=%d text=%q", document.NovelGold, item.Text)
	}
	snapshot := filepath.Join(output, "corpus", "example__gen", item.BaseCommit+".chunks.jsonl")
	file, err := os.Open(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	paths := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			Kind, Path, Text string
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		paths[row.Path] = row.Kind == "file" && row.Text != ""
	}
	if !paths["cache/cache.go"] || !paths["cache/cache_test.go"] || paths["logo.png"] || paths["server/server.go"] {
		t.Fatalf("snapshot rows = %v, want the parent tree's text files only", paths)
	}
	if document.Snapshots[item.BaseCommit] != 3 {
		t.Fatalf("snapshot count = %v", document.Snapshots)
	}
}
