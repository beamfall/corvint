package changee2e

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

const (
	testServeArg  = "corvint-e2e-test-serve"
	testServeAddr = "CORVINT_E2E_TEST_SERVE"
	testServeLeak = "CORVINT_E2E_TEST_LEAK"
)

// testServe is the serve argv the environment tests declare: the test binary serves its working
// directory on the address named by a declared variable and reports an undeclared one.
func testServe() {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/corvint-leak", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, os.Getenv(testServeLeak)) })
	_ = http.ListenAndServe(os.Getenv(testServeAddr), mux)
	os.Exit(1)
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func testEnvironment(t *testing.T, addr string) *Environment {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Environment{Serve: []string{bin, testServeArg}, BaseURL: "http://" + addr, Ready: "/index.html", Env: []string{testServeAddr}}
}

func declarationsWith(t *testing.T, env any) string {
	t.Helper()
	var d map[string]any
	if err := json.Unmarshal([]byte(*overlayText(t, DeclarationsPath)), &d); err != nil {
		t.Fatal(err)
	}
	d["environment"] = env
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// CCE-V0-021: a valid environment is carried in the catalog, and every member outside the closed
// shape refuses e2e-blocks-invalid.
func TestCCEV0021EnvironmentShape(t *testing.T) {
	root, _ := fixtureRepo(t)
	good := map[string]any{"serve": []string{"npm", "run", "dev"}, "base_url": "http://127.0.0.1:5173", "ready": "/", "env": []string{"NODE_ENV"}}
	rev := commitFiles(t, root, map[string]*string{DeclarationsPath: text(declarationsWith(t, good))}, "environment")
	c, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Environment == nil || c.Environment.BaseURL != "http://127.0.0.1:5173" || !strings.Contains(string(c.Encoded()), `"environment"`) {
		t.Fatalf("environment %+v", c.Environment)
	}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range good {
			m[kk] = vv
		}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for name, env := range map[string]map[string]any{
		"remote host":    with("base_url", "http://example.com:80"),
		"no port":        with("base_url", "http://127.0.0.1"),
		"https":          with("base_url", "https://127.0.0.1:8443"),
		"base path":      with("base_url", "http://127.0.0.1:5173/app"),
		"empty serve":    with("serve", []string{}),
		"empty arg":      with("serve", []string{"npm", ""}),
		"relative ready": with("ready", "health"),
		"env value":      with("env", []string{"NODE_ENV=production"}),
		"duplicate env":  with("env", []string{"A", "A"}),
		"base url name":  with("env", []string{envBaseURL}),
		"missing ready":  with("ready", nil),
		"unknown member": with("cwd", "app"),
	} {
		rev := commitFiles(t, root, map[string]*string{DeclarationsPath: text(declarationsWith(t, env))}, name)
		if _, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"}); refusalCode(err) != "e2e-blocks-invalid" {
			t.Fatalf("%s: got %v", name, err)
		}
	}
	if (*Environment)(nil).digest() != environmentStatic || c.Environment.digest() == environmentStatic {
		t.Fatal("environment digest")
	}
}

// CCE-V0-021: serve runs inside the tree without a shell and with only PATH, HOME and the declared
// names; ready gates the run; stop kills the process group.
func TestCCEV0021ServeTree(t *testing.T) {
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "index.html"), []byte("side tree"), 0o644); err != nil {
		t.Fatal(err)
	}
	addr := freeLoopbackAddr(t)
	t.Setenv(testServeAddr, addr)
	t.Setenv(testServeLeak, "leaked")
	env := testEnvironment(t, addr)
	stop, err := serveTree(bg, env, tree)
	if err != nil {
		t.Fatal(err)
	}
	get := func(p string) string {
		resp, err := http.Get(env.origin() + strings.TrimPrefix(p, "/"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := get("/index.html"); got != "side tree" {
		t.Fatalf("served %q, want the side tree", got)
	}
	if got := get("/corvint-leak"); got != "" {
		t.Fatalf("undeclared variable reached serve: %q", got)
	}
	// A second serve on an answering port refuses instead of testing a stray server.
	if _, err := serveTree(bg, env, tree); !isEnvCode(err, "port-in-use") {
		t.Fatalf("second serve: %v", err)
	}
	stop()
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = c.Close()
		t.Fatal("serve still answers after stop")
	}

	missing := &Environment{Serve: []string{"corvint-no-such-serve-tool"}, BaseURL: env.BaseURL, Ready: "/"}
	if _, err := serveTree(bg, missing, tree); !isEnvCode(err, "serve-tool-missing") {
		t.Fatalf("missing tool: %v", err)
	}
	exits := &Environment{Serve: []string{"false"}, BaseURL: env.BaseURL, Ready: "/"}
	if _, err := serveTree(bg, exits, tree); !isEnvCode(err, "serve-exited") {
		t.Fatalf("exiting serve: %v", err)
	}
	// A failed environment is a side without a qualified receipt, so the verdict is INCONCLUSIVE.
	if _, reason := qualify(jstestprovider.Receipt{}, &envError{"ready-timeout"}, 1); reason != "environment-unavailable:ready-timeout" {
		t.Fatalf("qualify reason %q", reason)
	}
	side := sideProof{Reason: "environment-unavailable:ready-timeout"}
	if v, _ := verdict(side, side, nil); v != VerdictInconclusive {
		t.Fatalf("verdict %s", v)
	}
}

func isEnvCode(err error, code string) bool {
	got, ok := asEnvError(err)
	return ok && got == "environment-unavailable:"+code
}

// CCE-V0-021 live: with a declared environment, each side and both controls run against the serve
// argv's server, and the proof records the environment digest instead of "static".
func TestCCEV0021LiveEnvironmentDiscriminates(t *testing.T) {
	root, _ := fixtureRepo(t)
	addr := freeLoopbackAddr(t)
	t.Setenv(testServeAddr, addr)
	env := testEnvironment(t, addr)
	base := commitFiles(t, root, map[string]*string{DeclarationsPath: text(declarationsWith(t, env))}, "environment")
	head := commitBehaviourChange(t, root)
	p, doc := liveProveDoc(t, root, base, head)
	if doc.Environment != env.digest() {
		t.Fatalf("environment %q, want %q", doc.Environment, env.digest())
	}
	requireOutcomes(t, p.Head, "passed", false)
	requireOutcomes(t, p.Base, "failed", true)
	requireVerdict(t, p, VerdictDiscriminates)
}
