// SPDX-License-Identifier: Apache-2.0
package lspstdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/goplsclient"
	"github.com/Beamfall/corvint/internal/lspsnapshot"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPositionConversions(t *testing.T) {
	text := "α😀x\r\nz"
	for _, v := range []struct {
		enc string
		n   int
	}{{"utf-8", 6}, {"utf-16", 3}, {"utf-32", 2}} {
		p, e := convertPosition(text, goplsclient.Position{Line: 0, Character: v.n}, v.enc, "utf-16")
		if e != nil || p.Character != 3 {
			t.Fatalf("%s: %+v %v", v.enc, p, e)
		}
		p, e = convertPosition(text, p, "utf-16", v.enc)
		if e != nil || p.Character != v.n {
			t.Fatal(p, e)
		}
	}
	for _, v := range []struct {
		enc string
		n   int
	}{{"utf-8", 1}, {"utf-8", 3}, {"utf-16", 2}, {"utf-32", 4}} {
		if _, e := convertPosition(text, goplsclient.Position{Character: v.n}, v.enc, "utf-16"); e == nil {
			t.Fatal("accepted invalid boundary", v)
		}
	}
}
func TestCapturedIdentityReopen(t *testing.T) {
	s, _ := lspsnapshot.NewSession("s", "file:///repo", lspsnapshot.Bounds{Documents: 2, DocumentBytes: 100, TotalBytes: 200})
	store := &overlayStore{session: s, docs: map[string]captured{}}
	uri := "file:///repo/a.go"
	if e := store.put(uri, 1, "x", true); e != nil {
		t.Fatal(e)
	}
	old, _ := store.snapshot(uri)
	store.close(uri)
	store.put(uri, 1, "x", true)
	now, _ := store.snapshot(uri)
	if old.Identity == now.Identity || store.current(old) || !store.current(now) {
		t.Fatal("reopen aliased capture")
	}
}
func TestSemanticHost(t *testing.T) {
	mode := os.Getenv("CORVINT_SEMANTIC_FAKE")
	if mode == "" {
		return
	}
	r := bufio.NewReader(os.Stdin)
	uri := ""
	for {
		b, e := readFrame(r)
		if e != nil {
			os.Exit(0)
		}
		var m struct {
			ID     json.RawMessage
			Method string
			Params json.RawMessage
		}
		json.Unmarshal(b, &m)
		switch m.Method {
		case "initialize":
			name := os.Getenv("CORVINT_SEMANTIC_NAME")
			os.WriteFile(filepath.Join(os.Getenv("CORVINT_SEMANTIC_SCRATCH"), "backend-pid"+name), []byte(strconv.Itoa(os.Getpid())), 0600)
			if mode == "init-hang" {
				time.Sleep(time.Minute)
				continue
			}
			if name != "" {
				// LWB-V0 wrapped fake: its own serverInfo and the argv it was given.
				os.WriteFile(filepath.Join(os.Getenv("CORVINT_SEMANTIC_SCRATCH"), "argv"+name), []byte(strings.Join(os.Args[slices.Index(os.Args, "--")+1:], " ")), 0600)
				reply(os.Stdout, m.ID, map[string]any{"serverInfo": map[string]string{"name": name, "version": "0.0.1"}, "capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": 2}}, 0, "")
				continue
			}
			reply(os.Stdout, m.ID, map[string]any{"serverInfo": map[string]string{"name": "gopls", "version": "v0.23.0"}, "capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": map[string]any{"openClose": true, "change": 1}}}, 0, "")
		case "shutdown":
			reply(os.Stdout, m.ID, nil, 0, "")
		case "exit":
			os.Exit(0)
		case "textDocument/didOpen", "textDocument/didChange", "textDocument/didClose":
			if mode == "diagnostics" {
				publishFakeDiagnostics(m.Params)
			}
			if m.Method != "textDocument/didOpen" {
				continue
			}
			var p struct {
				TextDocument struct{ URI, LanguageID string }
			}
			json.Unmarshal(m.Params, &p)
			uri = p.TextDocument.URI
			if name := os.Getenv("CORVINT_SEMANTIC_NAME"); name != "" {
				f, _ := os.OpenFile(filepath.Join(os.Getenv("CORVINT_SEMANTIC_SCRATCH"), "languages"+name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
				fmt.Fprintln(f, p.TextDocument.LanguageID)
				f.Close()
			}
		case "textDocument/definition":
			if mode == "guard-barrier" {
				dir := os.Getenv("CORVINT_SEMANTIC_SCRATCH")
				os.WriteFile(filepath.Join(dir, "received"), []byte("1"), 0600)
				f, e := os.Open(filepath.Join(dir, "release"))
				if e != nil {
					os.Exit(3)
				}
				var b [1]byte
				f.Read(b[:])
				f.Close()
			}
			if mode == "hang" {
				continue
			}
			if mode == "crash" {
				os.Exit(7)
			}
			if mode == "diagnostics" {
				publishFakeDiagnostics(m.Params)
			}
			reply(os.Stdout, m.ID, []any{map[string]any{"uri": uri, "range": goplsclient.Range{Start: goplsclient.Position{Line: 1, Character: 11}, End: goplsclient.Position{Line: 1, Character: 12}}}}, 0, "")
		}
	}
}

