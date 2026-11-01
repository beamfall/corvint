package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
)

// legacySession runs one server over the 2025-11-25 handshake, sends each
// request after initialize, and returns the exit code, the responses by id and
// stderr.
func legacySession(t *testing.T, arguments []string, requests ...map[string]any) (int, map[float64]map[string]any, string) {
	t.Helper()
	lines := []string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}
	for index, request := range requests {
		request["jsonrpc"], request["id"] = "2.0", index+1
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), append([]string{"--protocol-version", "2025-11-25"}, arguments...), strings.NewReader(strings.Join(lines, "\n")+"\n"), &stdout, &stderr)
	responses := map[float64]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		id, _ := response["id"].(float64)
		responses[id] = response
	}
	return exit, responses, stderr.String()
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
}

func unavailableListed(t *testing.T, response map[string]any) []any {
	t.Helper()
	result, _ := response["result"].(map[string]any)
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list = %#v", response)
	}
	return tools
}

// MCPV0-950: a --root below a worktree's toplevel binds that toplevel, so the
// handshake succeeds and corvint.status observes the repository.
func TestMCPV0950SubdirectoryRootBindsToplevel(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	if output, err := exec.Command("git", "-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, output)
	}
	subdirectory := filepath.Join(root, "internal", "deep")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if toplevel, reason := rootbind.Resolve(subdirectory); reason != "" || !sameDirectory(t, toplevel, root) {
		t.Fatalf("Resolve=%q,%q want %q", toplevel, reason, root)
	}
	exit, responses, stderr := legacySession(t, []string{"--root", subdirectory, "--tool-profile", "test-validity,docs"},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.status", "arguments": map[string]any{}}})
	if exit != 0 || stderr != "" || responses[0]["result"] == nil {
		t.Fatalf("exit=%d stderr=%q initialize=%#v", exit, stderr, responses[0])
	}
	result, _ := responses[1]["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("status=%#v", responses[1])
	}
}

func sameDirectory(t *testing.T, left, right string) bool {
	t.Helper()
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

// MCPV0-950: a single plain root that is not usable still completes the
// handshake and lists the same tools as a usable root; every repository tool
// call is an isError invalid-root result, corvint.help still answers, an
// unknown tool stays invalid params, and stderr names the reason.
func TestMCPV0950UnusableRootServesListAndRefusesCalls(t *testing.T) {
	scratch := filepath.Clean(t.TempDir())
	repository := filepath.Join(scratch, "repo")
	gitInit(t, repository)
	plain := filepath.Join(scratch, "plain")
	file := filepath.Join(scratch, "file")
	symlinked := filepath.Join(scratch, "symlinked")
	for _, directory := range []string{plain, symlinked} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repository, ".git"), filepath.Join(symlinked, ".git")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	// Launch outside any worktree, so the missing root is not replaced by the
	// launch worktree (MCPV0-115) and keeps its MCPV0-950 outcome.
	if _, inside := launchWorktreeCandidate(scratch); inside {
		t.Skip("the temporary directory is inside a Git worktree")
	}
	t.Chdir(scratch)
	profile := "test-validity,docs"
	_, responses, _ := legacySession(t, []string{"--root", repository, "--tool-profile", profile}, map[string]any{"method": "tools/list", "params": map[string]any{}})
	want := unavailableListed(t, responses[1])
	for _, test := range []struct{ name, root, reason string }{
		{"non-repository directory", plain, rootbind.ReasonNoWorktree},
		{"missing directory", filepath.Join(scratch, "deleted"), rootbind.ReasonMissing},
		{"regular file", file, rootbind.ReasonNotDirectory},
		{"inside the git directory", filepath.Join(repository, ".git"), rootbind.ReasonInGitDir},
		{"symlinked marker", symlinked, rootbind.ReasonMarker},
		{"unexpanded variable", "${CLAUDE_PROJECT_DIR}", rootbind.ReasonNotAbsolute},
		{"empty", "", rootbind.ReasonNotAbsolute},
		{"unclean", scratch + "/plain/../plain", rootbind.ReasonUnclean},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := []map[string]any{{"method": "tools/list", "params": map[string]any{}}}
			for _, tool := range want {
				name := tool.(map[string]any)["name"].(string)
				requests = append(requests, map[string]any{"method": "tools/call", "params": map[string]any{"name": name, "arguments": map[string]any{}}})
			}
			requests = append(requests, map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.nope", "arguments": map[string]any{}}})
			exit, responses, stderr := legacySession(t, []string{"--root", test.root, "--tool-profile", profile}, requests...)
			wantStderr := "corvint-mcp: repository unavailable: " + test.reason + "; serving tools/list, and every repository tool call answers invalid-root\n"
			if exit != 0 || stderr != wantStderr || responses[0]["result"] == nil {
				t.Fatalf("exit=%d stderr=%q initialize=%#v", exit, stderr, responses[0])
			}
			if !reflect.DeepEqual(unavailableListed(t, responses[1]), want) {
				t.Fatalf("tools/list differs from a usable root's")
			}
			for index, tool := range want {
				name := tool.(map[string]any)["name"].(string)
				result, _ := responses[float64(index+2)]["result"].(map[string]any)
				structured, _ := result["structuredContent"].(map[string]any)
				if name == bridge.ToolHelp {
					if result["isError"] != false {
						t.Fatalf("help=%#v", responses[float64(index+2)])
					}
					continue
				}
				if result["isError"] != true || structured["code"] != invalidRoot || structured["tool"] != name {
					t.Fatalf("%s=%#v", name, responses[float64(index+2)])
				}
			}
			unknown, _ := responses[float64(len(want)+2)]["error"].(map[string]any)
			if unknown["code"] != float64(-32602) {
				t.Fatalf("unknown tool=%#v", responses[float64(len(want)+2)])
			}
		})
	}
}

