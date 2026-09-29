package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

const lspArgs = `{"task":"Find widget callers","subject":"internal/widget/widget.go","lsp":"gopls"}`

// MCPV0-029: two explicit boundaries, old descriptors and arguments unchanged.
func TestContextLSPProfileIsExplicit(t *testing.T) {
	t.Run("MCPV0-029 explicit activation", func(t *testing.T) {
		root := makeRepository(t)
		t.Setenv("CORVINT_CONTEXT_LSP", "gopls")
		legacy, e := NewTaskReview(root)
		if e != nil {
			t.Fatal(e)
		}
		for _, value := range []string{`"gopls"`, `"off"`, `""`, `null`, `true`} {
			if _, err := legacy.Call(context.Background(), ToolContext, []byte(`{"task":"x","lsp":`+value+`}`)); err == nil || err.Code != "invalid-arguments" {
				t.Fatalf("legacy accepted %s: %v", value, err)
			}
		}
		registry, e := NewTaskReviewLSP(root)
		if e != nil {
			t.Fatal(e)
		}
		registry.lspExecutable = "" // deterministic unavailable executable
		for _, value := range []string{`""`, `null`, `true`, `"other"`, `[]`} {
			if _, err := registry.Call(context.Background(), ToolContext, []byte(`{"task":"x","lsp":`+value+`}`)); err == nil || err.Code != "invalid-arguments" {
				t.Fatalf("accepted %s: %v", value, err)
			}
		}
		for _, args := range []string{`{"task":"x"}`, `{"task":"x","lsp":"off"}`} {
			result, err := registry.Call(context.Background(), ToolContext, []byte(args))
			if err != nil || result.Receipt["external"] != nil {
				t.Fatalf("ambient activation: %v %v", result, err)
			}
		}
		before := rootDigest(t, root)
		result, err := registry.Call(context.Background(), ToolContext, []byte(lspArgs))
		if err != nil {
			t.Fatal(err)
		}
		external := result.Receipt["external"].(map[string]any)
		row := external["providers"].([]any)[0].(map[string]any)
		if row["state"] != "unavailable" || row["reason"] != "gopls executable not found" {
			t.Fatalf("%v", row)
		}
		if before != rootDigest(t, root) {
			t.Fatal("repository mutated")
		}
		for _, r := range []*Registry{legacy, registry} {
			for _, tool := range r.Tools() {
				if tool.Name == ToolContext {
					_, present := tool.InputSchema["properties"].(map[string]any)["lsp"]
					if present != r.lsp {
						t.Fatal("profile descriptor leaked")
					}
				}
			}
		}
	})
}

// MCPV0-030/TCP-V0-052: both Git and gopls remain pinned after startup, and
// external evidence is withheld if the workspace changes during the session.
func TestContextLSPPinAndDrift(t *testing.T) {
	t.Run("MCPV0-030 pinned execution", func(t *testing.T) {
		for _, kind := range []string{"stable", "dirty", "head"} {
			t.Run(kind, func(t *testing.T) {
				root := makeRepository(t)
				git, err := gitstatus.Pin()
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				marker := filepath.Join(dir, "ran")
				mutation := ""
				if kind == "dirty" {
					mutation = "printf '\\n// changed\\n' >> internal/widget/widget.go\n"
				}
				if kind == "head" {
					mutation = "'" + git + "' -c core.hooksPath=/dev/null commit -q --allow-empty -m drift\n"
				}
				body := "#!/bin/sh\nprintf ran > '" + marker + "'\n" + mutation + "exit 3\n"
				if err := os.WriteFile(filepath.Join(dir, "gopls"), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
				oldPath := os.Getenv("PATH")
				t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
				registry, e := NewTaskReviewLSP(root)
				if e != nil {
					t.Fatal(e)
				}
				hostile := t.TempDir()
				bad := filepath.Join(hostile, "executed")
				for _, name := range []string{"git", "gopls"} {
					if err := os.WriteFile(filepath.Join(hostile, name), []byte("#!/bin/sh\nprintf bad > '"+bad+"'\nexit 1\n"), 0755); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("PATH", hostile+string(os.PathListSeparator)+oldPath)
				result, callErr := registry.Call(context.Background(), ToolContext, []byte(lspArgs))
				if callErr != nil {
					t.Fatal(callErr)
				}
				if _, err := os.Stat(bad); !os.IsNotExist(err) {
					t.Fatal("PATH replacement executed")
				}
				if _, err := os.Stat(marker); err != nil {
					t.Fatal("pinned server did not run", err)
				}
				if kind != "stable" {
					if result.State != "ABSTAINED" || result.Abstention.Reason != "REPOSITORY_STATE_UNSTABLE" || result.Receipt != nil {
						t.Fatalf("drift emitted: %+v", result)
					}
				} else if !strings.Contains(string(mustCanonicalLSP(t, result)), "gopls session failed") {
					t.Fatalf("missing failure row: %+v", result)
				}
			})
		}
	})
}
func mustCanonicalLSP(t *testing.T, r Result) []byte {
	t.Helper()
	b, e := r.CanonicalJSON()
	if e != nil {
		t.Fatal(e)
	}
	return b
}
