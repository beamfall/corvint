package console

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBrowserBoundaryBeforeTools(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "invoked")
	binary := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t, binary)
	cases := []struct {
		name, method, host, origin, body string
		status                           int
	}{
		{"foreign host", "GET", "attacker.example", "", "", 403},
		{"wrong port", "GET", "127.0.0.1:1", "", "", 403},
		{"foreign origin", "GET", s.host, "https://example.com", "", 403},
		{"null origin", "GET", s.host, "null", "", 403},
		{"write read route", "POST", s.host, "", "", 405},
		{"missing token", "POST", s.host, "", "verb=hold", 403},
		{"token query insufficient", "POST", s.host, "", "verb=hold", 403},
		{"oversized", "POST", s.host, "", "token=" + s.token + "&verb=hold&payload=" + strings.Repeat("x", 1<<20), 400},
		{"unreviewed", "POST", s.host, "", "token=" + s.token + "&verb=execute", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "/"
			if tc.method == "POST" && tc.name != "write read route" {
				path = "/mutate?token=" + s.token
			}
			r := httptest.NewRequest(tc.method, path, strings.NewReader(tc.body))
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing response protections")
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("rejected request invoked tool")
	}
}

func TestStructuredForms(t *testing.T) {
	help := strings.Replace(helpEnvelope, `"ticket hold"`, `"ticket hold","ticket create","ticket refine"`, 1)
	detail := `{"profile":"taskman-command-result/0","outcome":"OK","items":[{"ticketId":"ticket:acme:main:AT-1","title":"Shown title","revision":"7","record":{"body":"Shown body"}}]}`
	binary := stubTaskman(t, map[string]string{"help ": help, "ticket list": listEnvelope, "ticket show": detail, "queue status": `{"profile":"taskman-command-result/0","outcome":"OK","items":[{"queueId":"queue:acme:main"}]}`, "ticket refine": refusalEnvelope, "ticket create": `{"profile":"taskman-command-result/0","outcome":"OK","items":[{"ticketId":"ticket:acme:main:AT-9"}]}`})
	s := newTestServer(t, binary)
	board := get(t, s, "/")
	if !strings.Contains(board, "Create ticket") || !strings.Contains(board, `name="token"`) {
		t.Fatal("missing create form")
	}
	page := get(t, s, "/ticket?id=ticket:acme:main:AT-1")
	for _, want := range []string{`name="expected" value="7"`, `value="Shown title"`, `>Shown body</textarea>`, `Save title and body`} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %s", want)
		}
	}
	f := &ticketForm{Verb: "create", Title: "Title <&", Body: "Body", Criteria: "one\ntwo"}
	payload, err := (&Taskman{Binary: binary, Repo: s.options.Repo}).formPayload(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatal(err)
	}
	if data["executionClass"] != "MANUAL" || data["title"] != f.Title || len(data["acceptanceCriteria"].([]any)) != 2 {
		t.Fatalf("payload %s", payload)
	}
	post := func(verb, title string) *httptest.ResponseRecorder {
		form := url.Values{"token": {s.token}, "verb": {verb}, "title": {title}, "body": {"retain <body>"}, "requestId": {"r1"}, "issuedAt": {IssuedNow()}}
		if verb == "refine" {
			form.Set("ticket", "ticket:acme:main:AT-1")
			form.Set("expected", "7")
		}
		r := httptest.NewRequest("POST", "/mutate", strings.NewReader(form.Encode()))
		r.Host = s.host
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	refused := post("refine", "Changed")
	if !strings.Contains(refused.Body.String(), `value="Changed"`) || !strings.Contains(refused.Body.String(), "retain &lt;body&gt;") || !strings.Contains(refused.Body.String(), `name="expected" value="7"`) {
		t.Fatal("refusal lost submitted fields or shown revision")
	}
	created := post("create", "New")
	if !strings.Contains(created.Body.String(), `/ticket?id=ticket%3aacme%3amain%3aAT-9`) {
		t.Fatal("no created ticket link")
	}
	invalid := post("refine", "")
	if !strings.Contains(invalid.Body.String(), "Title is required") || !strings.Contains(invalid.Body.String(), "retain &lt;body&gt;") {
		t.Fatal("validation lost entries")
	}
}

