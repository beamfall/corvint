package lspprovider

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	capabilities := map[string]any{"definitionProvider": true, "referencesProvider": map[string]any{}}
	if raw := os.Getenv("LSPPROVIDER_FAKE_CAPABILITIES"); raw != "" {
		capabilities = nil
		_ = json.Unmarshal([]byte(raw), &capabilities)
	}
	for {
		request, err := s.read()
		if err != nil || request.Method == "exit" {
			return
		}
		if log := os.Getenv("LSPPROVIDER_FAKE_LOG"); log != "" {
			if file, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
				_, _ = fmt.Fprintln(file, request.Method)
				_ = file.Close()
			}
		}
		if len(request.ID) == 0 {
			continue
		}
		reply := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch {
		case request.Method == "initialize":
			reply["result"] = map[string]any{"capabilities": capabilities, "serverInfo": map[string]any{"name": name, "version": "v0.0.0-fake"}}
		case strings.HasPrefix(request.Method, "textDocument/") && os.Getenv("LSPPROVIDER_FAKE_CRASH") != "":
			// The server dies mid-session, after a successful initialize.
			os.Exit(0)
		case strings.HasPrefix(request.Method, "textDocument/"):
			// Real gopls names the document by its absolute file URI.
			var params struct {
				TextDocument struct{ URI string } `json:"textDocument"`
			}
			_ = json.Unmarshal(request.Params, &params)
			reply["error"] = map[string]any{"code": 0, "message": "no package metadata for file\n" + params.TextDocument.URI + "\nprivate file:///Users/another/account.go token=s3cr3t"}
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
	return fakeGoplsWith(t, name, "", "")
}

// fakeGoplsWith also sets the initialize capabilities (JSON, default both
// providers and no encoding) and a log file receiving each request method.
func fakeGoplsWith(t *testing.T, name, capabilities, log string) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gopls")
	script := fmt.Sprintf("#!/bin/sh\nLSPPROVIDER_FAKE=%s LSPPROVIDER_FAKE_CAPABILITIES='%s' LSPPROVIDER_FAKE_LOG=%q exec %q \"$@\"\n", name, capabilities, log, binary)
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
	// V1-0167: the reason names the document repository-relative, never by
	// the absolute URI gopls printed, so no home path enters the packet.
	if !strings.HasPrefix(result.Failure, want) || !strings.HasSuffix(result.Failure, "no package metadata for file a/a.go") || strings.Contains(result.Failure, "file://") || strings.Contains(result.Failure, "/Users/another") || strings.Contains(result.Failure, "s3cr3t") {
		t.Fatalf("failure %q", result.Failure)
	}
	second := newModule(t)
	otherRoot := Expand(context.Background(), second.request(t, fakeGopls(t, "gopls"), "a/a.go"))
	if otherRoot.Failure != result.Failure {
		t.Fatalf("machine-specific failure: %q versus %q", result.Failure, otherRoot.Failure)
	}
	for _, name := range []string{"other-server", "file:///Users/another/account.go"} {
		foreign := Expand(context.Background(), m.request(t, fakeGopls(t, name), "a/a.go"))
		if foreign.Record != nil || foreign.Failure != "language server did not identify as gopls; refused" {
			t.Fatalf("foreign %q: failure %q, record %s", name, foreign.Failure, foreign.Record)
		}
	}
}

func TestSafeQueryErrorWithholdsUnknownServerText(t *testing.T) {
	got := safeQueryError(queryDefinition, "a/a.go", fmt.Errorf("private file:///Users/another/account.go token=s3cr3t"))
	want := "textDocument/definition: server error for file a/a.go"
	if got != want {
		t.Fatalf("failure %q, want %q", got, want)
	}
}

// TestRootCommitIgnoresReplaceRefs: a replace ref that grafts a foreign root
// under the revision does not change the root commit the record is anchored
// on, because the read runs under Core's sanitized Git environment
// (EEP-V0-024, V1-0362).
func TestRootCommitIgnoresReplaceRefs(t *testing.T) {
	t.Parallel()
	m := newModule(t)
	foreign := gitOutput(t, m.root, "commit-tree", m.head+"^{tree}", "-m", "foreign root")
	gitOutput(t, m.root, "replace", "--graft", m.head, foreign)
	if replaced := gitOutput(t, m.root, "rev-list", "--max-parents=0", m.head); replaced != foreign {
		t.Fatalf("fixture replace ref not in effect: root %q, want %q", replaced, foreign)
	}
	origin, err := rootCommit(context.Background(), m.root, m.head)
	if err != nil || origin != m.head {
		t.Fatalf("root commit %q, %v; want %q", origin, err, m.head)
	}
}

