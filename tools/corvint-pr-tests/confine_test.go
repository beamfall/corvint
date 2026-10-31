//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/.github/testconfine"
)

const confineModule = "example.org/confine"

// confineRepository is a real Git repository (with .git) holding a module
// whose packages p and q both read the root's secret.txt from their tests.
// declaration is the .corvint/test-read-scopes.json body, or "" for none.
type confineRepository struct {
	t    *testing.T
	o    options
	ctx  context.Context
	root string
}

func newConfineRepository(t *testing.T, declaration string) *confineRepository {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	r := &confineRepository{t: t, ctx: context.Background(), root: root}
	r.o = options{root: root, out: out, mode: "run", source: strings.Repeat("a", 40), runtime: filepath.Join(out, "runtime")}
	reads := "package %s\nimport (\"os\"; \"path/filepath\"; \"testing\")\nfunc TestReadsRootSecret(t *testing.T) {\n\tif _, err := os.ReadFile(filepath.Join(\"..\", \"secret.txt\")); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	r.write("go.mod", "module "+confineModule+"\n\ngo 1.27.3\n")
	r.write("secret.txt", "repository path outside p\n")
	r.write("p/p_test.go", strings.Replace(reads, "%s", "p", 1))
	r.write("q/q_test.go", strings.Replace(reads, "%s", "q", 1))
	if declaration != "" {
		r.write(declarationPath, declaration)
	}
	r.git("init", "-q")
	r.git("config", "user.email", "fixture@example.invalid")
	r.git("config", "user.name", "Fixture")
	r.commit("base")
	r.o.base = r.git("rev-parse", "HEAD")
	r.o.target = r.o.base
	bin := t.TempDir()
	// The planner receipt and selector verdict the driver validates; the
	// selector selects q only, so a failure of p is outside the selection.
	r.o.planner = filepath.Join(bin, "planner")
	r.o.selector = filepath.Join(bin, "selector")
	planner := "#!/bin/sh\nprintf '{\"profile\":\"affected-plan/1\",\"tool\":\"affected\",\"ok\":true,\"mutates\":false,\"revision\":\"%s\",\"range\":{\"base\":\"%s\"},\"plan\":{\"dirty\":[]},\"provider\":{\"go\":{\"state\":\"FIXTURE\"}}}' \"$(git rev-parse HEAD)\" \"$3\"\n"
	for path, body := range map[string]string{r.o.planner: planner, r.o.selector: "#!/bin/sh\necho 'run " + confineModule + "/q'\n"} {
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := prepareCache(r.o); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *confineRepository) write(name, value string) {
	r.t.Helper()
	path := filepath.Join(r.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		r.t.Fatal(err)
	}
}

func (r *confineRepository) git(args ...string) string {
	r.t.Helper()
	s, err := git(r.ctx, r.o, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	if err != nil {
		r.t.Fatal(err)
	}
	return s
}

func (r *confineRepository) commit(message string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-qm", message)
	return r.git("rev-parse", "HEAD")
}

// buildWrapper builds the AFP-V0-023 wrapper outside every tested checkout,
// from this repository's .github/testconfine.
func buildWrapper(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	sourceRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	wrapper := filepath.Join(t.TempDir(), "test-confine")
	var b strings.Builder
	code, err := command(context.Background(), sourceRoot, append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS="), &b, &b, 2*time.Minute, "go", "build", "-trimpath", "-buildvcs=false", "-o", wrapper, "./.github/testconfine/cmd")
	if err != nil || code != 0 {
		t.Fatalf("build wrapper: %d %v %s", code, err, b.String())
	}
	return wrapper
}

func requireLandlock(t *testing.T) {
	t.Helper()
	if abi := testconfine.ABI(); abi < 2 {
		t.Skipf("AFP-V0-767: landlock ABI %d is below 2 on this kernel, so a declared package cannot be confined here", abi)
	}
}

func readExecution(t *testing.T, o options) execution {
	t.Helper()
	var e execution
	if err := readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestConfinedDriverRunsFailUndeclaredReads(t *testing.T) {
	t.Run("AFP-V0-766 AFP-V0-767 AFP-V0-769", confinedDriverRunsFailUndeclaredReads)
}

func confinedDriverRunsFailUndeclaredReads(t *testing.T) {
	r := newConfineRepository(t, `{"profile":"corvint-test-read-scopes/0","packages":{"p":[]}}`)
	ctx, o := r.ctx, r.o
	unconfined, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if unconfined.Confine != "" || unconfined.ConfinedEnv != nil || !equal(unconfined.Args, testArgs) {
		t.Fatalf("unconfined identity changed: %+v", unconfined)
	}
	o.confine = buildWrapper(t)
	id, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := digestFile(o.confine)
	wantArgs := append([]string{"test", "-exec", o.confine}, testArgs[1:]...)
	if id.Confine != digest || !equal(id.Args, wantArgs) || !equal(id.Env, unconfined.Env) {
		t.Fatalf("AFP-V0-766: confined identity: %+v", id)
	}
	if got := id.ConfinedEnv[len(id.ConfinedEnv)-1]; got != "GOFLAGS=-buildvcs=false" {
		t.Fatalf("AFP-V0-766: confined GOFLAGS not recorded: %q", got)
	}
	for _, v := range id.ConfinedEnv[:len(id.ConfinedEnv)-1] {
		if strings.HasPrefix(v, "GOFLAGS=") {
			t.Fatalf("AFP-V0-766: duplicate GOFLAGS: %v", id.ConfinedEnv)
		}
	}
	full := func(packages ...string) selection {
		return selection{Schema: schema, Base: o.base, Target: o.target, Identity: id, Packages: packages, Reason: "fixture: direct execution"}
	}

	// A wrapper or argv that differs from the frozen identity is refused, never run.
	drift := full("./...")
	drift.Identity.Confine = "other"
	if code, err := execute(ctx, o, drift); code != 2 || err == nil || !strings.Contains(err.Error(), "confinement") {
		t.Fatalf("AFP-V0-766: wrapper drift executed: %d %v", code, err)
	}
	dropped := o
	dropped.confine = filepath.Join(t.TempDir(), "missing")
	if code, err := execute(ctx, dropped, full("./...")); code != 2 || err == nil || !strings.Contains(err.Error(), "wrapper identity unavailable") {
		t.Fatalf("AFP-V0-766: unavailable wrapper ran unconfined: %d %v", code, err)
	}
	inside := o
	inside.confine = filepath.Join(o.root, "secret.txt")
	if _, err := toolIdentity(ctx, inside); err == nil || !strings.Contains(err.Error(), "inside tested checkout") {
		t.Fatalf("AFP-V0-766: wrapper inside the checkout admitted: %v", err)
	}
	container := o
	container.mode = "container"
	if code, err := dispatch(ctx, container); code != 2 || err == nil || !strings.Contains(err.Error(), "--confine") {
		t.Fatalf("AFP-V0-766: container launcher dropped --confine: %d %v", code, err)
	}
	// AFP-V0-769: a .git-less snapshot is refused before any test runs.
	if err := os.Rename(filepath.Join(o.root, ".git"), filepath.Join(o.out, "git-away")); err != nil {
		t.Fatal(err)
	}
	code, err := execute(ctx, o, full("./..."))
	if renameErr := os.Rename(filepath.Join(o.out, "git-away"), filepath.Join(o.root, ".git")); renameErr != nil {
		t.Fatal(renameErr)
	}
	if code != 2 || err == nil || !strings.Contains(err.Error(), "real clone with .git") {
		t.Fatalf("AFP-V0-769: .git-less root executed: %d %v", code, err)
	}

	requireLandlock(t)
	o.seedCache = raceCacheSeed(t, ctx, o)
	expected := []string{confineModule + "/p", confineModule + "/q"}

	// Selected run: the declared package p reads ../secret.txt, which it does not declare.
	code, err = execute(ctx, o, full(confineModule+"/p"))
	if err != nil || code != 1 {
		t.Fatalf("AFP-V0-767: selected confined run: %d %v", code, err)
	}
	e := readExecution(t, o)
	if !equal(e.Args, append(append([]string{}, wantArgs...), confineModule+"/p")) || !validRunEnv(e.Env, id) || e.Env[len(e.Env)-1] != confineRootVariable+o.root {
		t.Fatalf("AFP-V0-766: execution argv/env: %v %v", e.Args, e.Env)
	}
	failed, err := outcomes(filepath.Join(o.out, "go.json"), []string{confineModule + "/p"}, e)
	if err != nil || !equal(failed, []string{confineModule + "/p"}) {
		t.Fatalf("AFP-V0-767: selected failure: %v %v", failed, err)
	}
	raw, _ := os.ReadFile(filepath.Join(o.out, "go.json"))
	if !strings.Contains(string(raw), "permission denied") {
		t.Fatalf("AFP-V0-767: failure is not the Landlock denial:\n%s", raw)
	}

	// Historical shadow row: the full confined run fails p, the selection (q) omits it.
	o.mode = "shadow"
	r.write("q/q.go", "package q\n")
	o.target = r.commit("target")
	r.o.target = o.target
	shadowRow := observeRow(ctx, o, id)
	if !shadowRow.Valid || !equal(shadowRow.Misses, []string{confineModule + "/p"}) {
		t.Fatalf("AFP-V0-767: shadow row did not miss the confined failure: %+v", shadowRow)
	}
	e = readExecution(t, o)
	if failed, err = outcomes(filepath.Join(o.out, "go.json"), expected, e); err != nil || !equal(failed, []string{confineModule + "/p"}) {
		t.Fatalf("AFP-V0-767: shadow outcomes: %v %v", failed, err)
	}
	// The same read passes once declared: a shadow row over that change has no
	// miss, and its retained confined row validates only with its frozen argv
	// and environment (AFP-V0-769).
	r.write(declarationPath, `{"profile":"corvint-test-read-scopes/0","packages":{"p":["secret.txt"]}}`)
	o.base = o.target
	o.target = r.commit("declare secret.txt")
	declaredRow := observeRow(ctx, o, id)
	if !declaredRow.Valid || len(declaredRow.Misses) != 0 {
		t.Fatalf("AFP-V0-767: declared entry not granted: %+v", declaredRow)
	}
	e = readExecution(t, o)
	if e.Exit != 0 {
		t.Fatalf("AFP-V0-767: declared entry run exited %d", e.Exit)
	}
	if err := validateRow(o.out, declaredRow, pair{o.base, o.target}, id); err != nil {
		t.Fatalf("AFP-V0-769: confined row: %v", err)
	}
	for name, mutate := range map[string]func(*execution){
		"unconfined argv": func(e *execution) { e.Args = append(append([]string{}, testArgs...), "./...") },
		"no root":         func(e *execution) { e.Env = e.Env[:len(e.Env)-1] },
	} {
		tampered := e
		tampered.Env = append([]string{}, e.Env...)
		mutate(&tampered)
		if err := writeJSON(filepath.Join(o.out, "execution.json"), tampered); err != nil {
			t.Fatal(err)
		}
		changed := declaredRow
		changed.Files = map[string]string{}
		for k, v := range declaredRow.Files {
			changed.Files[k] = v
		}
		changed.Files["execution.json"], _ = digestFile(filepath.Join(o.out, "execution.json"))
		if validateRow(o.out, changed, pair{o.base, o.target}, id) == nil {
			t.Fatalf("AFP-V0-769: %s admitted", name)
		}
	}
	o.mode = "run"

	// Control: the undeclared read passes when the same package runs unconfined.
	r.write(declarationPath, `{"profile":"corvint-test-read-scopes/0","packages":{"p":[]}}`)
	o.target = r.commit("narrow again")
	control := o
	control.confine = ""
	controlID, err := toolIdentity(ctx, control)
	if err != nil {
		t.Fatal(err)
	}
	plain := full("./...")
	plain.Identity = controlID
	if code, err = execute(ctx, control, plain); err != nil || code != 0 {
		t.Fatalf("unconfined control: %d %v", code, err)
	}
}

func TestCampaignRefusesDeclarationWithoutConfine(t *testing.T) {
	t.Run("AFP-V0-768", campaignRefusesDeclarationWithoutConfine)
}

func campaignRefusesDeclarationWithoutConfine(t *testing.T) {
	r := newConfineRepository(t, "")
	ctx, o := r.ctx, r.o
	// 201 first-parent commits; only the newest carries a declaration.
	commits := []string{o.base}
	tree := r.git("rev-parse", "HEAD^{tree}")
	for i := 1; i < 200; i++ {
		commits = append(commits, r.git("commit-tree", tree, "-p", commits[len(commits)-1], "-m", "step"))
	}
	r.git("reset", "-q", "--hard", commits[len(commits)-1])
	r.write(declarationPath, `{"profile":"corvint-test-read-scopes/0","packages":{"p":["secret.txt"]}}`)
	commits = append(commits, r.commit("declare"))
	o.target = commits[200]
	id, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}

	freezeOut := o
	freezeOut.out = t.TempDir()
	if err := freeze(ctx, freezeOut); err == nil || !strings.Contains(err.Error(), "AFP-V0-768") {
		t.Fatalf("freeze started without a wrapper: %v", err)
	}
	if _, err := os.Stat(filepath.Join(freezeOut.out, "corpus.json")); !os.IsNotExist(err) {
		t.Fatalf("corpus written by a refused freeze: %v", err)
	}

	c := corpus{Schema: schema, Identity: id}
	for i := 0; i < 200; i++ {
		c.Pairs = append(c.Pairs, pair{commits[i], commits[i+1]})
	}
	resume := o
	resume.mode, resume.row, resume.out = "shadow", 200, t.TempDir()
	resume.corpus = filepath.Join(resume.out, "corpus.json")
	if err := writeJSON(resume.corpus, c); err != nil {
		t.Fatal(err)
	}
	if err := shadow(ctx, resume); err == nil || !strings.Contains(err.Error(), "AFP-V0-768") {
		t.Fatalf("shadow started without a wrapper: %v", err)
	}
	if _, err := os.Stat(shadowCheckout(resume)); !os.IsNotExist(err) {
		t.Fatalf("checkout created by a refused shadow: %v", err)
	}
	// A row whose target carries no declaration still runs without one.
	if err := requireConfinement(ctx, o, id, commits[1:200]); err != nil {
		t.Fatalf("undeclared targets refused: %v", err)
	}

	// A wrapper whose Landlock probe fails refuses as well.
	failing := o
	failing.confine = filepath.Join(t.TempDir(), "test-confine")
	if err := os.WriteFile(failing.confine, []byte("#!/bin/sh\necho landlock abi 0\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	failingID, err := toolIdentity(ctx, failing)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireConfinement(ctx, failing, failingID, commits[200:]); err == nil || !strings.Contains(err.Error(), "probe failed") || !strings.Contains(err.Error(), "landlock abi 0") {
		t.Fatalf("failing probe admitted: %v", err)
	}

	// Run mode falls back to the full suite rather than narrow on an unenforced declaration.
	if reason := unconfinedDeclaration(ctx, o, id); reason != "read-scope declaration without --confine" {
		t.Fatalf("run-mode reason: %q", reason)
	}
	if reason := unconfinedDeclaration(ctx, failing, failingID); reason != "" {
		t.Fatalf("confined run refused: %q", reason)
	}
	o.target = commits[199]
	if reason := unconfinedDeclaration(ctx, o, id); reason != "" {
		t.Fatalf("undeclared target refused: %q", reason)
	}

	// With the real wrapper the campaign starts and freezes a confined identity.
	if testconfine.ABI() < 2 {
		t.Logf("AFP-V0-768: landlock ABI %d below 2; the confined freeze is not observed here", testconfine.ABI())
		return
	}
	confined := o
	confined.target = commits[200]
	confined.out = t.TempDir()
	confined.confine = buildWrapper(t)
	if err := freeze(ctx, confined); err != nil {
		t.Fatal(err)
	}
	var frozen corpus
	if err := readJSON(filepath.Join(confined.out, "corpus.json"), &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Identity.Confine == "" || frozen.Identity.Args[1] != "-exec" || equal(frozen.Identity, id) {
		t.Fatalf("confined corpus identity: %+v", frozen.Identity)
	}
	// The campaign row freezes in the source repository and executes in the
	// owned clone; a second invocation resumes the retained row (AFP-V0-769).
	confined.mode, confined.row, confined.corpus = "shadow", 200, filepath.Join(confined.out, "corpus.json")
	confined.seedCache = raceCacheSeed(t, ctx, confined)
	for attempt := 0; attempt < 2; attempt++ {
		if err := shadow(ctx, confined); err != nil {
			t.Fatalf("confined shadow attempt %d: %v", attempt+1, err)
		}
	}
	var e execution
	if err := readJSON(filepath.Join(confined.out, "row-200", "execution.json"), &e); err != nil {
		t.Fatal(err)
	}
	if e.Args[1] != "-exec" || e.Env[len(e.Env)-1] != confineRootVariable+shadowCheckout(confined) {
		t.Fatalf("AFP-V0-769: shadow row did not run confined in the owned clone: %v %v", e.Args, e.Env)
	}
}

func TestConfinedRowValidationBindsArgvAndEnv(t *testing.T) {
	t.Run("AFP-V0-769", func(t *testing.T) {
		env := []string{"PATH=/usr/bin", "GOFLAGS="}
		plain := identity{Env: env}
		confined := identity{Env: env, Confine: strings.Repeat("c", 64)}
		root := confineRootVariable + "/work/checkout"
		for _, c := range []struct {
			name string
			env  []string
			id   identity
			want bool
		}{
			{"unconfined exact", env, plain, true},
			{"unconfined with root", append(append([]string{}, env...), root), plain, false},
			{"confined with root", append(append([]string{}, env...), root), confined, true},
			{"confined without root", env, confined, false},
			{"confined relative root", append(append([]string{}, env...), confineRootVariable+"work"), confined, false},
			{"confined unclean root", append(append([]string{}, env...), confineRootVariable+"/work/../x"), confined, false},
			{"confined two roots", append(append([]string{}, env...), root, root), confined, false},
			{"confined other variable", append(append([]string{}, env...), "HOME=/work"), confined, false},
			{"confined changed env", []string{"PATH=/bin", "GOFLAGS=", root}, confined, false},
		} {
			if got := validRunEnv(c.env, c.id); got != c.want {
				t.Errorf("%s: validRunEnv=%v, want %v", c.name, got, c.want)
			}
		}
		o := options{confine: "/trusted/test-confine"}
		if got := goTestArgs(o); !equal(got, []string{"test", "-exec", "/trusted/test-confine", "-json", "-p", "1", "-count=1", "-race", "-timeout", "50m"}) {
			t.Errorf("confined argv: %v", got)
		}
		if got := goTestArgs(options{}); !equal(got, testArgs) {
			t.Errorf("unconfined argv: %v", got)
		}
	})
}
