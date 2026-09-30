package goplsclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type snapshots struct {
	mu     sync.Mutex
	values map[string]Snapshot
}

func (s *snapshots) get(uri string) (Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[uri]
	return v, ok
}
func (s *snapshots) current(v Snapshot) bool { now, ok := s.get(v.URI); return ok && now == v }
func (s *snapshots) put(v Snapshot)          { s.mu.Lock(); defer s.mu.Unlock(); s.values[v.URI] = v }
func fakeConfig(t *testing.T, mode string) (Config, *snapshots) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "gopls")
	if strings.Contains(bin, "'") {
		t.Fatal("unsupported test binary path")
	}
	body := "#!/bin/sh\nexport GOPLS_TEST_MODE=" + mode + "\nexec '" + bin + "' -test.run '^TestHost$' -- serve\n"
	if err = os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	store := &snapshots{values: map[string]Snapshot{}}
	uri := fileURI(filepath.Join(root, "main.go"))
	store.put(Snapshot{URI: uri, Identity: "overlay-1", Text: "package p\nvar x int\nvar y = x\n", Version: 1, Overlay: true})
	return Config{Executable: script, Root: root, Snapshot: store.get, Current: store.current}, store
}
func startFake(t *testing.T, mode string) (*Client, *snapshots, string) {
	t.Helper()
	cfg, store := fakeConfig(t, mode)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-c.Done():
		case <-time.After(5 * time.Second):
			t.Error("cleanup did not finish")
		}
	})
	return c, store, fileURI(filepath.Join(cfg.Root, "main.go"))
}
func TestHost(t *testing.T) {
	mode := os.Getenv("GOPLS_TEST_MODE")
	if mode == "" {
		return
	}
	r := bufio.NewReader(os.Stdin)
	var uri string
	var pending json.RawMessage
	var child *exec.Cmd
	send := func(id json.RawMessage, result any) {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(b), b)
	}
	for {
		b, err := readFrame(r)
		if err != nil {
			os.Exit(0)
		}
		var req struct {
			ID     json.RawMessage
			Method string
			Params json.RawMessage
		}
		if json.Unmarshal(b, &req) != nil {
			os.Exit(2)
		}
		switch req.Method {
		case "initialize":
			send(req.ID, map[string]any{"serverInfo": map[string]string{"name": "gopls", "version": "v0.23.0"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "definitionProvider": true, "textDocumentSync": map[string]any{"openClose": true, "change": 2}}})
		case "textDocument/didOpen":
			var v struct{ TextDocument struct{ URI string } }
			_ = json.Unmarshal(req.Params, &v)
			uri = v.TextDocument.URI
		case "textDocument/definition":
			switch mode {
			case "cancel":
				if pending == nil {
					pending = req.ID
					continue
				}
			case "crash", "wait-cancel":
				child = exec.Command("/bin/sleep", "120")
				if child.Start() != nil {
					os.Exit(4)
				}
				_ = os.WriteFile("child-pid", []byte(strconv.Itoa(child.Process.Pid)), 0600)
				time.Sleep(150 * time.Millisecond)
				if mode == "wait-cancel" {
					time.Sleep(120 * time.Second)
				}
				os.Exit(1)
			case "secret":
				b := []byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"error":{"code":-1,"message":"/private/secret token=secret"}}`)
				fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(b), b)
				continue
			case "outside":
				uri = "file:///private/secret.go"
			case "missing-range":
				send(req.ID, []any{map[string]any{"uri": uri}})
				continue
			case "bad-range":
				send(req.ID, []any{map[string]any{"uri": uri, "range": Range{Position{0, 999}, Position{0, 999}}}})
				continue
			}
			send(req.ID, []any{map[string]any{"uri": uri, "range": Range{Position{1, 4}, Position{1, 5}}}})
		case "$/cancelRequest":
			if pending != nil {
				send(pending, nil)
			}
		case "shutdown":
			send(req.ID, nil)
		case "exit":
			os.Exit(0)
		}
	}
}

// GCS-V0-001 GCS-V0-002 GCS-V0-003
func TestSessionSyncAndCancellation(t *testing.T) {
	c, store, uri := startFake(t, "cancel")
	if err := c.Open(context.Background(), uri); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := c.Definition(ctx, uri, Position{2, 8}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
	rows, err := c.Definition(context.Background(), uri, Position{2, 8})
	if err != nil || len(rows) != 1 || !rows[0].Snapshot.Overlay {
		t.Fatalf("second request: %v %v", rows, err)
	}
	v, _ := store.get(uri)
	v.Version = 2
	v.Identity = "overlay-2"
	v.Text = "package p\nvar z int\nvar y = z\n"
	store.put(v)
	if _, err = c.Definition(context.Background(), uri, Position{2, 8}); !errors.Is(err, ErrSnapshot) {
		t.Fatal("unsynchronized overlay accepted")
	}
	if err = c.Change(context.Background(), uri); err != nil {
		t.Fatal(err)
	}
	if err = c.Change(context.Background(), uri); !errors.Is(err, ErrSnapshot) {
		t.Fatal("duplicate version accepted")
	}
	if err = c.CloseDocument(context.Background(), uri); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Definition(context.Background(), uri, Position{2, 8}); !errors.Is(err, ErrSnapshot) {
		t.Fatal("closed overlay accepted")
	}
	if err = c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !c.CleanupProven() {
		t.Fatal("cleanup not proven")
	}
}

// GCS-V0-004
func TestHostileResponses(t *testing.T) {
	for _, mode := range []string{"secret", "outside", "bad-range", "missing-range"} {
		t.Run(mode, func(t *testing.T) {
			c, _, uri := startFake(t, mode)
			if err := c.Open(context.Background(), uri); err != nil {
				t.Fatal(err)
			}
			rows, err := c.Definition(context.Background(), uri, Position{2, 8})
			if err == nil || len(rows) != 0 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe response: %v %v", rows, err)
			}
		})
	}
}

// GCS-V0-004
func TestFramingAndUTF16(t *testing.T) {
	for _, wire := range []string{"Content-Length: 1\r\nContent-Length: 1\r\n\r\nx", "Content-Length: 1048577\r\n\r\n", strings.Repeat("x", 8192), "Content-Length: -1\r\n\r\n", "Content-Length: 1\n\nx"} {
		if _, err := readFrame(bufio.NewReaderSize(strings.NewReader(wire), 4096)); err == nil {
			t.Fatal("malformed frame accepted")
		}
	}
	for _, p := range []Position{{0, 1}, {0, 3}, {1, 0}} {
		if !validPosition("a😀z\n", p) {
			t.Fatalf("valid UTF16 position refused: %v", p)
		}
	}
	if validPosition("a😀z", Position{0, 2}) {
		t.Fatal("surrogate split accepted")
	}
}

// GCS-V0-001
func TestNegotiation(t *testing.T) {
	base := `{"serverInfo":{"name":"gopls","version":"v0.23.0"},"capabilities":{"positionEncoding":"utf-16","definitionProvider":true,"textDocumentSync":{"openClose":true,"change":2}}}`
	for _, row := range []struct{ old, new string }{{"utf-16", "utf-8"}, {"\"definitionProvider\":true", "\"definitionProvider\":false"}, {"\"openClose\":true", "\"openClose\":false"}, {"gopls", "other"}, {"v0.23.0", "tokenSECRET"}} {
		c := &Client{}
		if c.negotiate(json.RawMessage(strings.Replace(base, row.old, row.new, 1))) == nil {
			t.Fatalf("unsupported negotiation accepted: %v", row)
		}
	}
}

// GCS-V0-003
func TestStaleReturn(t *testing.T) {
	cfg, store := fakeConfig(t, "normal")
	uri := fileURI(filepath.Join(cfg.Root, "main.go"))
	calls := 0
	cfg.Current = func(s Snapshot) bool { calls++; return calls < 5 && store.current(s) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-c.Done() }()
	if err = c.Open(ctx, uri); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Definition(ctx, uri, Position{2, 8}); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("stale reply escaped: %v", err)
	}
}

// GCS-V0-005. Explicit opt-in uses only the supplied local executable.
func TestLiveOverlay(t *testing.T) {
	executable := os.Getenv("GOPLSCLIENT_LIVE_EXECUTABLE")
	if executable == "" {
		t.Skip("NOT_RUN: explicit local gopls executable absent")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/overlay\n\ngo 1.27.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	disk := "package overlay\nvar Disk = 1\nvar Use = Disk\n"
	if err = os.WriteFile(filepath.Join(root, "main.go"), []byte(disk), 0600); err != nil {
		t.Fatal(err)
	}
	uri := fileURI(filepath.Join(root, "main.go"))
	store := &snapshots{values: map[string]Snapshot{uri: {URI: uri, Identity: "open-v1", Version: 1, Overlay: true, Text: "package overlay\n\nvar Unsaved = 1\nvar Use = Unsaved\n"}}}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c, err := Start(ctx, Config{executable, root, store.get, store.current})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-c.Done() }()
	if err = c.Open(ctx, uri); err != nil {
		t.Fatal(err)
	}
	rows, err := c.Definition(ctx, uri, Position{3, 12})
	if err != nil || len(rows) != 1 || rows[0].Range.Start.Line != 2 || rows[0].Snapshot.Identity != "open-v1" {
		t.Fatalf("overlay definition: %v %v", rows, err)
	}
	v, _ := store.get(uri)
	v.Version = 2
	v.Identity = "open-v2"
	v.Text = "package overlay\n\n\nvar Changed = 1\nvar Use = Changed\n"
	store.put(v)
	if err = c.Change(ctx, uri); err != nil {
		t.Fatal(err)
	}
	rows, err = c.Definition(ctx, uri, Position{4, 12})
	if err != nil || len(rows) != 1 || rows[0].Range.Start.Line != 3 || rows[0].Snapshot.Identity != "open-v2" {
		t.Fatalf("changed definition: %v %v", rows, err)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(actual) != disk {
		t.Fatal("disk changed")
	}
	t.Logf("experimental tuple version=%s digest=%s encoding=%s definitions=2 disk-unchanged=true", c.Profile().ServerVersion, c.Profile().ExecutableSHA256, c.Profile().PositionEncoding)
	if err = c.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
