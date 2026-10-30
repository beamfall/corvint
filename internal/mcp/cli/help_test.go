package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// MCPV0-100: on every closed profile, a corvint.help call answers on the wire
// with the embedded `corvint help --json` and `corvint help guide` bytes in
// both structuredContent and the enveloped text block. cmd/corvint's
// TestMCPV0100HelpToolMatchesTheCLI binds those bytes to the CLI's stdout.
func TestMCPV0100HelpToolAnswersOnEveryProfile(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` + meta + `,"name":"corvint.help","arguments":{}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{` + meta + `,"name":"corvint.help","arguments":{"topic":"guide"}}}` + "\n"
	want := map[float64]string{1: string(capcatalog.Snapshot()), 2: capcatalog.Guide()}
	for profile := range profileRegistries {
		arguments := []string{"--root", root}
		if profile != "" {
			arguments = append(arguments, "--tool-profile", profile)
		}
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), arguments, strings.NewReader(input), &stdout, &stderr); exit != 0 {
			t.Fatalf("%q exit=%d stderr=%q", arguments, exit, stderr.String())
		}
		lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("%q responses=%d", arguments, len(lines))
		}
		for _, line := range lines {
			var response struct {
				ID     float64 `json:"id"`
				Result struct {
					IsError           bool `json:"isError"`
					StructuredContent struct {
						Receipt struct {
							Text string `json:"text"`
						} `json:"receipt"`
					} `json:"structuredContent"`
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(line), &response); err != nil {
				t.Fatal(err)
			}
			result := response.Result
			if result.IsError || result.StructuredContent.Receipt.Text != want[response.ID] || len(result.Content) != 1 {
				t.Fatalf("%q id %v: result differs from the embedded help bytes", arguments, response.ID)
			}
			framed := result.Content[0].Text
			begin := strings.Index(framed, "\n{")
			end := strings.LastIndex(framed, "\n"+repoenvelope.Terminator)
			var text struct {
				Receipt struct {
					Text string `json:"text"`
				} `json:"receipt"`
			}
			if begin < 0 || end < begin || json.Unmarshal([]byte(framed[begin+1:end]), &text) != nil || text.Receipt.Text != want[response.ID] {
				t.Fatalf("%q id %v: text block differs from the embedded help bytes", arguments, response.ID)
			}
		}
	}
}

// MCPV0-100: under --follow-ref, corvint.help never resolves, checks out or
// reindexes the followed ref, and appends no follow-ref block: it answers from
// the binary when the ref is current, moved or deleted.
func TestMCPV0100HelpToolSkipsFollowRefBinding(t *testing.T) {
	main, lane := followFixture(t)
	head := gitTest(t, main, "rev-parse", "HEAD")
	gitTest(t, main, "update-ref", "refs/heads/follow", head)

	// Over stdio: a fresh server started with --follow-ref.
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` + meta + `,"name":"corvint.help","arguments":{"topic":"guide"}}}` + "\n"
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", lane, "--follow-ref", "follow"}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	if strings.Contains(stdout.String(), followRefSchema) || !strings.Contains(stdout.String(), `"isError":false`) {
		t.Fatalf("help under --follow-ref carried a follow-ref block or a tool error (%d bytes)", stdout.Len())
	}

	handler := followHandler(t, lane, "follow")
	handler.follow.refresh = func(context.Context, string) (bool, error) {
		t.Error("corvint.help refreshed the index of the followed ref")
		return false, nil
	}
	answer := func(state string) {
		t.Helper()
		result, failure := handler.Handle(context.Background(), protocol.Request{Method: "tools/call", Params: map[string]any{
			"_meta": map[string]any{}, "name": "corvint.help", "arguments": map[string]any{},
		}}, nil)
		if failure != nil {
			t.Fatalf("%s: protocol failure %#v", state, failure)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		receipt, _ := structured["receipt"].(map[string]any)
		content, _ := result["content"].([]any)
		if result["isError"] != false || receipt["text"] != string(capcatalog.Snapshot()) || len(content) != 1 {
			t.Fatalf("%s ref: help answer isError=%v blocks=%d differs from the embedded catalog", state, result["isError"], len(content))
		}
		if got := gitTest(t, lane, "rev-parse", "HEAD"); got != head {
			t.Fatalf("%s ref: help moved the followed worktree to %s", state, got)
		}
	}
	answer("current")
	gitTest(t, main, "update-ref", "refs/heads/follow", commitFile(t, main, "NEW.md", "moved\n"))
	answer("moved")
	gitTest(t, main, "update-ref", "-d", "refs/heads/follow")
	answer("deleted")
}
