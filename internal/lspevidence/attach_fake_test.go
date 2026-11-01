package lspevidence

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// fakeGoplsEnv makes the test binary act as a scripted gopls: every
// definition query answers the comma-separated LSPEVIDENCE_FAKE_DEFINITIONS
// paths (repository-relative, or absolute for outside the root), and every
// references query answers nothing.
const fakeGoplsEnv = "LSPEVIDENCE_FAKE_DEFINITIONS"

func serveFakeGopls() {
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
			parsed, _ := url.Parse(request.Params.RootURI)
			root = parsed.Path
			result = map[string]any{"serverInfo": map[string]any{"name": "gopls", "version": "v0.0.0-fake"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "definitionProvider": true, "referencesProvider": true}}
		case "textDocument/definition":
			locations := []any{}
			for _, path := range strings.Split(os.Getenv(fakeGoplsEnv), ",") {
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

func fakeGoplsExecutable(t *testing.T, definitions string) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gopls")
	script := fmt.Sprintf("#!/bin/sh\n%s=%q LSPEVIDENCE_FAKE_SERVE=1 exec %q \"$@\"\n", fakeGoplsEnv, definitions, binary)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAttachCarriesProvenanceAndDedupes runs Attach against a scripted gopls
// whose definition answers repeat a path, name a path outside the root, name
// a file with uncommitted edits and name a Python file. The section must keep
// one pinned relation per returned committed path with its source, snapshot
// and inclusion reason (LQP-V0-005), count the outside and unpinned answers,
// add no relation to c/c.go, which declares the same name B but was never
// returned (LQP-V0-006), and leave every other packet member unchanged under
// the external-provider authority (V1-0480).
func TestAttachCarriesProvenanceAndDedupes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/m\n\ngo 1.22\n",
		"a/a.go": "package a\n\nimport \"example.test/m/b\"\n\nfunc A() int { return b.B() }\n",
		"b/b.go": "package b\n\nfunc B() int { return 1 }\n",
		"c/c.go": "package c\n\nfunc B() int { return 2 }\n",
		"d/d.go": "package d\n\nfunc D() int { return 3 }\n",
		"x.py":   "def B():\n    return 4\n",
	}
	for path, text := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "c@example.test"}, {"config", "user.name", "C"}, {"add", "."}, {"commit", "-qm", "first"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "d", "d.go"), []byte("package d\n\nfunc D() int { return 9 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	index, err := contextindex.Build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{"results": []any{map[string]any{"id": "a/a.go", "rank": 1}}, "schema_version": 1}
	before, _ := json.Marshal(packet)
	executable := fakeGoplsExecutable(t, "b/b.go,b/b.go,"+filepath.Join(t.TempDir(), "z.go")+",x.py,d/d.go")
	if err := Attach(ctx, index, "a/a.go", packet, 6, executable); err != nil {
		t.Fatal(err)
	}
	section, _ := packet["external"].(map[string]any)
	delete(packet, "external")
	if after, _ := json.Marshal(packet); string(after) != string(before) {
		t.Fatalf("Attach changed a non-external member:\nbefore %s\nafter  %s", before, after)
	}
	if section["authority"] != "external-provider" {
		t.Fatalf("authority = %v", section["authority"])
	}
	query, _ := section["query"].(map[string]any)
	if fmt.Sprint(query["outside_repository"]) != "1" || fmt.Sprint(query["omitted_unpinned"]) != "1" {
		t.Fatalf("outside_repository = %v, omitted_unpinned = %v; want 1 and 1", query["outside_repository"], query["omitted_unpinned"])
	}
	rows, _ := section["path_relations"].([]any)
	var targets []string
	for _, row := range rows {
		item := row.(map[string]any)
		relation := item["relation"].(map[string]any)
		to := relation["to"].(map[string]any)
		from := relation["from"].(map[string]any)
		targets = append(targets, fmt.Sprint(to["path"]))
		if item["provider"] != "gopls" || relation["evidence"] != "inferred" || item["verification"] != "verified" {
			t.Fatalf("relation lacks its source or verification: %v", item)
		}
		if !strings.HasPrefix(fmt.Sprint(relation["rule"]), "gopls textDocument/definition at a/a.go:") ||
			!strings.HasPrefix(fmt.Sprint(relation["reference"]), "query sha256:") ||
			!strings.Contains(fmt.Sprint(item["reason"]), "related by inferred path-to-path relation") {
			t.Fatalf("relation lacks its inclusion reason: %v", item)
		}
		if fmt.Sprint(from["blob"]) == "" || fmt.Sprint(to["blob"]) == "" {
			t.Fatalf("relation endpoint is not pinned to a blob: %v", relation)
		}
	}
	if want := []string{"b/b.go", "x.py"}; !reflect.DeepEqual(targets, want) {
		t.Fatalf("relation targets = %v, want %v (one per returned committed path; none to c/c.go or the dirty d/d.go)", targets, want)
	}
}