type observedSemanticWriter struct {
	*io.PipeWriter
	writes chan struct{}
}

func (w observedSemanticWriter) Write(p []byte) (int, error) {
	w.writes <- struct{}{}
	return w.PipeWriter.Write(p)
}

type semanticHarness struct {
	writes    chan struct{}
	in        *io.PipeWriter
	out       *io.PipeReader
	reader    *bufio.Reader
	done      chan error
	cancel    context.CancelFunc
	root, uri string
	scratch   string
}

func semanticHarnessNew(t *testing.T, mode string) *semanticHarness {
	return semanticHarnessOptions(t, mode, false)
}
func semanticHarnessOptions(t *testing.T, mode string, guard bool) *semanticHarness {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	exe, _ := os.Executable()
	if strings.Contains(exe, "'") {
		t.Fatal("quote")
	}
	scratch := root
	if guard {
		relaxObservationBudget(t)
		scratch, _ = filepath.EvalSymlinks(t.TempDir())
		fixtureGit(t, root, "init", "-q")
		fixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture")
	}
	binary := filepath.Join(scratch, "gopls")
	if e = os.WriteFile(binary, []byte("#!/bin/sh\nexport CORVINT_SEMANTIC_FAKE="+mode+"\nexport CORVINT_SEMANTIC_SCRATCH='"+scratch+"'\nexec '"+exe+"' -test.run '^TestSemanticHost$' -- serve\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if mode == "live" {
		binary = os.Getenv("CORVINT_TEST_GOPLS")
		if binary == "" {
			t.Skip("explicit CORVINT_TEST_GOPLS required")
		}
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/overlay\ngo 1.27.1\n"), 0600)
		os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nvar disk int\n"), 0600)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	h := &semanticHarness{in: inW, out: outR, reader: bufio.NewReader(outR), done: make(chan error, 1), writes: make(chan struct{}, 16), cancel: cancel, root: root, scratch: scratch}
	h.uri = (&url.URL{Scheme: "file", Path: filepath.Join(root, "main.go")}).String()
	go func() {
		h.done <- ServeSemantic(ctx, inR, observedSemanticWriter{outW, h.writes}, SemanticConfig{Executable: binary, Root: root, WorkspaceDriftGuard: guard})
	}()
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

// relaxObservationBudget raises the shared LED-V0-003 observation budget for one guard harness
// test. The 200 ms product budget is load-sensitive: on a loaded CI host the pre-dispatch check
// overran it, latched stale and answered the definition without contacting the backend, so a
// barrier test waited on a FIFO the backend never opened (V1-2671, GH #1766). Tests that assert
// over-budget behavior call guard.check with their own budget instead.
func relaxObservationBudget(t *testing.T) {
	t.Helper()
	previous := workspaceObservationBudget
	workspaceObservationBudget = 30 * time.Second
	t.Cleanup(func() { workspaceObservationBudget = previous })
}

// bound is a whole-test deadline. On expiry it reports every goroutine, then cancels the session
// and closes both harness pipes so a blocked send, recv or barrier handshake fails at once
// instead of holding the package to its 30 m timeout (V1-2671).
func (h *semanticHarness) bound(t *testing.T, d time.Duration) {
	t.Helper()
	timer := time.AfterFunc(d, func() {
		buf := make([]byte, 1<<20)
		t.Errorf("test exceeded its %s deadline; goroutines:\n%s", d, buf[:runtime.Stack(buf, true)])
		h.cancel()
		h.in.Close()
		h.out.Close()
	})
	t.Cleanup(func() { timer.Stop() })
}
func (h *semanticHarness) send(t *testing.T, m any) {
	t.Helper()
	b, _ := json.Marshal(m)
	if _, e := fmt.Fprint(h.in, frame(string(b))); e != nil {
		t.Fatal(e)
	}
}
func (h *semanticHarness) recv(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	b, e := readFrame(h.reader)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(b, &m)
	return m
}
func (h *semanticHarness) init(t *testing.T) {
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": (&url.URL{Scheme: "file", Path: h.root}).String(), "capabilities": map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-8"}}}}})
	m := h.recv(t)
	if m["error"] != nil {
		t.Fatal(string(m["error"]), string(m["result"]))
	}
	var result struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(m["result"], &result); err != nil {
		t.Fatal(err)
	}
	if _, advertised := result.Capabilities["definitionProvider"]; advertised {
		t.Fatal("unqualified standard definition capability advertised")
	}
	assertTestedCapabilitySubset(t, result.Capabilities)
	var experimental map[string]json.RawMessage
	if err := json.Unmarshal(result.Capabilities["experimental"], &experimental); err != nil || string(experimental["corvintDefinitionProbe"]) != "true" {
		t.Fatal("missing explicit development probe marker", err)
	}
	var marker map[string]string
	if e := json.Unmarshal(experimental["corvintContext"], &marker); e != nil || len(marker) != 2 || marker["method"] != "corvint/context" || marker["schema"] != "corvint-editor-context/0" {
		t.Fatal("invalid context discovery marker", e)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
}
func (h *semanticHarness) open(t *testing.T) {
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri, "version": 1, "languageId": "go", "text": "package p\n/*😀*/ var x int\nvar y = x\n"}}})
}
func (h *semanticHarness) definition(t *testing.T) {
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri}, "position": goplsclient.Position{Line: 2, Character: 8}}})
}
func TestSemanticOverlayDefinition(t *testing.T) {
	for _, mode := range []string{"normal", "live"} {
		t.Run(mode, func(t *testing.T) {
			h := semanticHarnessNew(t, mode)
			h.init(t)
			h.open(t)
			h.definition(t)
			m := h.recv(t)
			if m["error"] != nil {
				t.Fatal(string(m["error"]))
			}
			var rows []struct {
				URI   string
				Range goplsclient.Range
			}
			if e := json.Unmarshal(m["result"], &rows); e != nil || len(rows) != 1 || rows[0].URI != h.uri || rows[0].Range.Start.Character != 13 {
				t.Fatalf("%s", m["result"])
			}
		})
	}
}
func TestSemanticCancelAndChange(t *testing.T) {
	for _, method := range []string{"$/cancelRequest", "textDocument/didChange"} {
		t.Run(method, func(t *testing.T) {
			h := semanticHarnessNew(t, "hang")
			h.init(t)
			h.open(t)
			h.definition(t)
			var p any = map[string]any{"id": 2}
			if method != "$/cancelRequest" {
				p = map[string]any{"textDocument": map[string]any{"uri": h.uri, "version": 2}, "contentChanges": []any{map[string]string{"text": "package p\n"}}}
			}
			h.send(t, map[string]any{"jsonrpc": "2.0", "method": method, "params": p})
			m := h.recv(t)
			if !strings.Contains(string(m["error"]), "-32800") {
				t.Fatal(string(m["error"]))
			}
		})
	}
}
func TestSemanticEOFDuringInitialize(t *testing.T) {
	h := semanticHarnessNew(t, "init-hang")
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": (&url.URL{Scheme: "file", Path: h.root}).String()}})
	h.in.Close()
	select {
	case e := <-h.done:
		h.done <- e
	case <-time.After(5 * time.Second):
		t.Fatal("EOF did not retire initialization")
	}
}

