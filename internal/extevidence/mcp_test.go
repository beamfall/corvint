package extevidence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func mcpSource(t *testing.T, mode, file string) string {
	source := providerCommand(t, "mcp", mode, file)
	return strings.Replace(source, commandSourcePrefix, mcpSourcePrefix, 1)
}

func serveMCPHelper(args []string) int {
	mode, path := args[0], args[1]
	if mode == "child" {
		time.Sleep(time.Minute)
		return 0
	}
	if strings.HasPrefix(mode, "descendant") {
		child := exec.Command(os.Args[0], "-test.run=^TestProviderCommandHelper$", "--", helperMarker, "mcp", "child", path)
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if child.Start() != nil {
			return 9
		}
		if os.WriteFile(path+".pid", []byte(strconv.Itoa(child.Process.Pid)), 0600) != nil {
			return 10
		}
		if mode == "descendant-stdin" {
			time.Sleep(time.Minute)
			return 0
		}
		if mode == "descendant-output" {
			os.Stdout.Write(bytes.Repeat([]byte("x"), maxMCPBytes+1))
			time.Sleep(time.Minute)
			return 0
		}
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"method":"initialize"`) {
		return 1
	}
	if mode == "timeout" {
		time.Sleep(time.Minute)
		return 1
	}
	if mode == "request" {
		io.WriteString(os.Stdout, `{"jsonrpc":"2.0","id":1,"method":"sampling/createMessage"}`+"\n")
		return 0
	}
	if mode == "overflow" {
		os.Stdout.Write(bytes.Repeat([]byte("x"), maxMCPBytes+1))
		return 0
	}
	if mode == "stderr" {
		os.Stderr.Write(bytes.Repeat([]byte("x"), maxCommandStderrBytes+1))
		return 0
	}
	if mode == "version" {
		io.WriteString(os.Stdout, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"1999-01-01","capabilities":{"tools":{}},"serverInfo":{}}}`+"\n")
		return 0
	}
	io.WriteString(os.Stdout, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"fixture","version":"1"}}}`+"\n")
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"method":"notifications/initialized"`) {
		return 2
	}
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"name":"corvint_evidence","arguments":{}`) {
		return 3
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 4
	}
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": string(data)}}}
	if mode == "structured" {
		result["structuredContent"] = map[string]any{}
	}
	if mode == "error" {
		result["isError"] = true
	}
	if mode == "case-error" {
		result["isError"] = true
		result["IsError"] = false
	}
	if mode == "null-error" {
		result["isError"] = nil
	}
	response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "result": result})
	if mode == "case-error" {
		response = bytes.Replace(response, []byte(`"IsError":false,`), nil, 1)
		response = bytes.Replace(response, []byte(`"isError":true`), []byte(`"isError":true,"IsError":false`), 1)
	}
	if mode == "duplicate" {
		response = bytes.Replace(response, []byte(`"jsonrpc":"2.0"`), []byte(`"jsonrpc":"2.0","jsonrpc":"2.0"`), 1)
	}
	os.Stdout.Write(append(response, '\n'))
	if mode == "extra" {
		io.WriteString(os.Stdout, `{"jsonrpc":"2.0","method":"notifications/message"}`+"\n")
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	if mode == "exit" {
		return 7
	}
	return 0
}

func TestMCPTransportConformance(t *testing.T) {
	t.Parallel()
	t.Run("EEP-MCP-003 unchanged records", func(t *testing.T) {
		transportConformance(t, func(file string) string { return mcpSource(t, "serve", file) })
	})
}

func TestMCPTransportFailures(t *testing.T) {
	repo := newRepository(t)
	file := writeRecord(t, t.TempDir(), "record.json", fixture(t, repo.head))
	for _, mode := range []string{"request", "overflow", "stderr", "version", "structured", "error", "case-error", "null-error", "duplicate", "extra", "exit"} {
		t.Run("EEP-MCP-002 "+mode, func(t *testing.T) {
			assertMCPClosed(t, Section(context.Background(), repo.index(), []string{mcpSource(t, mode, file)}, nil, nil, 10))
		})
	}
	t.Run("EEP-MCP-004 timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		assertMCPClosed(t, Section(ctx, repo.index(), []string{mcpSource(t, "timeout", file)}, nil, nil, 10))
	})
	t.Run("EEP-MCP-004 unavailable", func(t *testing.T) {
		source, _ := ParseMCP(`["/corvint-does-not-exist"]`)
		assertMCPClosed(t, Section(context.Background(), repo.index(), []string{source}, nil, nil, 10))
	})
}

func TestMCPDescendantCleanup(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("process groups require POSIX")
	}
	repo := newRepository(t)
	for _, mode := range []string{"descendant", "descendant-stdin", "descendant-output"} {
		t.Run("EEP-MCP-004 "+mode, func(t *testing.T) {
			file := writeRecord(t, t.TempDir(), "record.json", fixture(t, repo.head))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			section := Section(ctx, repo.index(), []string{mcpSource(t, mode, file)}, nil, nil, 10)
			raw, err := os.ReadFile(file + ".pid")
			if err != nil {
				t.Fatalf("descendant did not start: %v", err)
			}
			pid, err := strconv.Atoi(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			process, _ := os.FindProcess(pid)
			defer process.Release()
			probeCtx, probeCancel := context.WithTimeout(context.Background(), time.Second)
			defer probeCancel()
			state, _ := exec.CommandContext(probeCtx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
			if process.Signal(syscall.Signal(0)) == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
				_ = process.Kill()
				t.Fatalf("descendant %d survived: %s", pid, state)
			}
			if mode == "descendant" {
				row := section["providers"].([]any)[0].(map[string]any)
				if row["state"] != StateLoaded {
					t.Fatalf("normal session not loaded: %v", row)
				}
			} else {
				assertMCPClosed(t, section)
			}
		})
	}
}

func assertMCPClosed(t *testing.T, section map[string]any) {
	t.Helper()
	row := section["providers"].([]any)[0].(map[string]any)
	if row["state"] != StateUnavailable || row["sha256"] != "" {
		t.Fatalf("not closed: %v", row)
	}
	for _, field := range []string{"results", "downstream", "verification", "unknowns"} {
		if values, ok := section[field].([]any); ok && len(values) != 0 {
			t.Fatalf("partial %s: %v", field, values)
		}
	}
}
