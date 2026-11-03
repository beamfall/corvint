package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestAHI631ClaudePluginMCPLaunchHandshakes is the maintained plugin smoke test (AHI-631,
// V1-3894). It launches the real built binary exactly as Claude Code does from the plugin's
// .mcp.json: `command` resolved on PATH, the declared args with ${CLAUDE_PROJECT_DIR}
// substituted, and the declared env. In a Git project and in a non-repository project directory
// the server must answer initialize, list exactly CORVINT_PLUGIN_TOOLS, and report the plugin
// compatible from corvint.status, instead of closing the connection before the handshake.
func TestAHI631ClaudePluginMCPLaunchHandshakes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "integrations", "claude-code", "plugins", "corvint", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		t.Fatal(err)
	}
	server := declared.MCPServers["corvint"]
	binary := portableDogfoodRunner(t).binary
	if filepath.Base(binary) != server.Command && filepath.Base(binary) != server.Command+".exe" {
		t.Fatalf("declared command %q is not the corvint binary %q", server.Command, binary)
	}
	repository := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", repository).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	for name, project := range map[string]string{"repository": repository, "not-a-repository": t.TempDir()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			args := slices.Clone(server.Args)
			for i, arg := range args {
				args[i] = strings.ReplaceAll(arg, "${CLAUDE_PROJECT_DIR}", project)
			}
			env := append(os.Environ(), "PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"), "CLAUDE_PROJECT_DIR="+project)
			for key, value := range server.Env {
				env = append(env, key+"="+value)
			}
			input := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"claude-code","version":"2.1.316"}}}` + "\n" +
				`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
				`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.status","arguments":{}}}` + "\n"
			var stdout, stderr bytes.Buffer
			command, err := startCopiedBinary(func() *exec.Cmd {
				command := exec.Command(lookPathIn(t, env, server.Command), args...)
				command.Dir, command.Env = project, env
				command.Stdin, command.Stdout, command.Stderr = strings.NewReader(input), &stdout, &stderr
				stdout.Reset()
				stderr.Reset()
				return command
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := command.Wait(); err != nil {
				t.Fatalf("server exited %v before answering: stdout=%s stderr=%s", err, &stdout, &stderr)
			}
			answers := map[int]json.RawMessage{}
			for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
				var answer struct {
					ID     *int
					Result json.RawMessage
				}
				if json.Unmarshal(line, &answer) == nil && answer.ID != nil && answer.Result != nil {
					answers[*answer.ID] = answer.Result
				}
			}
			if len(answers) != 3 {
				t.Fatalf("answers=%d, want initialize, tools/list and status: stdout=%s stderr=%s", len(answers), &stdout, &stderr)
			}
			var listed struct{ Tools []struct{ Name string } }
			if err := json.Unmarshal(answers[1], &listed); err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, tool := range listed.Tools {
				names = append(names, tool.Name)
			}
			if expected := strings.Split(server.Env["CORVINT_PLUGIN_TOOLS"], ","); !slices.Equal(expected, names) {
				t.Fatalf("CORVINT_PLUGIN_TOOLS=%q, tools/list %q", expected, names)
			}
			var status struct {
				Meta map[string]struct {
					Compatible bool   `json:"compatible"`
					Plugin     string `json:"plugin"`
				} `json:"_meta"`
			}
			if err := json.Unmarshal(answers[2], &status); err != nil {
				t.Fatal(err)
			}
			if compatibility := status.Meta["io.corvint/pluginCompatibility"]; !compatibility.Compatible || compatibility.Plugin != server.Env["CORVINT_PLUGIN_VERSION"] {
				t.Fatalf("corvint.status compatibility=%+v: %s", compatibility, answers[2])
			}
		})
	}
}

// lookPathIn resolves name on env's PATH, as the host does, rather than on this process's PATH.
func lookPathIn(t *testing.T, env []string, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for i := len(env) - 1; i >= 0; i-- {
		value, ok := strings.CutPrefix(env[i], "PATH=")
		if !ok {
			continue
		}
		for _, directory := range filepath.SplitList(value) {
			if info, err := os.Stat(filepath.Join(directory, name)); err == nil && !info.IsDir() {
				return filepath.Join(directory, name)
			}
		}
		break
	}
	t.Fatalf("%s is not on the host PATH", name)
	return ""
}