func TestSemanticRejectsRootsAndIncrementalChange(t *testing.T) {
	t.Run("multiple roots", func(t *testing.T) {
		h := semanticHarnessNew(t, "normal")
		h.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"workspaceFolders": []any{map[string]string{"uri": "file:///one"}, map[string]string{"uri": "file:///two"}}}})
		m := h.recv(t)
		if !strings.Contains(string(m["error"]), "-32602") {
			t.Fatal(string(m["error"]))
		}
	})
	t.Run("incremental change", func(t *testing.T) {
		h := semanticHarnessNew(t, "normal")
		h.init(t)
		h.open(t)
		h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri, "version": 2}, "contentChanges": []any{map[string]any{"text": "x", "range": goplsclient.Range{}}}}})
		select {
		case e := <-h.done:
			if e == nil {
				t.Fatal("accepted incremental update")
			}
			h.done <- e
		case <-time.After(5 * time.Second):
			t.Fatal("invalid sync did not retire session")
		}
	})
}

func TestSemanticBackendDeathUnblocksReply(t *testing.T) {
	h := semanticHarnessNew(t, "normal")
	h.init(t)
	<-h.writes
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "unsupported"})
	select {
	case <-h.writes:
	case <-time.After(time.Second):
		t.Fatal("reply did not begin")
	}
	// The writer has entered Write, and no reader consumes this response.
	raw, e := os.ReadFile(filepath.Join(h.root, "backend-pid"))
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(string(raw))
	if e != nil {
		t.Fatal(e)
	}
	process, e := os.FindProcess(pid)
	if e != nil {
		t.Fatal(e)
	}
	if e = process.Kill(); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-h.done:
		if e == nil {
			t.Fatal("backend death reported success")
		}
		h.done <- e
	case <-time.After(3 * time.Second):
		t.Fatal("backend death left reply blocked")
	}
}
func TestSemanticShutdownReplyBeforeExit(t *testing.T) {
	h := semanticHarnessNew(t, "normal")
	h.init(t)
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "shutdown"})
	m := h.recv(t)
	if string(m["result"]) != "null" || m["error"] != nil {
		t.Fatalf("shutdown response: %s", m)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	select {
	case e := <-h.done:
		if e != nil {
			t.Fatal(e)
		}
		h.done <- e
	case <-time.After(3 * time.Second):
		t.Fatal("exit did not retire session")
	}
}

