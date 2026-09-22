//go:build darwin || linux

package playwrightminimize

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// This explicit qualification runs the real executor against a browser and an
// attested Docker application. Corpus rederivation has separate integration tests.
func TestPSMLiveDockerPredecessorQualification(t *testing.T) {
	if os.Getenv("CORVINT_MINIMIZER_LIVE_QUALIFICATION") != "1" {
		t.Skip("NOT_RUN: set CORVINT_MINIMIZER_LIVE_QUALIFICATION=1 with qualified installed Playwright modules")
	}
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Fatal("installed Playwright modules required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	app, tests := filepath.Join(root, "app"), filepath.Join(root, "tests")
	for _, dir := range []string{app, tests} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Dockerfile", "compose.yml"} {
		data, err := os.ReadFile(filepath.Join(source, "internal/jstestprovider/testdata/application-attestation", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(ctx, goTool, "build", "-o", filepath.Join(app, "fixture-server"), "./internal/jstestprovider/testdata/application-attestation/server")
	build.Dir = source
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("server build: %v %s", err, out)
	}
	provider := filepath.Join(root, "provider")
	liveCommand(t, ctx, source, "go", "build", "-o", provider, "./internal/jstestprovider/testdata/application-attestation/provider")
	liveGitInit(t, ctx, app)
	name := fmt.Sprintf("corvint-minimize-%d", os.Getpid())
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		_ = exec.CommandContext(cleanupCtx, docker, "rm", "-f", name).Run()
		_ = exec.CommandContext(cleanupCtx, docker, "image", "rm", "-f", name+":fixture").Run()
		remaining, err := exec.CommandContext(cleanupCtx, docker, "ps", "-aq", "--filter", "name=^/"+name+"$").Output()
		if err != nil || len(strings.TrimSpace(string(remaining))) != 0 {
			t.Error("owned Docker fixture cleanup unresolved")
		}
	}()
	liveCommand(t, ctx, app, docker, "build", "-t", name+":fixture", app)
	liveCommand(t, ctx, app, docker, "run", "-d", "--name", name, "-p", "127.0.0.1::8080", name+":fixture")
	port := liveCommand(t, ctx, app, docker, "port", name, "8080/tcp")
	url := "http://" + port
	for i := 0; i < 100; i++ {
		if liveCommand(t, ctx, app, docker, "inspect", "--format", "{{.State.Health.Status}}", name) == "healthy" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	gitTool, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	appRevision := liveCommand(t, ctx, app, gitTool, "rev-parse", "HEAD")
	expectation := jstestprovider.ApplicationAttestationExpectation{Repository: jstestprovider.ApplicationRepositoryExpectation{RootCommit: appRevision, Revision: appRevision, Tree: liveCommand(t, ctx, app, gitTool, "rev-parse", "HEAD^{tree}"), DirtyPolicy: "require-clean"}, Build: jstestprovider.ApplicationArtifactIdentity{Kind: "image", Digest: liveCommand(t, ctx, app, docker, "inspect", "--format", "{{.Image}}", name)}, InstanceKind: "docker-container"}
	compose, _ := os.ReadFile(filepath.Join(app, "compose.yml"))
	expectation.Configuration = jstestprovider.ApplicationArtifactIdentity{Kind: "compose", Digest: digestBytes(compose)}
	configData, _ := json.Marshal(struct {
		Profile     string                                           `json:"profile"`
		Expectation jstestprovider.ApplicationAttestationExpectation `json:"expectation"`
	}{"corvint-application-attestation-config/0", expectation})
	providerConfig := filepath.Join(root, "provider.json")
	if err := os.WriteFile(providerConfig, append(configData, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	config := "module.exports = {testDir: '.', outputDir: " + strconvJSON(filepath.Join(root, "output")) + ", workers: 1, retries: 0, projects:[{name:'chromium',use:{browserName:'chromium',channel:'',launchOptions:{executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'}}}]};\n"
	testSource := "const {test,expect}=require('@playwright/test');\nlet poisoned=false;\ntest('predecessor',async ({page})=>{await page.goto(" + strconvJSON(url) + ");poisoned=true;});\ntest('target',async ({page})=>{await page.goto(" + strconvJSON(url) + ");expect(poisoned).toBe(false);});\n"
	for path, data := range map[string]string{"playwright.config.cjs": config, "leak.spec.cjs": testSource} {
		if err := os.WriteFile(filepath.Join(tests, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(modules, filepath.Join(tests, "node_modules")); err != nil {
		t.Fatal(err)
	}
	liveGitInit(t, ctx, tests)
	seed := digestBytes([]byte("live-minimizer-seed"))
	t.Setenv("CORVINT_MINIMIZER_SEED", seed)
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: tests, ConfigFile: filepath.Join(tests, "playwright.config.cjs"), TestFiles: []string{filepath.Join(tests, "leak.spec.cjs")}, RunnerName: "playwright", RunnerVersion: "1.63.0", DeclaredEnvKeys: []string{"CORVINT_MINIMIZER_SEED"}, Timeout: 30 * time.Second}, ExternalServer: true, ServerReadyURL: url + "/health", ServerReadyLimit: 5 * time.Second, ApplicationAttestation: &jstestprovider.ApplicationAttestationProvider{Argv: []string{provider, "--docker", docker, "--git", gitTool, "--repository", app, "--compose", filepath.Join(app, "compose.yml"), "--container", name}, ConfigFile: providerConfig, Timeout: 5 * time.Second}, ObserveDescendants: true}
	original, err := jstestprovider.RunE2E(ctx, cfg)
	if err != nil || len(original.Tests) != 2 {
		t.Fatalf("baseline: %v %+v", err, original.Infrastructure)
	}
	predecessor, target := original.Tests[0], original.Tests[1]
	if target.Name != "target" || target.State != jstestprovider.StateFailed || !jstestprovider.QualifiedReceiptBindingReady(original, target) {
		t.Fatalf("failure not qualified: %+v", original)
	}
	identity := RunIdentity{Fixed: FixedIdentity{TestRevision: original.TestRepositoryAtStart.Revision, ApplicationRevision: appRevision, ConfigDigest: digestBytes([]byte(config)), Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "Google Chrome 153.0.8010.48", Project: "chromium", FixtureSchema: "playwright-use", FixtureDigest: digestBytes(target.Project.Use), SeedIdentity: seed, ApplicationAttestationDigest: "sha256:" + original.ApplicationAttestation.Before.OutputDigest}, Order: []string{predecessor.ID, target.ID}, WorkerTopology: WorkerTopology{Workers: 1, Policy: "fixed"}}
	reset := commandFixture(t, root, "reset", "#!/bin/sh\nexit 0\n")
	cleanup := commandFixture(t, root, "cleanup", "#!/bin/sh\nexit 0\n")
	policy := ResetPolicy{Name: "fresh Playwright process/browser; stateless external application", Digest: digestJSON(struct {
		Reset   Command
		Cleanup Command
	}{reset, cleanup}), ClearBrowserState: true}
	runner := liveRunner{input: LiveRequest{Request: Request{Target: target.ID}, Config: cfg, Reset: reset, Cleanup: cleanup}, original: &original}
	trial := Trial{ID: "live-predecessor", Identity: identity, ResetPolicy: policy, Kind: TrialReproduction}
	result, err := runner.Run(ctx, trial)
	if checked := validateReceipt(trial, result, err); !checked.Valid || result.Outcome != OutcomeFailed {
		t.Fatalf("real predecessor reproduction: %v %+v", err, checked.InvalidReasons)
	}
	trial.ID = "live-isolated"
	trial.Kind = TrialIsolation
	trial.Identity.Order = []string{target.ID}
	result, err = runner.Run(ctx, trial)
	if checked := validateReceipt(trial, result, err); !checked.Valid || result.Outcome != OutcomePassed {
		t.Fatalf("real isolated pass: %v %+v", err, checked.InvalidReasons)
	}
}

func liveCommand(t *testing.T, ctx context.Context, dir, program string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v %s", program, args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func liveGitInit(t *testing.T, ctx context.Context, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "fixture@example.invalid"}, {"config", "user.name", "Fixture"}, {"add", "."}, {"commit", "-qm", "fixture"}} {
		liveCommand(t, ctx, dir, "git", args...)
	}
}
func strconvJSON(value string) string { data, _ := json.Marshal(value); return string(data) }
