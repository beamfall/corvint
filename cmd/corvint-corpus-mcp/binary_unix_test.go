//go:build unix

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func TestCorpusSeparateCLIMCPBinaries(t *testing.T) {
	t.Run("DCP-V1-002 DCP-V1-011 DCP-V1-016 cross binary", func(t *testing.T) {
		repository, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		binaries := t.TempDir()
		execute := func(dir string, input []byte, args ...string) []byte {
			t.Helper()
			executable, err := exec.LookPath(args[0])
			if err != nil {
				t.Fatal(err)
			}
			args[0] = executable
			result := procgroup.Run(t.Context(), procgroup.Spec{Argv: args, Dir: dir, Env: os.Environ(), Stdin: input, Timeout: 3 * time.Minute, OutputLimit: 4 << 20})
			if result.Err != nil || result.ExitStatus != 0 || !result.OwnedProcessGroupCleanup {
				t.Fatalf("%v: %+v %s", args, result, string(result.Stderr))
			}
			return result.Stdout
		}
		execute(repository, nil, "go", "build", "-o", binaries, "./cmd/corvint", "./cmd/corvint-corpus-mcp")
		root, _ := corpusServerFixture(t, "# Binary interoperability\n\nOriginal shared evidence.\n")
		cli := filepath.Join(binaries, "corvint")
		revision := strings.TrimSpace(string(execute(root, nil, "git", "rev-parse", "HEAD")))
		manifest := execute(root, nil, cli, "docs", "corpus", "manifest", "--revision", revision, "--scope", "source.md", "--timestamp", "2026-09-19T00:00:00Z")
		if err := os.WriteFile(filepath.Join(root, "input.json"), manifest, 0600); err != nil {
			t.Fatal(err)
		}
		artifact := execute(root, nil, cli, "docs", "corpus", "build", "--manifest", "input.json")
		if err := os.WriteFile(filepath.Join(root, "corpus.json"), artifact, 0600); err != nil {
			t.Fatal(err)
		}
		want := execute(root, nil, cli, "docs", "corpus", "search", "--artifact", "corpus.json", "--query", "Original")
		request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"name":"corvint.docs_search","arguments":{"query":"Original"}}}` + "\n")
		reply := execute(root, request, filepath.Join(binaries, "corvint-corpus-mcp"), "--root", root, "--artifact", "corpus.json")
		var result struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(reply, &result); err != nil {
			t.Fatal(err)
		}
		if result.Result.IsError || len(result.Result.Content) != 1 || !strings.Contains(result.Result.Content[0].Text, string(want)) {
			t.Fatalf("separate binary parity failed: %s", reply)
		}
	})
}
