package sessiontest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

func goflagsOf(env []string) string {
	flags := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOFLAGS="); ok {
			flags = v
		}
	}
	return flags
}

// A declared flag that changes the built test binary reaches the runner
// identity, so an index built without it is a runner mismatch; one the
// identity cannot carry is refused, which keeps the static plan (GLS-V0-020).
func TestRunnerEnvCarriesBuildFlags(t *testing.T) {
	base := []string{"PATH=" + os.Getenv("PATH"), "GOFLAGS=-mod=mod"}
	declared := func(flags ...string) Declared {
		return Declared{GoEnv: []string{"GOTOOLCHAIN=local"}, GoFlags: flags}
	}
	for name, c := range map[string]struct {
		flags   []string
		want    string
		timeout time.Duration // zero: the go test default
	}{
		"neutral flags add nothing":   {[]string{"-p", "1", "-timeout", "30m", "-count=1", "-failfast"}, "-mod=mod", 30 * time.Minute},
		"ldflags in one word":         {[]string{"-ldflags=-X=example.com/m/p.mode=on"}, "-mod=mod -ldflags=-X=example.com/m/p.mode=on", 0},
		"ldflags in two words":        {[]string{"-ldflags", "-X=example.com/m/p.mode=on"}, "-mod=mod -ldflags=-X=example.com/m/p.mode=on", 0},
		"double-dash build flags":     {[]string{"--race", "--tags", "a,b", "-gcflags=all=-N", "-trimpath"}, "-mod=mod -race -tags=a,b -gcflags=all=-N -trimpath", 0},
		"sanitizers and boolean form": {[]string{"-msan", "-asan", "-buildvcs=false"}, "-mod=mod -msan -asan -buildvcs=false", 0},
	} {
		env, timeout, err := RunnerEnv("", ProfileLocal, declared(c.flags...), base)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := goflagsOf(env); got != c.want {
			t.Errorf("%s: GOFLAGS %q, want %q", name, got, c.want)
		}
		if want := c.timeout; timeout != want && (want != 0 || timeout != linecov.DefaultTestTimeout) {
			t.Errorf("%s: test timeout %v, want %v", name, timeout, want)
		}
	}
	for name, flags := range map[string][]string{
		"caller overlay inline":    {"-overlay=caller.json"},
		"caller overlay separate":  {"--overlay", "caller.json"},
		"unknown transform":        {"-future-build-transform=unknown"},
		"test behaviour":           {"-shuffle=on"},
		"short mode":               {"-short"},
		"coverage":                 {"-cover"},
		"repeat count":             {"-count=3"},
		"an exec wrapper":          {"-exec", "wrap"},
		"a value GOFLAGS splits":   {"-ldflags", "-X a=b -s"},
		"a quoted value":           {`-gcflags='-N'`},
		"a valued flag at the end": {"-ldflags"},
		"a timeout not a duration": {"-timeout=soon"},
		"verbose":                  {"-v"},
		"verbosity off":            {"-v=false"},
		"verbosity on":             {"--v=true"},
		"json":                     {"-json"},
		"json off":                 {"-json=false"},
		"double-dash json":         {"--json"},
		"test-binary verbosity":    {"-test.v=false"},
	} {
		if env, _, err := RunnerEnv("", ProfileLocal, declared(flags...), base); err == nil {
			t.Errorf("%s: %q gave %q, want a refusal", name, flags, goflagsOf(env))
		} else if !strings.Contains(err.Error(), strings.TrimLeft(strings.SplitN(flags[0], "=", 2)[0], "-")) {
			t.Errorf("%s: refusal %q does not name the flag", name, err)
		}
	}
	env, timeout, err := RunnerEnv("", ProfileCI, declared("-shuffle=on"), base)
	if err != nil || !strings.Contains(goflagsOf(env), "-race") || timeout != 50*time.Minute {
		t.Errorf("CI profile: GOFLAGS %q timeout %v err %v, want the CI flags with -race and -timeout 50m", goflagsOf(env), timeout, err)
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := t.TempDir()
	if err := os.WriteFile(root+"/go.mod", []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, flags := range [][]string{nil, {"-ldflags=-X=example.com/m/p.mode=on"}} {
		env, timeout, err := RunnerEnv(root, ProfileLocal, declared(flags...), append(os.Environ(), "GOFLAGS="))
		if err != nil {
			t.Fatal(err)
		}
		id, _, err := linecov.RunnerID(context.Background(), root, env, timeout, "")
		if err != nil {
			t.Fatal(err)
		}
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Errorf("the -ldflags declaration left the runner identity unchanged: %v", ids)
	}
}

// A GOFLAGS setting saved by go env -w is part of the effective GOFLAGS the
// declared build flags join, so the runner identity names both even when the
// environment sets no GOFLAGS (GLS-V0-020).
func TestRunnerEnvKeepsSavedGoFlags(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goenv := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(goenv, []byte("GOFLAGS=-tags=feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var environ []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GOFLAGS=") && !strings.HasPrefix(kv, "GOENV=") {
			environ = append(environ, kv)
		}
	}
	environ = append(environ, "GOENV="+goenv)
	env, timeout, err := RunnerEnv(root, ProfileLocal, Declared{GoEnv: []string{"GOTOOLCHAIN=local"}, GoFlags: []string{"-race"}}, environ)
	if err != nil {
		t.Fatal(err)
	}
	if got := goflagsOf(env); got != "-tags=feature -race" {
		t.Errorf("GOFLAGS %q, want the saved -tags=feature and the declared -race", got)
	}
	id, _, err := linecov.RunnerID(context.Background(), root, env, timeout, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(id, `goflags="-tags=feature -race"`) {
		t.Errorf("runner identity %q does not name both flags", id)
	}
}

// TestRunnerEnvSavedTestFlagOrFailedGoEnvFallsBack: a test flag saved with
// go env -w (go test passes it to the test binary, which collection does
// not apply) makes the identity unreadable, and a go env GOFLAGS that fails
// refuses the environment; both leave selection on the static plan.
func TestRunnerEnvSavedTestFlagOrFailedGoEnvFallsBack(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goenv := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(goenv, []byte("GOFLAGS=-short\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var environ []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GOFLAGS=") && !strings.HasPrefix(kv, "GOENV=") && !strings.HasPrefix(kv, "GOTOOLCHAIN=") {
			environ = append(environ, kv)
		}
	}
	environ = append(environ, "GOENV="+goenv)
	env, timeout, err := RunnerEnv(root, ProfileLocal, Declared{GoEnv: []string{"GOTOOLCHAIN=local"}, GoFlags: []string{"-race"}}, environ)
	if err != nil {
		t.Fatal(err)
	}
	if got := goflagsOf(env); got != "-short -race" {
		t.Errorf("GOFLAGS %q, want the saved -short and the declared -race", got)
	}
	if id, _, err := linecov.RunnerID(context.Background(), root, env, timeout, ""); err == nil || !strings.Contains(err.Error(), `"-short"`) {
		t.Errorf("RunnerID = %q, %v; want a refusal naming the saved -short", id, err)
	}
	if _, _, err := RunnerEnv(root, ProfileLocal, Declared{GoEnv: []string{"GOTOOLCHAIN=not-a-toolchain"}, GoFlags: []string{"-race"}}, environ); err == nil || !strings.Contains(err.Error(), "go env GOFLAGS") {
		t.Errorf("RunnerEnv under a failing go env = %v, want the go env GOFLAGS refusal", err)
	}
}

// TestRunnerEnvDeclaredVerbosityKeepsInverseVerboseTest: collection runs
// every test verbose (-test.v=test2json), but a declared -v=false comes after
// `corvint test`'s -json and turns verbosity off when the tests run, so a test
// that calls F only when !testing.Verbose() records no F coverage. Under that
// declaration the selection must not narrow, or a change to F would leave out
// the test it fails (GLS-V0-020).
func TestRunnerEnvDeclaredVerbosityKeepsInverseVerboseTest(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	ctx := context.Background()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.com/m\n\ngo 1.22\n",
		"q/q.go":      "package q\n\nfunc F() int {\n\treturn 1\n}\n",
		"q/q_test.go": "package q\n\nimport \"testing\"\n\nfunc TestQuiet(t *testing.T) {\n\tif !testing.Verbose() && F() != 1 {\n\t\tt.Fatal(\"quiet F changed\")\n\t}\n}\n\nfunc TestOther(t *testing.T) {}\n",
		".gitignore":  "",
	}
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.test"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	var environ []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GOFLAGS=") && !strings.HasPrefix(kv, "GOENV=") {
			environ = append(environ, kv)
		}
	}
	environ = append(environ, "GOFLAGS=", "GOENV=off")
	runner, goflags, err := linecov.RunnerID(ctx, dir, environ, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := linecov.Collect(ctx, dir, linecov.CollectOptions{Module: "example.com/m", Packages: []string{"q"}, Work: t.TempDir(), Parallel: 2, Timeout: 5 * time.Minute, SuiteTimeout: 5 * time.Minute, TestTimeout: linecov.DefaultTestTimeout, GOFLAGS: goflags})
	if err != nil {
		t.Fatal(err)
	}
	var records []linecov.Record
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s.%s diverged: %s", o.Package, o.Name, o.Divergent)
		}
		records = append(records, *o.Record)
	}
	oids, err := linecov.OIDs(ctx, dir, base)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/m", base, runner, oids, records)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "q", "q.go")
	if err := os.WriteFile(src, []byte("package q\n\nfunc F() int {\n\treturn 2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-qam", "change F")
	head := git("rev-parse", "HEAD")

	for _, flags := range [][]string{{"-count=1", "-v=false"}, {"-count=1", "-json=false"}} {
		env, timeout, refused := RunnerEnv(dir, ProfileLocal, Declared{GoFlags: flags}, environ)
		if refused != nil {
			if !strings.Contains(refused.Error(), strings.SplitN(flags[1], "=", 2)[0]) {
				t.Errorf("%v: refusal %q does not name the flag", flags, refused)
			}
			continue // runner-unavailable: the static plan runs q whole
		}
		id, _, err := linecov.RunnerID(ctx, dir, env, timeout, "")
		if err != nil || id != runner {
			continue // runner-mismatch or unavailable: also the static plan
		}
		diffs, err := linecov.Diff(ctx, dir, base, head)
		if err != nil {
			t.Fatal(err)
		}
		heads, err := linecov.HeadTests(dir, []string{"q"})
		if err != nil {
			t.Fatal(err)
		}
		plans, reason := ix.Narrow(os.Environ(), []string{"q"}, heads, linecov.Effect{}, ix.Analyze(diffs, linecov.Reader(ctx, dir, base)))
		if p := plans["q"]; reason == "" && p.Mode != linecov.Whole && !slices.Contains(p.Tests, "TestQuiet") {
			t.Errorf("%v: the line plan %+v leaves out TestQuiet, which runs F under the declaration", flags, plans)
		}
	}
	// The fault is real: run as `corvint test` runs it, TestQuiet fails.
	// The package is named by its absolute temp path: it is no repository directory.
	cmd := exec.Command("go", "test", "-json", "-count=1", "-v=false", "-run", "^TestQuiet$", filepath.Join(dir, "q"))
	cmd.Dir, cmd.Env = dir, environ
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "quiet F changed") {
		t.Fatalf("fault not detected: %v %s", err, out)
	}
}

