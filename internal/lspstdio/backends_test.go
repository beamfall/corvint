// SPDX-License-Identifier: Apache-2.0
package lspstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/goplsclient"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

const wrappedText = "package p\n/*😀*/ var x int\nvar y = x\n"

// fakeServer writes an in-repo fake language server: a script that re-enters
// this test binary as TestSemanticHost under name. No real server is used.
func fakeServer(t *testing.T, scratch, name, mode string) string {
	t.Helper()
	exe, _ := os.Executable()
	if strings.Contains(exe, "'") || strings.Contains(scratch, "'") {
		t.Fatal("quote")
	}
	binary := filepath.Join(scratch, "fake-"+mode+"-"+name)
	body := "#!/bin/sh\nexport CORVINT_SEMANTIC_FAKE=" + mode + "\nexport CORVINT_SEMANTIC_NAME=" + name + "\nexport CORVINT_SEMANTIC_SCRATCH='" + scratch + "'\nexec '" + exe + "' -test.run '^TestSemanticHost$' -- \"$@\"\n"
	if name == "" {
		body = "#!/bin/sh\nexport CORVINT_SEMANTIC_FAKE=" + mode + "\nexport CORVINT_SEMANTIC_SCRATCH='" + scratch + "'\nexec '" + exe + "' -test.run '^TestSemanticHost$' -- serve\n"
	}
	if e := os.WriteFile(binary, []byte(body), 0700); e != nil {
		t.Fatal(e)
	}
	return binary
}

func wrappedHarness(t *testing.T, build func(scratch string) SemanticConfig) *semanticHarness {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	config := build(root)
	config.Root = root
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	h := &semanticHarness{in: inW, reader: bufio.NewReader(outR), done: make(chan error, 1), cancel: cancel, root: root, scratch: root}
	go func() { h.done <- ServeSemantic(ctx, inR, outW, config) }()
	t.Cleanup(func() {
		cancel()
		inW.Close()
		outR.Close()
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			t.Error("session leaked")
		}
	})
	return h
}

func (h *semanticHarness) docURI(name string) string {
	return (&url.URL{Scheme: "file", Path: filepath.Join(h.root, name)}).String()
}

// initialize returns the experimental capability map.
func (h *semanticHarness) initialize(t *testing.T) (map[string]json.RawMessage, json.RawMessage) {
	t.Helper()
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": (&url.URL{Scheme: "file", Path: h.root}).String(), "capabilities": map[string]any{}}})
	m := h.recv(t)
	var result struct {
		Capabilities struct {
			Experimental map[string]json.RawMessage `json:"experimental"`
		} `json:"capabilities"`
	}
	json.Unmarshal(m["result"], &result)
	if m["error"] == nil {
		h.send(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
	}
	return result.Capabilities.Experimental, m["error"]
}

func (h *semanticHarness) openDoc(t *testing.T, name, language string) string {
	uri := h.docURI(name)
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "languageId": language, "text": wrappedText}}})
	return uri
}

