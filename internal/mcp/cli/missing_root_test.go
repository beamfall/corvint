package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// startupSession runs one in-process session like multiRootSession, but
// returns the exit code and stderr instead of requiring a silent start.
func startupSession(t *testing.T, arguments []string, requests ...map[string]any) (int, string, map[string]map[string]any) {
	t.Helper()
	var input, stdout, stderr bytes.Buffer
	for _, request := range requests {
		request["jsonrpc"] = "2.0"
		request["params"].(map[string]any)["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(append(raw, '\n'))
	}
	exit := run(context.Background(), arguments, &input, &stdout, &stderr)
	responses := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		responses[fmt.Sprint(response["id"])] = response
	}
	return exit, stderr.String(), responses
}

// missingSpareRoot is a --root like a deleted Claude desktop spare: neither it
// nor its parent exists.
func missingSpareRoot(t *testing.T) string {
	return filepath.Join(filepath.Clean(t.TempDir()), "spares", "spare-0001")
}

// MCPV0-115 (V1-3497): a --root that does not exist serves the Git worktree
// enclosing the launch directory, from a subdirectory or a linked worktree, and
// says so once on stderr and on every tool result; the first block and
// structuredContent are those of a direct --root of that worktree.
func TestMCPV0115MissingRootServesLaunchWorktree(t *testing.T) {
	skipUnqualifiedStatus(t)
	main := makeHostileRepository(t, "core")
	linked := filepath.Join(filepath.Clean(t.TempDir()), "lane")
	if output, err := exec.Command("git", "-C", main, "worktree", "add", "-q", "-b", "lane", linked).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, output)
	}
	linked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", linked, "commit", "-q", "--allow-empty", "-m", "lane").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	if err := os.Mkdir(filepath.Join(main, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := missingSpareRoot(t)
	const wantStderr = "corvint-mcp: --root does not exist; serving the Git worktree enclosing the launch directory instead\n"

	for _, tc := range []struct{ name, launch, worktree string }{
		{"subdirectory", filepath.Join(main, "sub"), main},
		{"linked worktree", linked, linked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.worktree)
			direct := multiRootSession(t, []string{"--root", tc.worktree}, toolCall(1, "corvint.status", map[string]any{}))["1"]
			t.Chdir(tc.launch)
			exit, stderr, responses := startupSession(t, []string{"--root", missing}, toolCall(1, "corvint.status", map[string]any{}))
			if exit != 0 || stderr != wantStderr {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if got, want := statusRevision(t, responses["1"]), headRevision(t, tc.worktree); got != want {
				t.Fatalf("served revision %s, want the launch worktree's %s", got, want)
			}
			result, baseline := responses["1"]["result"].(map[string]any), direct["result"].(map[string]any)
			content := result["content"].([]any)
			if len(content) != 2 || !reflect.DeepEqual(content[0], baseline["content"].([]any)[0]) ||
				!reflect.DeepEqual(result["structuredContent"], baseline["structuredContent"]) {
				t.Fatalf("substituted result=%#v direct=%#v", result, baseline)
			}
			notice := content[1].(map[string]any)
			var object map[string]any
			if notice["type"] != "text" || json.Unmarshal([]byte(notice["text"].(string)), &object) != nil ||
				object["schema"] != worktreeNoticeSchema || object["code"] != "missing-root-launch-worktree" || object["mutates"] != false {
				t.Fatalf("notice=%#v", notice)
			}
			for _, path := range []string{missing, main, linked} {
				if strings.Contains(notice["text"].(string), path) || strings.Contains(stderr, path) {
					t.Fatal("substitution notice names a path")
				}
			}
		})
	}
}

