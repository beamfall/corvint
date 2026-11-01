package changee2e

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// freeLoopbackAddr returns a loopback address that was free a moment ago. serve binds the declared
// fixed port itself, so the port is released before serveTree runs and a co-tenant could take it in
// between. The port is drawn from below the kernel's ephemeral range, which every co-tenant's port-0
// listener and outbound connection uses: the self-hosted runners share one kernel whose range holds
// only 4096 ports, where a freed ephemeral port was taken before serve started (V1-2616).
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	low := 32768 // Linux default; macOS and Windows start higher
	if raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range"); err == nil {
		if fields := strings.Fields(string(raw)); len(fields) == 2 {
			if n, err := strconv.Atoi(fields[0]); err == nil {
				low = n
			}
		}
	}
	if low > 32768 {
		low = 32768
	}
	for range 200 {
		port := 0 // no band below the range: fall back to a kernel-chosen port
		if low > 10000 {
			port = 10000 + rand.IntN(low-10000)
		}
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		return addr
	}
	t.Fatal("no free loopback port below the ephemeral range")
	return ""
}

func testEnvironment(t *testing.T, addr string) *Environment {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Environment{Serve: []string{bin, testServeArg}, BaseURL: "http://" + addr, Ready: "/", Env: []string{testServeAddr}}
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
		"path name":      with("env", []string{"PATH"}),
		"home name":      with("env", []string{"HOME"}),
		"node bin name":  with("env", []string{envNodeBin}),
		"bbf name":       with("env", []string{"CORVINT_BEHAVIOR_COMMAND_ROLE"}),
		"escaping serve": with("serve", []string{"../bin/serve"}),
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
// names; ready gates the run on a 2xx answer; stop kills the process group and releases the port.
func TestCCEV0021ServeTree(t *testing.T) {
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "index.html"), []byte("side tree"), 0o644); err != nil {
		t.Fatal(err)
	}
	addr := freeLoopbackAddr(t)
	t.Setenv(testServeAddr, addr)
	t.Setenv(testServeLeak, "leaked")
	env := testEnvironment(t, addr)
	path := os.Getenv("PATH")
	s, err := serveTree(bg, env, tree, "", path)
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
	if _, err := serveTree(bg, env, tree, "", path); !isEnvCode(err, "port-in-use") {
		t.Fatalf("second serve: %v", err)
	}
	s.stop()
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = c.Close()
		t.Fatal("serve still answers after stop")
	}

	// A serve that ends after ready is no longer the server under test.
	s, err = serveTree(bg, env, tree, "", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.cmd.Process.Kill()
	<-s.exited
	if err := s.alive(); !isEnvCode(err, "serve-exited") {
		t.Fatalf("alive after exit: %v", err)
	}
	s.stop()

	missing := &Environment{Serve: []string{"corvint-no-such-serve-tool"}, BaseURL: env.BaseURL, Ready: "/"}
	if _, err := serveTree(bg, missing, tree, lookServe(missing), path); !isEnvCode(err, "serve-tool-missing") {
		t.Fatalf("missing tool: %v", err)
	}
	exits := &Environment{Serve: []string{"false"}, BaseURL: env.BaseURL, Ready: "/"}
	if _, err := serveTree(bg, exits, tree, lookServe(exits), path); !isEnvCode(err, "serve-exited") {
		t.Fatalf("exiting serve: %v", err)
	}
	// A failed environment is a side without a qualified receipt, so the verdict is INCONCLUSIVE.
	if _, reason := qualify(jstestprovider.Receipt{}, &envError{"ready-timeout"}, 1, nil); reason != "environment-unavailable:ready-timeout" {
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

// CCE-V0-021: a relative serve[0] names an executable inside the side's tree and may not leave it
// through a symlink; a bare name is the executable Prove resolved, and nothing else.
func TestCCEV0021ServeCommandResolution(t *testing.T) {
	tree, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "bin", "serve"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(tree, "bin", "out")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, bin, want, code string }{
		{"./bin/serve", "", filepath.Join(tree, "bin", "serve"), ""},
		{"bin/out", "", "", "serve-outside-tree"},
		{"bin/absent", "", "", "serve-tool-missing"},
		{"npm", "", "", "serve-tool-missing"},
		{"npm", self, self, ""},
	} {
		got, err := serveCommand(&Environment{Serve: []string{c.name}}, tree, c.bin)
		if c.code != "" && !isEnvCode(err, c.code) || c.code == "" && (err != nil || got != c.want) {
			t.Fatalf("%s: got %q, %v", c.name, got, err)
		}
	}
	if lookServe(&Environment{Serve: []string{"./bin/serve"}}) != "" || !filepath.IsAbs(lookServe(&Environment{Serve: []string{"sh"}})) {
		t.Fatal("lookServe resolves only bare names, to absolute paths")
	}
}

// CCE-V0-021: ready accepts only a 2xx answer, matching the provider's external-server check.
func TestCCEV0021ReadyRequires2xx(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(bg, time.Second)
	defer cancel()
	if err := pollReady(ctx, srv.URL+"/", make(chan struct{})); !isEnvCode(err, "ready-cancelled") {
		t.Fatalf("404 accepted as ready: %v", err)
	}
}

// CCE-V0-021: the control request passes only the declared names that are set, and carries the
// serve executable and PATH Prove resolved, so the BBF hook serves as the sides did.
func TestCCEV0021ControlRequestEnvironment(t *testing.T) {
	t.Setenv("CORVINT_E2E_SET_NAME", "1")
	env := &Environment{Serve: []string{"npm", "run", "dev"}, BaseURL: "http://127.0.0.1:5173", Ready: "/", Env: []string{"CORVINT_E2E_SET_NAME", "CORVINT_E2E_UNSET_NAME"}}
	e := &proveEnv{environment: env, serveBin: "/usr/bin/true", servePath: "/usr/bin"}
	req := e.controlRequest(Scenario{}, controlSpecs(Scenario{}), "rev", "/repo", "/disposable", jstestprovider.Receipt{})
	if !slices.Contains(req.DeclaredEnvKeys, "CORVINT_E2E_SET_NAME") || slices.Contains(req.DeclaredEnvKeys, "CORVINT_E2E_UNSET_NAME") {
		t.Fatalf("declared keys %v", req.DeclaredEnvKeys)
	}
	for _, c := range req.Controls {
		if c.Definition["serve_bin"] != "/usr/bin/true" || c.Definition["serve_path"] != "/usr/bin" || c.Definition["environment"] == "" {
			t.Fatalf("definition %v", c.Definition)
		}
	}
	// A path serve[0] has no resolved executable, and BBF refuses an empty definition value.
	e.serveBin = ""
	for _, c := range e.controlRequest(Scenario{}, controlSpecs(Scenario{}), "rev", "/repo", "/disposable", jstestprovider.Receipt{}).Controls {
		if _, ok := c.Definition["serve_bin"]; ok {
			t.Fatalf("empty serve_bin carried: %v", c.Definition)
		}
	}
}
