package testsupport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	fakeGoplsServeEnv       = "CORVINT_TEST_FAKE_GOPLS"
	fakeGoplsDefinitionsEnv = "CORVINT_TEST_FAKE_GOPLS_DEFINITIONS"
)

// ServeFakeGoplsIfRequested turns a test binary into a scripted gopls when
// FakeGoplsExecutable launched it, and exits when the session ends. Call it
// first in TestMain. The fake advertises UTF-16 definition and references,
// answers every definition query with the configured paths and every
// references query with none.
func ServeFakeGoplsIfRequested() {
	if os.Getenv(fakeGoplsServeEnv) == "" {
		return
	}
	serveFakeGopls(os.Getenv(fakeGoplsDefinitionsEnv))
	os.Exit(0)
}

// FakeGoplsExecutable writes an executable named gopls that relaunches the
// current test binary as the scripted server. definitions is a
// comma-separated list of paths, relative to the initialize root or absolute
// for a path outside it. The test binary's TestMain must call
// ServeFakeGoplsIfRequested.
func FakeGoplsExecutable(t testing.TB, definitions string) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gopls")
	script := fmt.Sprintf("#!/bin/sh\n%s=1 %s=%q exec %q \"$@\"\n", fakeGoplsServeEnv, fakeGoplsDefinitionsEnv, definitions, binary)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func serveFakeGopls(definitions string) {
	reader := bufio.NewReader(os.Stdin)
	root := ""
	for {
		length := -1
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
				length, _ = strconv.Atoi(strings.TrimSpace(value))
			}
		}
		body := make([]byte, max(length, 0))
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				RootURI string `json:"rootUri"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &request)
		if request.Method == "exit" {
			return
		}
		if len(request.ID) == 0 {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			if parsed, err := url.Parse(request.Params.RootURI); err == nil {
				root = parsed.Path
			}
			result = map[string]any{"serverInfo": map[string]any{"name": "gopls", "version": "v0.0.0-fake"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "definitionProvider": true, "referencesProvider": true}}
		case "textDocument/definition":
			locations := []any{}
			for _, path := range strings.Split(definitions, ",") {
				if path == "" {
					continue
				}
				if !filepath.IsAbs(path) {
					path = filepath.Join(root, filepath.FromSlash(path))
				}
				locations = append(locations, map[string]any{"uri": (&url.URL{Scheme: "file", Path: path}).String(), "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}})
			}
			result = locations
		case "textDocument/references":
			result = []any{}
		}
		reply, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(reply), reply)
	}
}