// MCPV0-115 with MCPV0-950: with no launch worktree to stand in, a missing --root
// keeps its MCPV0-950 outcome, served unavailable with a reason that names no path.
func TestMCPV0115MissingRootOutsideWorktreeServesUnavailable(t *testing.T) {
	outside := filepath.Clean(t.TempDir())
	if _, inside := enclosingWorktree(outside); inside {
		t.Skip("the temporary directory is inside a Git worktree")
	}
	t.Chdir(outside)
	exit, stderr, responses := startupSession(t, []string{"--root", missingSpareRoot(t)}, toolCall(1, "corvint.status", map[string]any{}))
	if exit != 0 || len(responses) == 0 ||
		stderr != "corvint-mcp: repository unavailable: --root does not exist; serving tools/list, and every repository tool call answers invalid-root\n" {
		t.Fatalf("exit=%d stderr=%q responses=%#v", exit, stderr, responses)
	}
	if _, substituted := missingRootFallback([]rootDeclaration{{root: missingSpareRoot(t)}}, false, false,
		func() (string, error) { return "", errors.New("getwd") }); substituted {
		t.Fatal("unobservable launch directory substituted a root")
	}
}

// MCPV0-115: only one plain, missing --root without --follow-ref or
// --corpus-artifact falls back; every other startup failure is unchanged even
// when launched inside a worktree.
func TestMCPV0115FallbackIsNarrow(t *testing.T) {
	main := makeHostileRepository(t, "core")
	plain := filepath.Clean(t.TempDir())
	file := filepath.Join(plain, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	missing := missingSpareRoot(t)
	t.Chdir(main)
	// The unchanged outcomes are MCPV0-950's: a plain unusable root is served
	// unavailable with its reason, any other refusal names the reason and exits 2.
	for _, tc := range []struct {
		name       string
		arguments  []string
		wantExit   int
		wantStderr string
	}{
		{"existing directory outside Git", []string{"--root", plain}, 0, "corvint-mcp: repository unavailable: --root is not inside a Git worktree; serving tools/list, and every repository tool call answers invalid-root\n"},
		{"path through a regular file", []string{"--root", filepath.Join(file, "repo")}, 0, "corvint-mcp: repository unavailable: --root does not exist; serving tools/list, and every repository tool call answers invalid-root\n"},
		{"aliased missing root", []string{"--root", "core=" + missing}, 2, "corvint-mcp: repository unavailable: --root does not exist\n"},
		{"missing root with --follow-ref", []string{"--root", missing, "--follow-ref", "feature"}, 2, "corvint-mcp: repository unavailable: --root does not exist\n"},
		{"missing root with --corpus-artifact", []string{"--root", missing, "--tool-profile", "test-validity,corpus", "--corpus-artifact", "corpus.json"}, 2, "corvint-mcp: repository unavailable: --root does not exist\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, stderr, responses := startupSession(t, tc.arguments, toolCall(1, "corvint.status", map[string]any{}))
			if exit != tc.wantExit || (exit == 2) != (len(responses) == 0) || stderr != tc.wantStderr {
				t.Fatalf("exit=%d stderr=%q responses=%#v", exit, stderr, responses)
			}
		})
	}
	// A nested directory whose .git is a symlink to another repository's metadata is the
	// candidate, not the enclosing outer repository; it is refused as a direct --root would be.
	t.Run("symlink marker inside an outer repository", func(t *testing.T) {
		other := makeHostileRepository(t, "other")
		inner := filepath.Join(main, "inner")
		if err := os.Mkdir(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(other, ".git"), filepath.Join(inner, ".git")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(inner)
		exit, stderr, _ := startupSession(t, []string{"--root", missing}, toolCall(1, "corvint.status", map[string]any{}))
		if strings.Contains(stderr, "serving the Git worktree enclosing") {
			t.Fatalf("the symlink-marker candidate was served as the launch worktree: exit=%d stderr=%q", exit, stderr)
		}
		direct, directStderr, _ := startupSession(t, []string{"--root", inner}, toolCall(1, "corvint.status", map[string]any{}))
		if direct != exit || directStderr != stderr {
			t.Fatalf("direct --root of the symlink-marker directory: exit=%d stderr=%q", direct, directStderr)
		}
	})
}