// A declared go.env variable a test can observe but that collection does
// not run under (an application setting, GODEBUG, GOMAXPROCS, a C toolchain)
// is refused, so the selection keeps the static plan; the Go toolchain
// settings the identity reads, or that change no executed code, are kept
// (GLS-V0-020).
func TestRunnerEnvRefusesUncarriedDeclaredEnv(t *testing.T) {
	base := []string{"PATH=" + os.Getenv("PATH"), "GOFLAGS="}
	for _, kv := range []string{"GOTOOLCHAIN=local", "GOCACHE=/tmp/c", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "CGO_ENABLED=1", "GOAMD64=v1", "GOFLAGS=-mod=mod"} {
		if _, _, err := RunnerEnv("", ProfileLocal, Declared{GoEnv: []string{kv}}, base); err != nil {
			t.Errorf("%s: %v, want it kept", kv, err)
		}
	}
	for _, kv := range []string{"APP_MODE=fast", "GODEBUG=x509sha1=1", "GOMAXPROCS=1", "GOGC=off", "CC=clang", "CGO_CFLAGS=-O0", "GO111MODULE=off"} {
		name, _, _ := strings.Cut(kv, "=")
		if _, _, err := RunnerEnv("", ProfileLocal, Declared{GoEnv: []string{kv}}, base); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %v, want a refusal naming %s", kv, err, name)
		}
	}
}
