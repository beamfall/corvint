package lspprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// module is a temporary committed Go module: a calls b, b calls c, and d
// calls a, so a seed at a reaches b and d at hop one and c at hop two.
type module struct{ root, head string }

func newModule(t *testing.T) module {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/m\n\ngo 1.22\n",
		"a/a.go": "package a\n\nimport \"example.test/m/b\"\n\n// A calls B.\nfunc A() int { return b.B() }\n",
		"b/b.go": "package b\n\nimport \"example.test/m/c\"\n\n// B calls C.\nfunc B() int { return c.C() }\n",
		"c/c.go": "package c\n\n// C is the leaf.\nfunc C() int { return 1 }\n",
		"d/d.go": "package d\n\nimport \"example.test/m/a\"\n\n// D calls A.\nfunc D() int { return a.A() }\n",
		"README": "not go\n",
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
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "first"}} {
		gitOutput(t, root, args...)
	}
	return module{root: root, head: gitOutput(t, root, "rev-parse", "HEAD")}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (m module) request(t *testing.T, executable string, seeds ...string) Request {
	return Request{Root: m.root, Revision: m.head, Seeds: seeds, Executable: executable, Committed: func(path string) (string, string, bool) {
		text, err := exec.Command("git", "-C", m.root, "show", m.head+":"+path).Output()
		if err != nil {
			return "", "", false
		}
		blob, err := exec.Command("git", "-C", m.root, "rev-parse", m.head+":"+path).Output()
		return string(text), strings.TrimSpace(string(blob)), err == nil
	}}
}

func TestTargets(t *testing.T) {
	t.Parallel()
	text := "package p\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc init() {}\n\nfunc Run() { s := \"é\"; fmt.Println(s); helper(); fmt.Println(s) }\n\nfunc helper() {}\n"
	got := targets(text, 8)
	want := []target{
		{queryReferences, "T", 4, 5},
		{queryReferences, "Run", 8, 5},
		{queryReferences, "helper", 10, 5},
		{queryDefinition, "fmt.Println", 8, 27},
		{queryDefinition, "helper", 8, 39},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	if len(targets(text, 1)) != 2 {
		t.Fatal("perKind bounds each kind")
	}
}

// TestSessionAnswersServerRequests: the client answers a server request that
// arrives before the response, and skips notifications.
func TestSessionAnswersServerRequests(t *testing.T) {
	t.Parallel()
	toServer, clientWriter := io.Pipe()
	clientReader, fromServer := io.Pipe()
	s := &session{reader: bufio.NewReader(clientReader), writer: clientWriter, root: "/r"}
	go func() {
		server := &session{reader: bufio.NewReader(toServer), writer: fromServer}
		request, _ := server.read()
		_ = server.send(map[string]any{"jsonrpc": "2.0", "method": "window/logMessage", "params": map[string]any{}})
		_ = server.send(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "workspace/configuration", "params": map[string]any{"items": []any{1, 2}}})
		answer, _ := server.read()
		_ = server.send(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": answer.Result})
	}()
	result, err := s.call("initialize", nil)
	if err != nil || string(result) != "[null,null]" {
		t.Fatalf("result %s, %v", result, err)
	}
}

// TestExpandDegrades: no applicable seed, no executable, and a server that
// fails the session each yield a failure reason and no record (EEP-V0-026).
func TestExpandDegrades(t *testing.T) {
	t.Parallel()
	m := newModule(t)
	failing := filepath.Join(t.TempDir(), "gopls")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		request Request
		reason  string
	}{
		"no go seed":    {m.request(t, failing, "README", "missing.go"), "not applicable"},
		"no executable": {m.request(t, "", "a/a.go"), "gopls executable not found"},
		"failing":       {m.request(t, failing, "a/a.go"), "gopls session failed"},
	}
	for name, c := range cases {
		result := Expand(context.Background(), c.request)
		if result.Record != nil || !strings.HasPrefix(result.Failure, c.reason) {
			t.Errorf("%s: failure %q, record %s", name, result.Failure, result.Record)
		}
		if result.Query["sha256"] == "" {
			t.Errorf("%s: the query digest is reported even on failure", name)
		}
	}
}