func TestBrowserGitReadsDoNotRunFiltersOrRedirect(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = gitEnvironment()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Console Test")
	git("config", "user.email", "console@example.test")
	os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("committed\n"), 0644)
	git("add", "tracked.txt")
	git("commit", "-m", "fixture")
	head := git("rev-parse", "HEAD")
	marker := filepath.Join(t.TempDir(), "filter-ran")
	git("config", "filter.trap.clean", "touch '"+marker+"'; cat")
	git("config", "filter.trap.process", "touch '"+marker+"'; cat")
	os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("tracked.txt filter=trap\n"), 0644)
	os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0644)
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", t.TempDir())
	s, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}
	body := get(t, s, "/code?path=tracked.txt")
	if !strings.Contains(body, "committed") || !strings.Contains(body, head) {
		t.Fatal("ambient Git redirected immutable read")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("GET executed repository filter")
	}
}

func TestATMCanonicalFormEscapes(t *testing.T) {
	f := &ticketForm{Verb: "refine", Title: "Unicode \u2028 separator", Body: "\b\f\u2029 literal \\u2028 \\b <&>"}
	payload, err := (&Taskman{}).formPayload(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"body\":\"\\u0008\\u000c\u2029 literal \\\\u2028 \\\\b <&>\",\"title\":\"Unicode \u2028 separator\"}"
	if payload != want {
		t.Fatalf("canonical payload got %q want %q", payload, want)
	}
}

// TestFormLimitsMatchATM is the review repair for the structured forms: the
// console refuses exactly at ATM's §1 limits and keeps every entry on refusal.
func TestFormLimitsMatchATM(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	valid := &ticketForm{Verb: "create", Title: long(512), Body: long(64 << 10), Criteria: strings.Repeat(long(4<<10)+"\n", 64)}
	if err := valid.validate(); err != nil {
		t.Fatalf("maximal ATM entries refused: %v", err)
	}
	cases := map[string]*ticketForm{
		"title":     {Verb: "create", Title: long(513)},
		"body":      {Verb: "create", Title: "t", Body: long(64<<10 + 1)},
		"count":     {Verb: "create", Title: "t", Criteria: strings.Repeat("c\n", 65)},
		"criterion": {Verb: "create", Title: "t", Criteria: long(4<<10 + 1)},
	}
	for name, form := range cases {
		err := form.validate()
		if err == nil || !strings.Contains(err.Error(), "retained") {
			t.Fatalf("%s over ATM's limit: %v", name, err)
		}
	}
	binary := stubTaskman(t, map[string]string{"help ": strings.Replace(helpEnvelope, `"ticket hold"`, `"ticket hold","ticket create"`, 1)})
	s := newTestServer(t, binary)
	form := url.Values{"token": {s.token}, "verb": {"create"}, "title": {long(513)}, "body": {"kept body"}, "criteria": {"kept criterion"}, "requestId": {"r1"}, "issuedAt": {IssuedNow()}}
	r := httptest.NewRequest("POST", "/mutate", strings.NewReader(form.Encode()))
	r.Host = s.host
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	page := w.Body.String()
	for _, want := range []string{"512 bytes", `value="` + long(513) + `"`, ">kept body</textarea>", ">kept criterion</textarea>"} {
		if !strings.Contains(page, want) {
			t.Fatalf("refusal page lost %q", want[:min(len(want), 40)])
		}
	}
}

func TestConsoleRootMustBeGitToplevel(t *testing.T) {
	root := t.TempDir()
	init := exec.Command("git", "-C", root, "init", "-q")
	init.Env = gitEnvironment()
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	subdirectory := filepath.Join(root, "internal")
	if err := os.Mkdir(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RequireToplevel(context.Background(), root, time.Minute); err != nil {
		t.Fatalf("toplevel refused: %v", err)
	}
	for _, refused := range []string{subdirectory, t.TempDir()} {
		if err := RequireToplevel(context.Background(), refused, time.Minute); err == nil {
			t.Errorf("%s accepted as a repository root", refused)
		}
	}
}