func (h *semanticHarness) request(t *testing.T, id int, method, uri string) map[string]json.RawMessage {
	t.Helper()
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": goplsclient.Position{Line: 2, Character: 8}}
	if method == "corvint/context" {
		params = map[string]any{"textDocument": map[string]any{"uri": uri}, "task": "investigate", "limit": 3}
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	m := h.recv(t)
	if string(m["id"]) != fmt.Sprint(id) {
		t.Fatalf("reply order: %s", m)
	}
	return m
}

type wireFailure struct {
	Code int
	Data struct{ Reason string }
}

func failureOf(m map[string]json.RawMessage) wireFailure {
	var f wireFailure
	json.Unmarshal(m["error"], &f)
	return f
}

type observedRow struct {
	URI         string
	Observation map[string]string `json:"corvintObservation"`
}

func observer(t *testing.T, m map[string]json.RawMessage) string {
	t.Helper()
	var rows []observedRow
	if m["error"] != nil || json.Unmarshal(m["result"], &rows) != nil || len(rows) != 1 {
		t.Fatalf("definition: %s", m)
	}
	o := rows[0].Observation
	if len(o) != 3 || o["class"] != "wrapped-server-observation" || o["authority"] != "NONE" {
		t.Fatalf("observation label %v", o)
	}
	return o["observer"]
}

// LWB-V0-001 LWB-V0-002 LWB-V0-003 LWB-V0-005
func TestWrappedSuffixRouting(t *testing.T) {
	var scratch string
	h := wrappedHarness(t, func(dir string) SemanticConfig {
		scratch = dir
		ruby, e := ParseServer(".rb,.rake=" + fakeServer(t, dir, "fake-ruby", "normal") + " --stdio-free")
		if e != nil {
			t.Fatal(e)
		}
		return SemanticConfig{Executable: fakeServer(t, dir, "", "normal"), Backends: []BackendConfig{TypeScriptBackend(fakeServer(t, dir, "fake-ts", "normal")), ruby}}
	})
	experimental, failure := h.initialize(t)
	if failure != nil {
		t.Fatal(string(failure))
	}
	var described []map[string]any
	if e := json.Unmarshal(experimental["corvintBackends"], &described); e != nil || len(described) != 3 {
		t.Fatal("backend description", e, string(experimental["corvintBackends"]))
	}
	for i, want := range [][2]string{{"gopls", "gopls"}, {"typescript", "fake-ts"}, {"ruby", "fake-ruby"}} {
		d := described[i]
		if d["name"] != want[0] || d["serverName"] != want[1] || d["authority"] != "NONE" || d["observationClass"] != "wrapped-server-observation" || len(d["executableSHA256"].(string)) != 64 {
			t.Fatalf("backend %d: %v", i, d)
		}
	}
	goURI := h.openDoc(t, "main.go", "go")
	tsURI := h.openDoc(t, "web/app.ts", "typescript")
	tsxURI := h.openDoc(t, "web/view.tsx", "typescriptreact")
	rbURI := h.openDoc(t, "lib/widget.rb", "ruby")
	for i, v := range []struct{ uri, want string }{{goURI, "gopls"}, {tsURI, "typescript"}, {tsxURI, "typescript"}, {rbURI, "ruby"}} {
		if got := observer(t, h.request(t, 10+i, "textDocument/definition", v.uri)); got != v.want {
			t.Fatalf("%s served by %s", v.uri, got)
		}
	}
	read := func(name string) string { b, _ := os.ReadFile(filepath.Join(scratch, name)); return string(b) }
	if read("languagesfake-ts") != "typescript\ntypescriptreact\n" || read("languagesfake-ruby") != "ruby\n" {
		t.Fatalf("languageId routing: %q %q", read("languagesfake-ts"), read("languagesfake-ruby"))
	}
	if read("argvfake-ts") != "--stdio" || read("argvfake-ruby") != "--stdio-free" {
		t.Fatalf("argv: %q %q", read("argvfake-ts"), read("argvfake-ruby"))
	}
	pids := []string{read("backend-pid"), read("backend-pidfake-ts"), read("backend-pidfake-ruby")}
	slices.Sort(pids)
	if pids[0] == "" || len(slices.Compact(pids)) != 3 {
		t.Fatal("one process per backend", pids)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 30, "method": "shutdown"})
	if m := h.recv(t); m["error"] != nil {
		t.Fatal(string(m["error"]))
	}
}

