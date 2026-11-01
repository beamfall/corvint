package testacceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const externalSentinel = "disposable-session-value-7d2e9a"

// externalFixtureHelper is the stand-in Playwright runner of the external_app fixture. It logs
// in to the caller-owned app with the cookie from the declared storage-state file, records only
// environment key names and the login result, and reports one test that passes only when the
// app accepted the session.
func externalFixtureHelper() bool {
	if len(os.Args) < 3 || filepath.Base(os.Args[1]) != "external-runner.fixture" {
		return false
	}
	state := os.Getenv("E2E_STORAGE_STATE")
	var doc struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	data, _ := os.ReadFile(state)
	_ = json.Unmarshal(data, &doc)
	login := "unauthenticated"
	if req, err := http.NewRequest(http.MethodGet, os.Getenv("E2E_BASE_URL")+"whoami", nil); err == nil {
		for _, c := range doc.Cookies {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			if resp.StatusCode == http.StatusOK {
				login = "authenticated"
			}
			resp.Body.Close()
		}
	}
	keys := []string{}
	for _, entry := range os.Environ() {
		keys = append(keys, strings.SplitN(entry, "=", 2)[0])
	}
	slices.Sort(keys)
	if f, err := os.OpenFile(filepath.Join(filepath.Dir(state), "runner.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		fmt.Fprintf(f, "%s %s\n", strings.Join(keys, ","), login)
		f.Close()
	}
	status, result := "expected", "passed"
	if login != "authenticated" {
		status, result = "unexpected", "failed"
	}
	fmt.Printf(`{"suites":[{"title":"test.js","file":"test.js","specs":[{"title":"counter","file":"test.js","line":1,"tests":[{"status":%q,"results":[{"status":%q,"retry":0,"duration":2}]}]}]}],"errors":[]}`, status, result)
	return true
}

type externalFixture struct {
	request Request
	exe     string
	hookDir string
	state   string
	logins  *atomic.Int32
}

// externalRequest stages a committed fixture repository, a caller-owned loopback app that
// counts requests carrying the disposable session cookie, a storage-state file outside the
// repository, and a shell health hook that logs its stdin and fails on a marker file.
func externalRequest(t *testing.T) externalFixture {
	t.Helper()
	repo := canonicalTemp(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json", "external-runner.fixture"} {
		write(t, filepath.Join(repo, name), "{}\n")
	}
	command(t, repo, "git", "init", "-q")
	command(t, repo, "git", "add", ".")
	command(t, repo, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	pin := Repository{Root: repo, Commit: command(t, repo, "git", "rev-parse", "HEAD"), Tree: command(t, repo, "git", "rev-parse", "HEAD^{tree}")}

	logins := &atomic.Int32{}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/whoami" {
			return
		}
		if c, err := r.Cookie("sid"); err != nil || c.Value != externalSentinel {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		logins.Add(1)
	}))
	t.Cleanup(app.Close)

	stateDir := canonicalTemp(t)
	state := filepath.Join(stateDir, "state.json")
	stateBytes, _ := json.Marshal(map[string]any{
		"cookies": []any{map[string]any{"name": "sid", "value": externalSentinel, "domain": "127.0.0.1", "path": "/"}},
		"origins": []any{map[string]any{"origin": app.URL, "localStorage": []any{map[string]any{"name": "user", "value": "disposable-user-" + externalSentinel}}}},
	})
	if err := os.WriteFile(state, stateBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	hookDir := canonicalTemp(t)
	hook := filepath.Join(hookDir, "health-hook")
	write(t, hook, "#!/bin/sh\ninput=$(/bin/cat)\nprintf '%s\\n' \"$input\" >> '"+hookDir+"/hook.log'\n"+
		"case \"$input\" in\n*'\"phase\":\"reset\"'*) [ -e '"+hookDir+"/fail-reset' ] && exit 3 ;;\n*'\"phase\":\"health\"'*) [ -e '"+hookDir+"/fail-health' ] && exit 4 ;;\nesac\nexit 0\n")

	entry := filepath.Join(repo, "external-runner.fixture")
	r := Request{Schema: RequestSchema, Product: pin, TestRepository: pin, Config: filepath.Join(repo, "config.js"), Package: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"),
		Runner:   Command{Argv: []string{exe, entry}, ExecutableSHA256: hashFile(t, exe), EntrypointSHA256: hashFile(t, entry)},
		ReadyURL: app.URL + "/", AppBuildDir: repo, RunnerVersion: "fixture", Environment: "external-fixture", Repeat: 2, TimeoutSeconds: 10,
		Tests: []Test{{ID: "counter", File: filepath.Join(repo, "test.js"), Line: 1, Title: "counter"}},
		ExternalApp: &ExternalApp{Profile: behaviorfalsify.AuthenticatedDisposable, Health: Command{Argv: []string{hook}, ExecutableSHA256: hashFile(t, hook)},
			Environment: []EnvironmentValue{{Key: "E2E_BASE_URL", Value: app.URL + "/"}, {Key: "E2E_STORAGE_STATE", Value: state}},
			AuthFiles:   []File{{Path: state, SHA256: Hash(stateBytes)}}}}
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json"} {
		p := filepath.Join(repo, name)
		r.Inputs = append(r.Inputs, File{Path: p, SHA256: hashFile(t, p)})
	}
	if err := Validate(r); err != nil {
		t.Fatal(err)
	}
	return externalFixture{request: r, exe: exe, hookDir: hookDir, state: state, logins: logins}
}