// MCPV0-950: aliased roots, --follow-ref and a corpus artifact still need a
// usable repository at startup, so they exit 2, now naming the reason.
func TestMCPV0950UnusableRootStillExitsWhereARepositoryIsRequired(t *testing.T) {
	scratch := filepath.Clean(t.TempDir())
	repository := filepath.Join(scratch, "repo")
	gitInit(t, repository)
	plain := filepath.Join(scratch, "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"--root", "core=" + repository, "--root", "web=" + plain},
		{"--root", plain, "--follow-ref", "main"},
		{"--root", plain, "--tool-profile", "corpus", "--corpus-artifact", "corpus.json"},
	} {
		var stdout, stderr bytes.Buffer
		exit := run(context.Background(), arguments, bytes.NewReader(nil), &stdout, &stderr)
		if exit != 2 || stdout.Len() != 0 || stderr.String() != "corvint-mcp: repository unavailable: "+rootbind.ReasonNoWorktree+"\n" {
			t.Fatalf("%q exit=%d stderr=%q", arguments, exit, stderr.String())
		}
	}
}

// MCPV0-950: each profile's root-free surface is exactly what its registry
// advertises, so an unavailable server lists no more and no fewer tools.
func TestMCPV0950ProfileToolsMatchRegistries(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	for profile, open := range profileRegistries {
		registry, err := open(root)
		if err != nil {
			t.Fatalf("%q: %v", profile, err)
		}
		bound, _ := json.Marshal(registry.Tools())
		unbound, _ := json.Marshal(profileTools[profile].Tools())
		if string(bound) != string(unbound) {
			t.Fatalf("%q unbound tools differ:\n%s\n%s", profile, bound, unbound)
		}
	}
	if len(profileTools) != len(profileRegistries) {
		t.Fatalf("profileTools has %d profiles, profileRegistries %d", len(profileTools), len(profileRegistries))
	}
}
