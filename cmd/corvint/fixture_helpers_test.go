package main

// Shared fixtures kept from test files removed with the context-injection cut (decision 0594).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitFixture(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := fixtureGitCommand(arguments...)
	command.Dir = root
	// Pin identity as well as dates after filtering inherited overrides: the
	// fixture commit id is carried by the packet (TCP-V0-150) and pinned by
	// the TCP-V0-024 wire golden.
	command.Env = append(fixtureGitEnvironment(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", arguments, err)
	}
	return strings.TrimSpace(string(output))
}

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return value
}

// runContextCommand lifts the frozen 10-second source bound that --expand and each of its Git
// reads share: these tests verify packets and refusals, not latency, and host load expired a
// read as git-failed (V1-1398, decision 0082).
func runContextCommand(t *testing.T, arguments ...string) (int, []byte, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(liftedSourceDeadline(context.Background()), arguments, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.Bytes(), stderr.String()
}

func writeSummaryFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// evidenceSummaryFiles is the fixture the default-wire golden was captured
// over; the tree it commits is content-addressed, so the golden is stable.
var evidenceSummaryFiles = map[string]string{
	"AGENTS.md":              "# Agents\n\nFollow docs/specs/demux-v0.md before changing cache.\n",
	"docs/specs/demux-v0.md": "# Demux V0\n\n## Requirements\n\n- `DMX-V0-001`: Split keeps empty keys.\n- `DMX-V0-002`: Join is the inverse of Split.\n\n## Non-goals\n\nNone.\n",
	"go.mod":                 "module example.test/ctx\n\ngo 1.27.0\n",
	"cache/demux.go":         "package cache\n\n// Split keeps empty keys.\nfunc Split(key string) string { return key }\n",
	"cache/demux_test.go":    "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
	"cache/join.go":          "package cache\n\nfunc Join(key string) string { return Split(key) }\n",
	"cache/keys.go":          "package cache\n\nfunc EmptyKeys() []string { return []string{Split(\"\")} }\n",
	"cache/latin1.txt":       "caf\xe9 keys\n",
}

func evidenceSummaryRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range evidenceSummaryFiles {
		writeSummaryFixture(t, root, name, content)
	}
	gitFixture(t, root, "init", "-q")
	gitFixture(t, root, "add", "-A")
	gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture")
	return root
}

const evidenceSummaryTask = "does `Split` keep empty keys per DMX-V0-001"

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, arguments := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

// ledgerRepository is a repository the beamfall profile recognises: it carries
// both record ledgers, so a query over it must rank feature records.
func ledgerRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/harness\n\ngo 1.27.0\n",
		"testing/features.yaml": "features:\n" +
			"  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n" +
			"  - id: revoked-device\n    area: auth\n    summary: Expired device session is revoked.\n    features: [session-revocation]\n    applies: [server]\n    status: shipped\n",
		"internal/auth/session.go": "package auth\n\n// feature:session-revocation\nfunc EnforceSessionRevocation() bool { return true }\n",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func refusalCode(t *testing.T, stderr []byte) string {
	t.Helper()
	var refusal map[string]any
	if err := json.Unmarshal(stderr, &refusal); err != nil {
		t.Fatalf("decode refusal: %v: %s", err, stderr)
	}
	code, _ := refusal["code"].(string)
	return code
}

// standaloneContextMember returns the "context" member a standalone verb emits,
// as the exact bytes it wrote.
func standaloneContextMember(t *testing.T, root string, arguments ...string) json.RawMessage {
	t.Helper()
	var stdout, stderr bytes.Buffer
	full := append([]string{"--root", root}, arguments...)
	if code := runContext(context.Background(), full, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("standalone %v exit %d: %s", arguments, code, stderr.String())
	}
	var payload struct {
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode standalone payload: %v: %s", err, stdout.String())
	}
	return payload.Context
}

func queryCLIRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "-q"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"},
	} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
		"internal/parser/token.go": "package parser\n\n// ParseToken validates token parser delimiters.\nfunc ParseToken(input string) string { return input }\n",
	}
	for relative, contents := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "initial authority fixture"}} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	historyPath := filepath.Join(root, "docs", "history.md")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for index, fixture := range []struct {
		contents string
		message  []byte
	}{
		{"unicode secret dotless i\n", []byte("roadmap prıvate_key\u2003=\u2003correct-horse-battery-staple ticket\n")},
		{"unicode secret dotted I\n", []byte("roadmap prİvate_key=value ticket\n")},
		{"unicode secret long s\n", []byte("roadmap ſecret=value ticket\n")},
		{"unicode secret kelvin\n", []byte("roadmap toKen=value ticket\n")},
		{"malformed subject\n", []byte{'r', 'o', 'a', 'd', 'm', 'a', 'p', ' ', 0xff, 0xff, ' ', 't', 'i', 'c', 'k', 'e', 't', '\n'}},
	} {
		if err := os.WriteFile(historyPath, []byte(fixture.contents), 0o644); err != nil {
			t.Fatal(err)
		}
		messagePath := filepath.Join(t.TempDir(), fmt.Sprintf("message-%d", index))
		if err := os.WriteFile(messagePath, fixture.message, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, arguments := range [][]string{{"add", "docs/history.md"}, {"commit", "-q", "-F", messagePath}} {
			command := fixtureGitCommand(arguments...)
			command.Dir = root
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", arguments, err, output)
			}
		}
	}
	return root
}

func gitOutput(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := fixtureGitCommand(append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(bytes.TrimSpace(output))
}

// taskContextRepository is a small committed Go repository for the context and snapshot tests.
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
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func repositoryListing(t *testing.T, root string) string {
	t.Helper()
	command := fixtureGitCommand("status", "--porcelain", "--ignored")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
