// SPDX-License-Identifier: AGPL-3.0-or-later
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
			os.WriteFile("backend-pid", []byte(strconv.Itoa(os.Getpid())), 0600)
			if mode == "init-hang" {
				time.Sleep(time.Minute)
				continue
			}
			reply(os.Stdout, m.ID, map[string]any{"serverInfo": map[string]string{"name": "gopls", "version": "v0.23.0"}, "capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": map[string]any{"openClose": true, "change": 1}}}, 0, "")
		case "shutdown":
			reply(os.Stdout, m.ID, nil, 0, "")
		case "exit":
			os.Exit(0)
		case "textDocument/didOpen":
			var p struct{ TextDocument struct{ URI string } }
			json.Unmarshal(m.Params, &p)
			uri = p.TextDocument.URI
		case "textDocument/definition":
			if mode == "hang" {
				continue
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
	reader    *bufio.Reader
	done      chan error
	cancel    context.CancelFunc
	root, uri string
}

func semanticHarnessNew(t *testing.T, mode string) *semanticHarness {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	exe, _ := os.Executable()
	if strings.Contains(exe, "'") {
		t.Fatal("quote")
	}
	binary := filepath.Join(root, "gopls")
	if e = os.WriteFile(binary, []byte("#!/bin/sh\nexport CORVINT_SEMANTIC_FAKE="+mode+"\nexec '"+exe+"' -test.run '^TestSemanticHost$' -- serve\n"), 0700); e != nil {
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
	h := &semanticHarness{in: inW, reader: bufio.NewReader(outR), done: make(chan error, 1), writes: make(chan struct{}, 16), cancel: cancel, root: root}
	h.uri = (&url.URL{Scheme: "file", Path: filepath.Join(root, "main.go")}).String()
	go func() {
		h.done <- ServeSemantic(ctx, inR, observedSemanticWriter{outW, h.writes}, SemanticConfig{Executable: binary, Root: root})
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
	if m["error"] != nil || !strings.Contains(string(m["result"]), `"definitionProvider":true`) {
		t.Fatal(string(m["error"]), string(m["result"]))
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