func (f externalFixture) execute(t *testing.T) Report {
	t.Helper()
	report, err := ExecuteWithWorker(context.Background(), f.request, Digest(f.request), f.exe, []string{"worker"}, "fixture-build", "")
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func cloneExternal(r Request) Request {
	x := *r.ExternalApp
	x.Environment = slices.Clone(x.Environment)
	x.AuthFiles = slices.Clone(x.AuthFiles)
	x.Health.Argv = slices.Clone(x.Health.Argv)
	r.ExternalApp = &x
	r.Runner.Argv = slices.Clone(r.Runner.Argv)
	r.Inputs = slices.Clone(r.Inputs)
	return r
}

func TestNEAV0012ExternalProfileValidation(t *testing.T) {
	f := externalRequest(t)
	repo := f.request.TestRepository.Root
	otherHook := filepath.Join(f.hookDir, "other-hook")
	write(t, otherHook, "#!/bin/sh\nexit 0\n")
	cases := map[string]struct {
		mutate func(*Request)
		code   string
	}{
		"freshness schema": {func(r *Request) { r.Schema = FreshRequestSchema }, "external-app-freshness-unsupported"},
		"other profile":    {func(r *Request) { r.ExternalApp.Profile = "authenticated" }, "external-app-profile-invalid"},
		"server declared": {func(r *Request) {
			r.Server = Command{Argv: []string{f.exe}, ExecutableSHA256: hashFile(t, f.exe)}
		}, "external-server-command-present"},
		"hook argument":   {func(r *Request) { r.ExternalApp.Health.Argv = append(r.ExternalApp.Health.Argv, "--reset") }, "external-health-invalid"},
		"hook drift":      {func(r *Request) { r.ExternalApp.Health.Argv = []string{otherHook} }, "external-health-invalid"},
		"hook entrypoint": {func(r *Request) { r.ExternalApp.Health.EntrypointSHA256 = Hash([]byte("x")) }, "external-health-invalid"},
		"loopback hostname": {func(r *Request) {
			r.ReadyURL = strings.Replace(r.ReadyURL, "127.0.0.1", "localhost", 1)
		}, "ready-url-not-loopback"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := cloneExternal(f.request)
			tc.mutate(&r)
			if err := Validate(r); err == nil || err.Error() != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
	t.Run("secret-shaped tracked input", func(t *testing.T) {
		r := cloneExternal(f.request)
		p := filepath.Join(repo, "test.js")
		write(t, p, "const password='correct-horse-battery-staple-91';\n")
		defer write(t, p, "{}\n")
		r.Inputs[0].SHA256 = hashFile(t, p)
		command(t, repo, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qam", "secret")
		r.Product.Commit = command(t, repo, "git", "rev-parse", "HEAD")
		r.Product.Tree = command(t, repo, "git", "rev-parse", "HEAD^{tree}")
		r.TestRepository = r.Product
		if err := Validate(r); err == nil || err.Error() != "input-secret-shaped" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNEAV0012CallerOwnedServerRun(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_"+strings.Repeat("Z9y8X7w6V5", 4)[:36])
	f := externalRequest(t)
	report := f.execute(t)
	if report.External == nil || report.External.Profile != behaviorfalsify.AuthenticatedDisposable || report.External.ServerOwnership != "caller" || report.External.ReadyURL != f.request.ReadyURL || len(report.External.AuthFiles) != 1 || report.External.AuthFiles[0] != f.request.ExternalApp.AuthFiles[0] {
		t.Fatalf("external = %+v", report.External)
	}
	for _, want := range []string{"external-server-state-caller-owned", "disposable-account-caller-declared"} {
		if !slices.Contains(report.Unknowns, want) {
			t.Fatalf("unknowns %v lack %s", report.Unknowns, want)
		}
	}
	if len(report.Runs) != 2 {
		t.Fatalf("runs = %+v", report.Runs)
	}
	for _, run := range report.Runs {
		if slices.Contains(run.Reasons, "provider-server-cleanup-unknown") || len(run.Rows) != 1 || run.Rows[0].State != jstestprovider.StatePassed {
			t.Fatalf("run = %+v", run)
		}
	}
	if got := f.logins.Load(); got != 2 {
		t.Fatalf("authenticated logins = %d, want 2", got)
	}
	if report.Verdict != "blocked" || (report.Blocked != "freshness-unknown" && report.Blocked != "infrastructure") {
		t.Fatalf("verdict = %s blocked = %q", report.Verdict, report.Blocked)
	}
	if !strings.Contains(report.Body, "| Authentication | authenticated-disposable |") || !strings.Contains(report.Body, "**Blocked reason: "+report.Blocked+"**") {
		t.Fatalf("body = %s", report.Body)
	}
	encoded, _ := json.Marshal(report)
	if bytes.Contains(encoded, []byte(externalSentinel)) {
		t.Fatal("report carries storage-state contents")
	}
}

func TestNEAV0013DeclaredEnvironment(t *testing.T) {
	cases := map[string]struct {
		values []EnvironmentValue
		code   string
	}{
		"lowercase key":    {[]EnvironmentValue{{Key: "e2e_url", Value: "x"}}, "external-environment-invalid"},
		"duplicate key":    {[]EnvironmentValue{{Key: "E2E_URL", Value: "a"}, {Key: "E2E_URL", Value: "b"}}, "external-environment-invalid"},
		"reserved key":     {[]EnvironmentValue{{Key: "HOME", Value: "/tmp"}}, "external-environment-key-reserved"},
		"reserved prefix":  {[]EnvironmentValue{{Key: "NODE_OPTIONS", Value: "--require=x"}}, "external-environment-key-reserved"},
		"corvint prefix":   {[]EnvironmentValue{{Key: "CORVINT_ROOT", Value: "/tmp"}}, "external-environment-key-reserved"},
		"shell startup":    {[]EnvironmentValue{{Key: "BASH_ENV", Value: "/tmp/x.sh"}}, "external-environment-key-reserved"},
		"proxy":            {[]EnvironmentValue{{Key: "HTTPS_PROXY", Value: "http://10.0.0.1:3128"}}, "external-environment-key-reserved"},
		"interpreter":      {[]EnvironmentValue{{Key: "PYTHONSTARTUP", Value: "/tmp/x.py"}}, "external-environment-key-reserved"},
		"locale prefix":    {[]EnvironmentValue{{Key: "LC_ALL", Value: "C"}}, "external-environment-key-reserved"},
		"credential key":   {[]EnvironmentValue{{Key: "E2E_SESSION_ID", Value: "plain"}}, "external-environment-key-credential"},
		"secret value":     {[]EnvironmentValue{{Key: "E2E_BASE_NAME", Value: "ghp_" + strings.Repeat("A1b2C3d4E5", 4)[:36]}}, "external-environment-value-secret"},
		"multi-line value": {[]EnvironmentValue{{Key: "E2E_URL", Value: "a\nb"}}, "external-environment-value-secret"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateEnvironment(tc.values); err == nil || err.Error() != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
	tooMany := make([]EnvironmentValue, 17)
	for i := range tooMany {
		tooMany[i] = EnvironmentValue{Key: fmt.Sprintf("E2E_K%d", i), Value: "v"}
	}
	if err := validateEnvironment(tooMany); err == nil || err.Error() != "external-environment-invalid" {
		t.Fatalf("17 keys: %v", err)
	}

	f := externalRequest(t)
	t.Run("values are pinned by the digest", func(t *testing.T) {
		r := cloneExternal(f.request)
		r.ExternalApp.Environment[0].Value = "http://127.0.0.1:1/"
		if Digest(r) == Digest(f.request) {
			t.Fatal("changed environment value kept the digest")
		}
	})
	t.Run("worker admits exactly the declared pairs", func(t *testing.T) {
		saved := os.Environ()
		temporary := os.TempDir()
		t.Cleanup(func() {
			os.Clearenv()
			for _, entry := range saved {
				parts := strings.SplitN(entry, "=", 2)
				os.Setenv(parts[0], parts[1])
			}
		})
		os.Clearenv()
		t.Setenv("TMPDIR", temporary)
		for _, entry := range append(workerEnvironment(), externalEnvironment(f.request)...) {
			parts := strings.SplitN(entry, "=", 2)
			t.Setenv(parts[0], parts[1])
		}
		if !WorkerEnvironmentSafeFor(f.request) || WorkerEnvironmentSafe() {
			t.Fatal("declared pairs not admitted only for their request")
		}
		t.Setenv("E2E_EXTRA", "x")
		if WorkerEnvironmentSafeFor(f.request) {
			t.Fatal("undeclared key admitted")
		}
	})
	t.Run("runner sees declared keys and nothing ambient", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "ghp_"+strings.Repeat("Q1w2E3r4T5", 4)[:36])
		g := externalRequest(t)
		g.execute(t)
		log, err := os.ReadFile(filepath.Join(filepath.Dir(g.state), "runner.log"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
			if line != "E2E_BASE_URL,E2E_STORAGE_STATE,LANG,LC_ALL,PATH,TMPDIR authenticated" {
				t.Fatalf("runner environment = %q", line)
			}
		}
	})
}

func TestNEAV0014AuthFilePinning(t *testing.T) {
	f := externalRequest(t)
	t.Run("validation rereads the pin", func(t *testing.T) {
		r := cloneExternal(f.request)
		r.ExternalApp.AuthFiles[0].SHA256 = Hash([]byte("other"))
		if err := Validate(r); err == nil || err.Error() != "auth-file-drift" {
			t.Fatalf("err = %v", err)
		}
	})
	repo := f.request.TestRepository.Root
	inside := filepath.Join(repo, "state.json")
	write(t, inside, `{"cookies":[]}`)
	dir := canonicalTemp(t)
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(f.state, link); err != nil {
		t.Fatal(err)
	}
	badShape := filepath.Join(dir, "bad.json")
	write(t, badShape, `{"cookies":[],"password":"x"}`)
	pinned := f.request.ExternalApp.AuthFiles[0]
	cases := map[string]struct {
		files []File
		code  string
	}{
		"no files":         {nil, "auth-file-invalid"},
		"five files":       {[]File{pinned, pinned, pinned, pinned, pinned}, "auth-file-invalid"},
		"duplicate":        {[]File{pinned, pinned}, "auth-file-invalid"},
		"inside the repo":  {[]File{{Path: inside, SHA256: hashFile(t, inside)}}, "auth-file-invalid"},
		"symlinked path":   {[]File{{Path: link, SHA256: pinned.SHA256}}, "auth-file-invalid"},
		"unknown top key":  {[]File{{Path: badShape, SHA256: hashFile(t, badShape)}}, "auth-file-invalid"},
		"relative path":    {[]File{{Path: "state.json", SHA256: pinned.SHA256}}, "auth-file-invalid"},
		"digest mismatch":  {[]File{{Path: pinned.Path, SHA256: Hash([]byte("other"))}}, "auth-file-drift"},
		"missing the file": {[]File{{Path: filepath.Join(dir, "absent.json"), SHA256: pinned.SHA256}}, "auth-file-invalid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := cloneExternal(f.request)
			r.ExternalApp.AuthFiles = tc.files
			if _, err := authFileBytes(r); err == nil || err.Error() != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
	t.Run("drift after a run is recorded", func(t *testing.T) {
		g := externalRequest(t)
		if err := os.WriteFile(g.state, []byte(`{"cookies":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		run := Run{Reasons: []string{}}
		externalRecheck(&run, g.request)
		if !slices.Equal(run.Reasons, []string{"auth-file-drift", "inputs-drifted"}) {
			t.Fatalf("reasons = %v", run.Reasons)
		}
		if _, err := ExecuteWithWorker(context.Background(), g.request, Digest(g.request), g.exe, []string{"worker"}, "b", ""); err == nil || err.Error() != "auth-file-drift" {
			t.Fatalf("execute err = %v", err)
		}
	})
}

func TestNEAV0014AuthMaterialNeverPublished(t *testing.T) {
	f := externalRequest(t)
	t.Run("synthetic report", func(t *testing.T) {
		for _, report := range []Report{
			{Runs: []Run{{Rows: []Row{{ObservedID: externalSentinel}}}}},
			{Environment: "disposable-user-" + externalSentinel},
		} {
			if err := refuseAuthMaterial(f.request, report); err == nil || err.Error() != "auth-material-in-report" {
				t.Fatalf("err = %v", err)
			}
		}
		if err := refuseAuthMaterial(f.request, Report{Environment: "clean"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("execution refuses to publish", func(t *testing.T) {
		r := cloneExternal(f.request)
		r.Environment = externalSentinel
		if err := Validate(r); err != nil {
			t.Fatal(err)
		}
		if _, err := ExecuteWithWorker(context.Background(), r, Digest(r), f.exe, []string{"worker"}, "b", ""); err == nil || err.Error() != "auth-material-in-report" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNEAV0015HealthHookFailure(t *testing.T) {
	t.Run("reset failure skips the runner", func(t *testing.T) {
		f := externalRequest(t)
		write(t, filepath.Join(f.hookDir, "fail-reset"), "x")
		report := f.execute(t)
		for _, run := range report.Runs {
			if !slices.Contains(run.Reasons, "external-reset-failed") || len(run.Rows) != 0 || len(run.ExternalHooks) != 1 || run.ExternalHooks[0].Phase != "reset" || run.ExternalHooks[0].Status != "failed" {
				t.Fatalf("run = %+v", run)
			}
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(f.state), "runner.log")); !os.IsNotExist(err) || f.logins.Load() != 0 {
			t.Fatalf("runner ran after a failed reset: %v", err)
		}
		if report.Verdict != "blocked" || report.Blocked != "infrastructure" {
			t.Fatalf("verdict = %s %q", report.Verdict, report.Blocked)
		}
	})
	t.Run("health failure after the run", func(t *testing.T) {
		f := externalRequest(t)
		write(t, filepath.Join(f.hookDir, "fail-health"), "x")
		report := f.execute(t)
		for _, run := range report.Runs {
			if !slices.Contains(run.Reasons, "external-health-failed") || len(run.ExternalHooks) != 2 || run.ExternalHooks[0].Status != "passed" || run.ExternalHooks[1].Phase != "health" || run.ExternalHooks[1].Status != "failed" {
				t.Fatalf("run = %+v", run)
			}
		}
		if report.Blocked != "infrastructure" {
			t.Fatalf("blocked = %q", report.Blocked)
		}
	})
	t.Run("hook input", func(t *testing.T) {
		f := externalRequest(t)
		f.execute(t)
		log, err := os.ReadFile(filepath.Join(f.hookDir, "hook.log"))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{}
		for _, n := range []int{1, 2} {
			for _, phase := range []string{"reset", "health"} {
				want = append(want, fmt.Sprintf(`{"schema":%q,"phase":%q,"kind":"repeat","ordinal":%d}`, ExternalHealthSchema, phase, n))
			}
		}
		if got := strings.Split(strings.TrimSpace(string(log)), "\n"); !slices.Equal(got, want) {
			t.Fatalf("hook inputs = %q", got)
		}
	})
	t.Run("hook outcome", func(t *testing.T) {
		f := externalRequest(t)
		write(t, filepath.Join(f.hookDir, "fail-health"), "x")
		if h := externalHook(context.Background(), f.request, "reset", "repeat", 1); h.Status != "passed" || h.Phase != "reset" {
			t.Fatalf("reset = %+v", h)
		}
		if h := externalHook(context.Background(), f.request, "health", "repeat", 1); h.Status != "failed" {
			t.Fatalf("health = %+v", h)
		}
	})
	t.Run("replaced hook never runs", func(t *testing.T) {
		f := externalRequest(t)
		marker := filepath.Join(f.hookDir, "replaced")
		write(t, f.request.ExternalApp.Health.Argv[0], "#!/bin/sh\n: > '"+marker+"'\nexit 0\n")
		if h := externalHook(context.Background(), f.request, "health", "repeat", 1); h.Status != "failed" {
			t.Fatalf("replaced hook = %+v", h)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("replaced hook bytes executed: %v", err)
		}
	})
	t.Run("reset hook changes storage state", func(t *testing.T) {
		f := externalRequest(t)
		hook := f.request.ExternalApp.Health.Argv[0]
		write(t, hook, "#!/bin/sh\nprintf '%s' '{\"cookies\":[]}' > '"+f.state+"'\nexit 0\n")
		b, err := os.ReadFile(hook)
		if err != nil {
			t.Fatal(err)
		}
		f.request.ExternalApp.Health.ExecutableSHA256 = Hash(b)
		if _, err := ExecuteWithWorker(context.Background(), f.request, Digest(f.request), f.exe, []string{"worker"}, "fixture-build", ""); err == nil || err.Error() != "auth-file-drift" {
			t.Fatalf("err = %v", err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(f.state), "runner.log")); !os.IsNotExist(err) || f.logins.Load() != 0 {
			t.Fatalf("runner read a storage state that drifted after reset: %v", err)
		}
	})
}

func TestNEAV0016LegacyWireUnchanged(t *testing.T) {
	r := scopeRequest(t, false)
	encoded, _ := json.Marshal(r)
	job, _ := json.Marshal(Job{Request: r, Kind: "repeat", Ordinal: hookOrdinal(r, 1)})
	if bytes.Contains(encoded, []byte(`"external_app"`)) || bytes.Contains(job, []byte(`"ordinal"`)) {
		t.Fatalf("legacy request or job encodes external fields: %s", job)
	}
	exe, _ := filepath.EvalSymlinks(r.Runner.Argv[0])
	report, err := ExecuteWithWorker(context.Background(), r, Digest(r), exe, []string{"worker"}, "b", canonicalTemp(t))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(report)
	for _, field := range []string{`"external"`, `"external_hooks"`} {
		if bytes.Contains(b, []byte(field)) {
			t.Fatalf("legacy report encodes %s", field)
		}
	}
	if strings.Contains(report.Body, "Authentication") {
		t.Fatalf("legacy body has an authentication row: %s", report.Body)
	}
}

func TestNEAV0017BlockedReasonSplit(t *testing.T) {
	absent := Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{Absent: true}}
	clean := []Run{{Kind: "repeat", Reasons: []string{}}, {Kind: "repeat", Reasons: []string{}}}
	a := Assessment{Cleanup: "observed-absent", Reasons: []string{"negative-control-incomplete", "provider-freshness-unknown"}}
	cases := map[string]struct {
		runs     []Run
		a        Assessment
		c        Control
		complete bool
		want     string
	}{
		"clean, no control declared": {clean, a, Control{Status: "missing", Cleanup: absent}, true, "freshness-unknown"},
		"clean, control killed":      {clean, a, Control{Status: "killed", Cleanup: absent}, true, "freshness-unknown"},
		"run reason":                 {[]Run{{Kind: "repeat", Reasons: []string{"external-health-failed"}}}, a, Control{Status: "missing", Cleanup: absent}, true, "infrastructure"},
		"repeats incomplete":         {clean, a, Control{Status: "missing", Cleanup: absent}, false, "infrastructure"},
		"run cleanup unknown":        {clean, Assessment{Cleanup: "unknown"}, Control{Status: "missing", Cleanup: absent}, true, "infrastructure"},
		"control cleanup unknown":    {clean, a, Control{Status: "missing"}, true, "infrastructure"},
		"control invalid":            {clean, a, Control{Status: "invalid", Cleanup: absent}, true, "infrastructure"},
		"control not run":            {clean, a, Control{Status: "not-run", Cleanup: absent}, true, "infrastructure"},
		"control blocked":            {clean, a, Control{Status: "blocked", Cleanup: absent}, true, "infrastructure"},
		"test incomplete":            {clean, Assessment{Cleanup: "observed-absent", Reasons: []string{"test-incomplete"}}, Control{Status: "missing", Cleanup: absent}, true, "infrastructure"},
		"probe reason only":          {append(slices.Clone(clean), Run{Kind: "probe-original", Reasons: []string{"x"}}), a, Control{Status: "missing", Cleanup: absent}, true, "freshness-unknown"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := blockedReason(tc.runs, tc.a, tc.c, tc.complete); got != tc.want {
				t.Fatalf("blockedReason = %s, want %s", got, tc.want)
			}
		})
	}
	t.Run("report level", func(t *testing.T) {
		r := Request{Schema: RequestSchema, Repeat: 1, Tests: []Test{{ID: "a"}, {ID: "b"}}}
		row := func(id string, state jstestprovider.ExecutionState) Row {
			return Row{ID: id, State: state, Attempts: 1, IdentityUnknown: []string{}}
		}
		build := func(b jstestprovider.ExecutionState, controlB string) Report {
			report := Report{Runs: []Run{{Kind: "repeat", Reasons: []string{}, Cleanup: absent, Rows: []Row{row("a", jstestprovider.StatePassed), row("b", b)}}},
				Controls: []Control{{ID: "a", Status: "missing", Cleanup: absent}, {ID: "b", Status: controlB, Cleanup: absent}}}
			classify(&report, r)
			return report
		}
		if report := build(jstestprovider.StatePassed, "missing"); report.Verdict != "blocked" || report.Blocked != "freshness-unknown" || report.Assessments[0].Blocked != "freshness-unknown" {
			t.Fatalf("clean report = %s %q", report.Verdict, report.Blocked)
		}
		if report := build(jstestprovider.StatePassed, "invalid"); report.Blocked != "infrastructure" || report.Assessments[0].Blocked != "freshness-unknown" || report.Assessments[1].Blocked != "infrastructure" || !strings.Contains(report.Body, "**Blocked reason: infrastructure**") {
			t.Fatalf("one infrastructure assessment = %q %+v", report.Blocked, report.Assessments)
		}
		if report := build(jstestprovider.StateFailed, "missing"); report.Verdict != "rejected" || report.Blocked != "" || strings.Contains(report.Body, "Blocked reason") {
			t.Fatalf("rejected report = %s %q", report.Verdict, report.Blocked)
		}
	})
}
