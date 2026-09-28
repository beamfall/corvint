package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestRunTaskContextSubjectlessCounterparts(t *testing.T) {
	t.Run("TCP-V0-004 selected lexical sources retain paired tests", func(t *testing.T) {
		root := taskContextRepository(t)
		files := map[string]string{
			"alpha.go":      "package fixture\n// Record response status when flushing.\nfunc Alpha() {}\n",
			"beta.go":       "package fixture\n// Record response status when flushing.\nfunc Beta() {}\n",
			"alpha_test.go": "package fixture\nfunc TestAlpha() {}\n",
			"beta_test.go":  "package fixture\nfunc TestBeta() {}\n",
		}
		for i := 0; i < 12; i++ {
			files[fmt.Sprintf("unrelated%02d_test.go", i)] = fmt.Sprintf("package fixture\n// response status\nfunc TestUnrelated%d() {}\n", i)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{{"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "counterparts"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		}
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "context", "--task", "record response status when flushing", "--limit", "8"}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, &stderr)
		}
		var packet struct {
			Results []struct {
				Kind, ID string
				Evidence []struct{ Reason, Authority string }
			}
		}
		if err := json.Unmarshal(stdout.Bytes(), &packet); err != nil {
			t.Fatal(err)
		}
		positions := map[string]int{}
		for i, row := range packet.Results {
			positions[row.ID] = i
		}
		for _, name := range []string{"alpha.go", "beta.go", "alpha_test.go", "beta_test.go"} {
			if _, ok := positions[name]; !ok {
				t.Fatalf("missing %s: %s", name, &stdout)
			}
		}
		if len(packet.Results) != 8 || positions["alpha.go"] >= positions["beta.go"] {
			t.Fatalf("limit or lexical strength order changed: %s", &stdout)
		}
		for _, row := range packet.Results {
			if strings.HasPrefix(row.ID, "unrelated") && positions[row.ID] < positions["beta_test.go"] {
				t.Fatalf("unrelated test precedes counterpart: %s", &stdout)
			}
		}
	})
}