// TestExpandLiveGopls: gopls expands one and two hops from a seed, every row
// names its hop origin and is pinned to committed blobs, the record respects
// its bounds, and a modified file is omitted rather than trusted
// (EEP-V0-024, EEP-V0-025). Skipped when gopls is not installed.
func TestExpandLiveGopls(t *testing.T) {
	executable, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("gopls not installed")
	}
	m := newModule(t)
	result := Expand(context.Background(), m.request(t, executable, "a/a.go"))
	if result.Failure != "" {
		t.Fatalf("failure: %s", result.Failure)
	}
	if len(result.Record) > MaxRecordBytes {
		t.Fatalf("record is %d bytes", len(result.Record))
	}
	var record struct {
		Schema    string
		Provider  struct{ ID, Revision string }
		Relations []struct {
			From, To              struct{ Path, Blob string }
			Type, Rule, Reference string
		}
	}
	if err := json.Unmarshal(result.Record, &record); err != nil {
		t.Fatal(err)
	}
	if record.Schema != Schema || record.Provider.ID != ProviderID || record.Provider.Revision == "unknown" {
		t.Fatalf("record identity = %s %+v", record.Schema, record.Provider)
	}
	edges := map[string]string{}
	for _, relation := range record.Relations {
		if relation.From.Blob == "" || relation.To.Blob == "" {
			t.Errorf("unpinned row: %+v", relation)
		}
		edges[relation.From.Path+">"+relation.To.Path+" "+relation.Type] = relation.Reference
	}
	digest := "query sha256:" + result.Query["sha256"].(string) + "; "
	want := map[string]string{
		"a/a.go>b/b.go " + typeUsesDefinition: digest + "hop 1 from seed a/a.go",
		"a/a.go>d/d.go " + typeReferencedBy:   digest + "hop 1 from seed a/a.go",
		"b/b.go>c/c.go " + typeUsesDefinition: digest + "hop 2 from seed a/a.go via b/b.go",
	}
	for edge, reference := range want {
		if edges[edge] != reference {
			t.Errorf("edge %s: reference %q, want %q (edges %v)", edge, edges[edge], reference, edges)
		}
	}
	if err := os.WriteFile(filepath.Join(m.root, "c", "c.go"), []byte("package c\n\n// C changed.\nfunc C() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modified := Expand(context.Background(), m.request(t, executable, "a/a.go"))
	if strings.Contains(string(modified.Record), `"c/c.go"`) || modified.Query["omitted_rows"].(int) == 0 {
		t.Fatalf("a modified file must be omitted, not trusted: %s", modified.Record)
	}
}

// TestMain turns the test binary into a fake language server when a wrapper
// script invokes it with LSPPROVIDER_FAKE set: it answers initialize with the
// given serverInfo name and every definition or reference query with an
// error, as gopls does for a package it cannot load.
func TestMain(m *testing.M) {
	if name := os.Getenv("LSPPROVIDER_FAKE"); name != "" {
		fakeServer(name)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeServer(name string) {
	s := &session{reader: bufio.NewReader(os.Stdin), writer: os.Stdout}
	for {
		request, err := s.read()
		if err != nil || request.Method == "exit" {
			return
		}
		if len(request.ID) == 0 {
			continue
		}
		reply := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch {
		case request.Method == "initialize":
			reply["result"] = map[string]any{"serverInfo": map[string]any{"name": name, "version": "v0.0.0-fake"}}
		case strings.HasPrefix(request.Method, "textDocument/"):
			reply["error"] = map[string]any{"code": 0, "message": "no package metadata for file\nfake"}
		default:
			reply["result"] = nil
		}
		_ = s.send(reply)
	}
}

// fakeGopls is an absolute executable that runs this test binary as the
// fake server under the given serverInfo name.
func fakeGopls(t *testing.T, name string) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gopls")
	script := fmt.Sprintf("#!/bin/sh\nLSPPROVIDER_FAKE=%s exec %q \"$@\"\n", name, binary)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExpandEveryQueryFailedIsUnavailable: a server that answers every query
// with an error is not a loaded provider with zero relations but a failure
// naming the first error, with the failed queries counted (EEP-V0-026); a
// server that is not gopls is refused (EEP-V0-024).
func TestExpandEveryQueryFailedIsUnavailable(t *testing.T) {
	t.Parallel()
	m := newModule(t)
	result := Expand(context.Background(), m.request(t, fakeGopls(t, "gopls"), "a/a.go"))
	issued, _ := result.Query["queries_issued"].(int)
	if result.Record != nil || issued == 0 || result.Query["failed_queries"] != issued {
		t.Fatalf("record %s, query %v", result.Record, result.Query)
	}
	want := fmt.Sprintf("gopls answered all %d queries with an error; first: textDocument/", issued)
	if !strings.HasPrefix(result.Failure, want) || !strings.Contains(result.Failure, "no package metadata for file fake") {
		t.Fatalf("failure %q", result.Failure)
	}
	foreign := Expand(context.Background(), m.request(t, fakeGopls(t, "other-server"), "a/a.go"))
	if foreign.Record != nil || foreign.Failure != "language server identified as other-server, not gopls; refused" {
		t.Fatalf("foreign: failure %q, record %s", foreign.Failure, foreign.Record)
	}
}