// LWB-V0-002 LWB-V0-007
func TestWrappedUnsupportedSuffix(t *testing.T) {
	h := wrappedHarness(t, func(dir string) SemanticConfig {
		return SemanticConfig{Backends: []BackendConfig{TypeScriptBackend(fakeServer(t, dir, "fake-ts", "normal"))}}
	})
	if _, failure := h.initialize(t); failure != nil {
		t.Fatal(string(failure))
	}
	pyURI := h.openDoc(t, "tool.py", "python")
	goURI := h.openDoc(t, "main.go", "go")
	id := 20
	for _, uri := range []string{pyURI, goURI, h.docURI("README")} {
		for _, method := range []string{"textDocument/definition", "corvint/context"} {
			id++
			if f := failureOf(h.request(t, id, method, uri)); f.Code != -32602 || f.Data.Reason != "UNSUPPORTED_SUFFIX" {
				t.Fatalf("%s %s: %+v", method, uri, f)
			}
		}
	}
	// The session and the routed backend continue.
	if got := observer(t, h.request(t, 40, "textDocument/definition", h.openDoc(t, "web/app.mjs", "javascript"))); got != "typescript" {
		t.Fatal(got)
	}
}

// LWB-V0-006 LWB-V0-007
func TestBackendConfiguration(t *testing.T) {
	dir := t.TempDir()
	fake := fakeServer(t, dir, "fake-ts", "normal")
	for spec, want := range map[string]error{".py=" + fake: ErrUnsupportedSuffix, ".go=" + fake: ErrUnsupportedSuffix, ".TS=" + fake: ErrUnsupportedSuffix, ".ts,.rb=" + fake: ErrBackendConfig, ".ts,.ts=" + fake: ErrBackendConfig, "=" + fake: ErrBackendConfig, ".ts=": ErrBackendConfig, ".ts": ErrBackendConfig} {
		if _, e := ParseServer(spec); !errors.Is(e, want) {
			t.Errorf("%s: %v", spec, e)
		}
	}
	b, e := ParseServer(".ts,.js=" + fake + " --stdio --log-level=4")
	if e != nil || b.Family != "typescript" || !slices.Equal(b.Suffixes, []string{".ts", ".js"}) || !slices.Equal(b.Argv, []string{fake, "--stdio", "--log-level=4"}) {
		t.Fatal(b, e)
	}
	missing := filepath.Join(dir, "absent-ruby-lsp")
	for name, v := range map[string]struct {
		gopls    string
		backends []BackendConfig
		want     error
	}{
		"valid":            {"", []BackendConfig{TypeScriptBackend(fake), RubyBackend(fake)}, nil},
		"missing wrapped":  {"", []BackendConfig{TypeScriptBackend(fake), RubyBackend(missing)}, ErrServerBinaryMissing},
		"missing gopls":    {missing, nil, ErrServerBinaryMissing},
		"relative":         {"", []BackendConfig{RubyBackend("ruby-lsp")}, ErrBackendConfig},
		"two per family":   {"", []BackendConfig{TypeScriptBackend(fake), {Family: "typescript", Argv: []string{fake}, Suffixes: []string{".tsx"}}}, ErrBackendConfig},
		"family mismatch":  {"", []BackendConfig{{Family: "ruby", Argv: []string{fake}, Suffixes: []string{".ts"}}}, ErrUnsupportedSuffix},
		"unknown family":   {"", []BackendConfig{{Family: "python", Argv: []string{fake}, Suffixes: []string{".py"}}}, ErrBackendConfig},
		"no suffixes":      {"", []BackendConfig{{Family: "ruby", Argv: []string{fake}}}, ErrBackendConfig},
		"duplicate suffix": {"", []BackendConfig{{Family: "ruby", Argv: []string{fake}, Suffixes: []string{".rb", ".rb"}}}, ErrBackendConfig},
	} {
		if e := CheckBackends(v.gopls, v.backends); !errors.Is(e, v.want) && !(v.want == nil && e == nil) {
			t.Errorf("%s: %v", name, e)
		}
	}
}

