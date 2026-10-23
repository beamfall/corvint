package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
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