// assertTestedCapabilitySubset pins LQP-V0-012 (V1-0481): the companion
// advertises only the tested standard members. Any new standard provider,
// diagnostics included, must first gain transcript tests and be added here.
func assertTestedCapabilitySubset(t *testing.T, capabilities map[string]json.RawMessage) {
	t.Helper()
	for name := range capabilities {
		switch name {
		case "positionEncoding", "textDocumentSync", "experimental":
		default:
			t.Fatalf("untested standard capability %q advertised", name)
		}
	}
	var sync map[string]json.RawMessage
	if err := json.Unmarshal(capabilities["textDocumentSync"], &sync); err != nil {
		t.Fatal("textDocumentSync is not an options object", err)
	}
	for name := range sync {
		switch name {
		case "openClose", "change", "save":
		default:
			t.Fatalf("untested textDocumentSync member %q advertised", name)
		}
	}
	if string(sync["openClose"]) != "true" || string(sync["change"]) != "1" {
		t.Fatalf("textDocumentSync must be full open/close sync: %s", capabilities["textDocumentSync"])
	}
}

// publishFakeDiagnostics makes the fake backend push an upstream diagnostic
// for the document named in params, as real gopls does after each sync.
func publishFakeDiagnostics(params json.RawMessage) {
	var p struct {
		TextDocument struct{ URI string }
	}
	json.Unmarshal(params, &p)
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{map[string]any{"range": goplsclient.Range{}, "message": "upstream diagnostic"}}}})
	fmt.Fprint(os.Stdout, frame(string(b)))
	f, _ := os.OpenFile(filepath.Join(os.Getenv("CORVINT_SEMANTIC_SCRATCH"), "diagnostics-published"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	fmt.Fprintln(f, p.TextDocument.URI)
	f.Close()
}

// TestSemanticWithholdsUpstreamDiagnostics pins the diagnostics clause of
// LQP-V0-012 for the unqualified companion (V1-0481): diagnostics are not a
// qualified method, so across open, change, definition and close the client
// receives no publishDiagnostics, even when the backend pushes them. With
// none published there is no stale diagnostic left to withdraw. The unsaved
// change and the read leave the root's files byte-identical (V1-0481: no read
// writes source).
func TestSemanticWithholdsUpstreamDiagnostics(t *testing.T) {
	h := semanticHarnessNew(t, "diagnostics")
	if err := os.WriteFile(filepath.Join(h.root, "main.go"), []byte("package p\nvar disk int\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeFiles := fixtureContents(t, h.root)
	h.bound(t, time.Minute)
	h.init(t)
	h.open(t)
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri, "version": 2}, "contentChanges": []any{map[string]string{"text": "package p\n/*😀*/ var x int\nvar y = x\n"}}}})
	h.definition(t)
	until := func(id string) map[string]json.RawMessage {
		for {
			m := h.recv(t)
			var method string
			json.Unmarshal(m["method"], &method)
			if strings.Contains(method, "iagnostic") {
				t.Fatalf("companion forwarded %s: %s", method, m["params"])
			}
			if string(m["id"]) == id {
				return m
			}
		}
	}
	if m := until("2"); m["error"] != nil {
		t.Fatalf("definition: %s", m["error"])
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didClose", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri}}})
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "shutdown"})
	if m := until("3"); string(m["result"]) != "null" || m["error"] != nil {
		t.Fatalf("shutdown: %s", m)
	}
	// The backend pushed a diagnostic after open, change, definition and close.
	published, _ := os.ReadFile(filepath.Join(h.scratch, "diagnostics-published"))
	if n := strings.Count(string(published), "\n"); n < 4 {
		t.Fatalf("backend published %d diagnostics, want at least 4", n)
	}
	// The fake backend logs into the shared scratch root; only its logs may be new.
	afterFiles := fixtureContents(t, h.root)
	delete(afterFiles, "diagnostics-published")
	delete(afterFiles, "backend-pid")
	if !reflect.DeepEqual(beforeFiles, afterFiles) {
		t.Fatalf("the session changed a file under the root: before %x after %x", beforeFiles, afterFiles)
	}
}