// LWB-V0-006: a server binary removed after configuration is refused at
// initialize, and any backend already started is retired with it.
func TestWrappedMissingBinaryAtInitialize(t *testing.T) {
	h := wrappedHarness(t, func(dir string) SemanticConfig {
		return SemanticConfig{Backends: []BackendConfig{TypeScriptBackend(fakeServer(t, dir, "fake-ts", "normal")), RubyBackend(filepath.Join(dir, "absent-ruby-lsp"))}}
	})
	_, raw := h.initialize(t)
	var f wireFailure
	if json.Unmarshal(raw, &f) != nil || f.Code != -32001 || f.Data.Reason != "SERVER_BINARY_MISSING" {
		t.Fatalf("%s", raw)
	}
	if bytes.Contains(raw, []byte("absent-ruby-lsp")) {
		t.Fatal("refusal leaked a path")
	}
}

// LWB-V0-008: a wrapped server that exits latches SERVER_CRASHED for its
// suffixes; the session, other backends and Corvint-owned paths continue.
func TestWrappedServerCrash(t *testing.T) {
	h := wrappedHarness(t, func(dir string) SemanticConfig {
		return SemanticConfig{Backends: []BackendConfig{TypeScriptBackend(fakeServer(t, dir, "fake-ts", "crash")), RubyBackend(fakeServer(t, dir, "fake-ruby", "normal"))}}
	})
	if _, failure := h.initialize(t); failure != nil {
		t.Fatal(string(failure))
	}
	tsURI := h.openDoc(t, "web/app.ts", "typescript")
	rbURI := h.openDoc(t, "lib/widget.rb", "ruby")
	for id := 50; id < 52; id++ {
		if f := failureOf(h.request(t, id, "textDocument/definition", tsURI)); f.Code != -32001 || f.Data.Reason != "SERVER_CRASHED" {
			t.Fatalf("attempt %d: %+v", id, f)
		}
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": tsURI, "version": 2}, "contentChanges": []any{map[string]any{"text": wrappedText + "\n"}}}})
	if got := observer(t, h.request(t, 53, "textDocument/definition", rbURI)); got != "ruby" {
		t.Fatal(got)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 54, "method": "shutdown"})
	if m := h.recv(t); m["error"] != nil {
		t.Fatal(string(m["error"]))
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	select {
	case e := <-h.done:
		h.done <- e
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("exit")
	}
}

// LWB-V0-009: the built corvint-lsp wraps fake TS and Ruby servers under the
// workspace drift guard, and corvint/context for those documents returns the
// exact Core object at the bound revision, advisory labels included.
func TestWrappedContextAtBoundRevision(t *testing.T) {
	executable := contextBinary(t)
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	files := map[string]string{"AGENTS.md": "Widget changes need test/widget_test.rb and web/app.test.ts.\n", "lib/widget.rb": "module Shop\n  autoload :Cart, \"shop/cart\"\n  class Widget\n    def value = 42\n  end\nend\n", "lib/shop/cart.rb": "module Shop\n  class Cart\n  end\nend\n", "test/widget_test.rb": "require_relative \"../lib/widget\"\n", "web/app.ts": "export function value(): number { return 42 }\n", "web/app.test.ts": "import { value } from \"./app\"\n"}
	for path, body := range files {
		full := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(full), 0700)
		if e := os.WriteFile(full, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Fixture"}} {
		fixtureGit(t, root, args...)
	}
	head := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD")))
	scratch, _ := filepath.EvalSymlinks(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, executable, "--experimental", "--root", root, "--workspace-drift-guard", "--tsserver", fakeServer(t, scratch, "fake-ts", "normal"), "--ruby-lsp", fakeServer(t, scratch, "fake-ruby", "normal"))
	stdin, _ := c.StdinPipe()
	stdout, _ := c.StdoutPipe()
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if e := c.Start(); e != nil {
		t.Fatal(e)
	}
	h := &semanticHarness{in: nil, reader: bufio.NewReader(stdout), root: root}
	send := func(m any) {
		b, _ := json.Marshal(m)
		if _, e := fmt.Fprint(stdin, frame(string(b))); e != nil {
			t.Fatal(e, stderr.String())
		}
	}
	recv := func() map[string]json.RawMessage {
		b, e := readFrame(h.reader)
		if e != nil {
			t.Fatal(e, stderr.String())
		}
		var m map[string]json.RawMessage
		json.Unmarshal(b, &m)
		return m
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": (&url.URL{Scheme: "file", Path: root}).String(), "capabilities": map[string]any{}}})
	if m := recv(); m["error"] != nil || !bytes.Contains(m["result"], []byte(`"corvintWorkspaceDriftGuard"`)) {
		t.Fatalf("initialize: %s", m)
	}
	send(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
	registry, failure := bridge.NewTaskReview(root)
	if failure != nil {
		t.Fatal(failure)
	}
	for i, v := range []struct{ path, language string }{{"web/app.ts", "typescript"}, {"lib/widget.rb", "ruby"}} {
		uri := h.docURI(v.path)
		body, _ := os.ReadFile(filepath.Join(root, v.path))
		send(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "languageId": v.language, "text": string(body)}}})
		send(map[string]any{"jsonrpc": "2.0", "id": 10 + i, "method": "corvint/context", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "task": "change widget value", "limit": 6}})
		m := recv()
		var result struct {
			Schema string
			Core   json.RawMessage
		}
		if m["error"] != nil || json.Unmarshal(m["result"], &result) != nil || result.Schema != "corvint-editor-context/0" {
			t.Fatalf("%s context: %s", v.path, m)
		}
		var core struct {
			Repository struct{ CommitRevision string } `json:"repository"`
		}
		if json.Unmarshal(result.Core, &core) != nil || core.Repository.CommitRevision != head {
			t.Fatalf("%s core not bound to HEAD: %.300s", v.path, result.Core)
		}
		input, _ := json.Marshal(contextWorkerInput{Subject: v.path, Task: "change widget value", Limit: 6})
		expected, failure := registry.Call(context.Background(), bridge.ToolContext, input)
		if failure != nil {
			t.Fatal(failure)
		}
		object, failure := expected.Object()
		if failure != nil {
			t.Fatal(failure)
		}
		want, _ := json.Marshal(object)
		var got bytes.Buffer
		json.Compact(&got, result.Core)
		if got.String() != string(want) {
			t.Fatalf("%s: Core object changed by the wrapped path", v.path)
		}
		// Corvint's own non-Go evidence keeps its syntax-class labels, and no
		// wrapped-server observation or server identity enters Core evidence.
		if !bytes.Contains(result.Core, []byte(`"authority":"syntax"`)) || bytes.Contains(result.Core, []byte(wrappedObservationClass)) || bytes.Contains(result.Core, []byte("fake-")) {
			t.Fatalf("%s: evidence labels changed: %.400s", v.path, result.Core)
		}
	}
	if status := fixtureGit(t, root, "status", "--porcelain"); len(status) != 0 {
		t.Fatalf("fixture worktree changed: %s", status)
	}
	// Moving HEAD after initialize leaves the bound revision: the guard refuses
	// context instead of answering from the new commit.
	fixtureGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "Moved")
	send(map[string]any{"jsonrpc": "2.0", "id": 19, "method": "corvint/context", "params": map[string]any{"textDocument": map[string]any{"uri": h.docURI("web/app.ts")}, "task": "change widget value", "limit": 6}})
	if m := recv(); failureOf(m).Code != -32801 || failureOf(m).Data.Reason != "REPOSITORY_CHANGED" {
		t.Fatalf("context after HEAD moved: %s", m)
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 20, "method": "shutdown"})
	if m := recv(); m["error"] != nil {
		t.Fatal(string(m["error"]))
	}
	send(map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if e := c.Wait(); e != nil {
		t.Fatal(e, stderr.String())
	}
}