// TestRecordExplainsOmissions_EEPV0820 binds EEP-V0-820 (V1-0482): every
// omitted relation is counted by its cause, the causes sum to omitted_rows,
// and a repeated answer is not an omission. The byte bound is reached with
// paths of about 600 bytes and clipped long symbols, so every absolute path
// stays under macOS's 1024-byte limit.
func TestRecordExplainsOmissions_EEPV0820(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		dir, symbol                  string
		unpinned, rowBound, byteMore bool
	}{
		{name: "row bound and unpinned", dir: "p/", unpinned: true, rowBound: true},
		{name: "byte bound", dir: strings.Repeat("deep/", 120), symbol: strings.Repeat("S", 600), byteMore: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			committed := map[string]string{}
			write := func(path, text string) {
				full := filepath.Join(root, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
				committed[path] = text
			}
			seed := tc.dir + "seed.go"
			write(seed, "package seed\n")
			var links []link
			for i := 0; i < MaxRows+2; i++ {
				path := fmt.Sprintf("%sf%02d.go", tc.dir, i)
				if full := filepath.Join(root, filepath.FromSlash(path)); len(full) >= 1024 {
					t.Fatalf("path %d bytes, over the macOS limit", len(full))
				}
				write(path, "package p\n")
				links = append(links, link{from: seed, to: path, method: "textDocument/definition", symbol: tc.symbol, hop: 1, seed: seed})
			}
			links = append(links, links[0]) // a repeated answer
			if tc.unpinned {
				write("modified.go", "package p\n")
				if err := os.WriteFile(filepath.Join(root, "modified.go"), []byte("package p // edited\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				links = append(links, link{from: seed, to: "modified.go", method: "textDocument/definition", hop: 1, seed: seed}, link{from: seed, to: "untracked.go", method: "textDocument/references", hop: 1, seed: seed})
			}
			request := Request{Root: root, Revision: strings.Repeat("a", 40), Committed: func(path string) (string, string, bool) {
				text, ok := committed[path]
				return text, strings.Repeat("b", 40), ok
			}}
			query := querySummary(request, []string{seed})
			encoded, _ := record(request, strings.Repeat("c", 40), expansion{links: links}, query)
			var document struct{ Relations []any }
			if err := json.Unmarshal(encoded, &document); err != nil || len(encoded) > MaxRecordBytes {
				t.Fatalf("record %d bytes: %v", len(encoded), err)
			}
			unpinned, rows, bytes := query["omitted_unpinned"].(int), query["omitted_row_bound"].(int), query["omitted_byte_bound"].(int)
			if query["omitted_rows"].(int) != unpinned+rows+bytes {
				t.Fatalf("causes do not sum: %v", query)
			}
			if tc.unpinned && (unpinned != 2 || rows != 2 || bytes != 0 || len(document.Relations) != MaxRows) {
				t.Fatalf("query %v, relations %d", query, len(document.Relations))
			}
			if tc.byteMore && (unpinned != 0 || rows != 2 || bytes == 0 || len(document.Relations)+bytes != MaxRows) {
				t.Fatalf("query %v, relations %d", query, len(document.Relations))
			}
		})
	}
}

// TestExpandNegotiatesBeforeQuerying_EEPV0821 binds EEP-V0-821 (V1-0479): the
// provider offers only UTF-16, refuses another encoding or a server with no
// admitted method before any query, queries only advertised methods, and
// reports the negotiation and the executable digest in the query summary.
func TestExpandNegotiatesBeforeQuerying_EEPV0821(t *testing.T) {
	t.Parallel()
	m := newModule(t)
	cases := []struct {
		name, capabilities, failure string
		methods, unavailable        []string
	}{
		{"default encoding, both providers", "", "", []string{queryDefinition, queryReferences}, []string{}},
		{"explicit utf-16, definition only", `{"positionEncoding":"utf-16","definitionProvider":true,"referencesProvider":false}`, "", []string{queryDefinition}, []string{queryReferences}},
		{"references options only", `{"referencesProvider":{"workDoneProgress":false}}`, "", []string{queryReferences}, []string{queryDefinition}},
		{"utf-8 refused", `{"positionEncoding":"utf-8","definitionProvider":true,"referencesProvider":true}`, "gopls negotiated an unsupported position encoding; refused", nil, nil},
		{"null encoding refused", `{"positionEncoding":null,"definitionProvider":true}`, "gopls negotiated an unsupported position encoding; refused", nil, nil},
		{"no providers refused", `{"definitionProvider":null,"referencesProvider":"yes"}`, "gopls advertised neither definition nor references; refused", []string{}, []string{queryDefinition, queryReferences}},
	}
	for _, c := range cases {
		log := filepath.Join(t.TempDir(), "methods.log")
		executable := fakeGoplsWith(t, "gopls", c.capabilities, log)
		bytes, err := os.ReadFile(executable)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(bytes)
		result := Expand(context.Background(), m.request(t, executable, "a/a.go"))
		if result.Query["executable_sha256"] != hex.EncodeToString(digest[:]) {
			t.Errorf("%s: executable digest %v", c.name, result.Query["executable_sha256"])
		}
		raw, _ := os.ReadFile(log)
		sent := strings.Fields(string(raw))
		var queried []string
		for _, method := range sent {
			if strings.HasPrefix(method, "textDocument/") {
				queried = append(queried, method)
			}
		}
		if len(sent) == 0 || sent[0] != "initialize" {
			t.Fatalf("%s: methods sent %v", c.name, sent)
		}
		if c.failure != "" {
			if result.Failure != c.failure || result.Record != nil || len(queried) != 0 {
				t.Errorf("%s: failure %q, record %s, queried %v", c.name, result.Failure, result.Record, queried)
			}
			if c.methods == nil {
				if _, reported := result.Query["position_encoding"]; reported {
					t.Errorf("%s: a refused encoding is reported as negotiated: %v", c.name, result.Query)
				}
				continue
			}
		}
		if result.Query["position_encoding"] != "utf-16" || !slices.Equal(result.Query["methods"].([]string), c.methods) || !slices.Equal(result.Query["unavailable_methods"].([]string), c.unavailable) {
			t.Errorf("%s: summary %v", c.name, result.Query)
		}
		for _, method := range queried {
			if !slices.Contains(c.methods, method) {
				t.Errorf("%s: unadvertised %s queried", c.name, method)
			}
		}
		if c.failure == "" && len(queried) == 0 {
			t.Errorf("%s: no admitted query was sent", c.name)
		}
	}
	// A server that dies on its first query after negotiating fails the
	// session but still reports what it negotiated.
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	crashing := filepath.Join(t.TempDir(), "gopls")
	script := fmt.Sprintf("#!/bin/sh\nLSPPROVIDER_FAKE=gopls LSPPROVIDER_FAKE_CAPABILITIES='{\"definitionProvider\":true}' LSPPROVIDER_FAKE_CRASH=1 exec %q \"$@\"\n", binary)
	if err := os.WriteFile(crashing, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	crashed := Expand(context.Background(), m.request(t, crashing, "a/a.go"))
	if crashed.Failure != "gopls session failed" || crashed.Record != nil {
		t.Fatalf("mid-session crash: failure %q, record %s", crashed.Failure, crashed.Record)
	}
	if crashed.Query["position_encoding"] != "utf-16" || !slices.Equal(crashed.Query["methods"].([]string), []string{queryDefinition}) || !slices.Equal(crashed.Query["unavailable_methods"].([]string), []string{queryReferences}) {
		t.Fatalf("mid-session crash dropped the negotiated summary: %v", crashed.Query)
	}
	unreadable := Expand(context.Background(), m.request(t, t.TempDir(), "a/a.go"))
	if unreadable.Failure != "gopls executable unreadable" || unreadable.Record != nil {
		t.Fatalf("unreadable executable: failure %q", unreadable.Failure)
	}
}
