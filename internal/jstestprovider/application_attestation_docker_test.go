//go:build darwin || linux

package jstestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestApplicationAttestationDockerComposeQualification(t *testing.T) {
	if !*dockerQualification {
		t.Skip("NOT_RUN: pass -corvint-docker-qualification for the explicit Docker Compose qualification")
	}
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Fatal("NOT_RUN: CORVINT_PLAYWRIGHT_MODULES must name installed Playwright modules")
	}
	docker := qualificationExecutable(t, "docker")
	git := qualificationExecutable(t, "git")
	goTool := qualificationExecutable(t, "go")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	appRepository := filepath.Join(root, "application")
	testRepository := filepath.Join(root, "tests")
	providerPath := filepath.Join(root, "application-attestation-provider")
	project := fmt.Sprintf("corvint-attestation-%d", os.Getpid())
	containerName := project + "-app"
	imageName := project + ":fixture"
	compose := filepath.Join(appRepository, "compose.yml")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	prepareDockerApplicationFixture(t, ctx, goTool, git, appRepository)
	prepareDockerProvider(t, ctx, goTool, providerPath)
	preparePlaywrightRepository(t, git, modules, testRepository, filepath.Join(root, "playwright-output"))

	provisioningDone := make(chan struct{})
	var finishProvisioningOnce sync.Once
	finishProvisioning := func() { finishProvisioningOnce.Do(func() { close(provisioningDone) }) }
	defer finishProvisioning()
	cleanup := qualificationDockerCleanup(docker, containerName, imageName)
	t.Cleanup(func() { finishProvisioning(); cleanup() })
	qualificationCleanupAfterCancellation(ctx, provisioningDone, cleanup)

	validateComposeFixture(t, compose)
	provisionCtx, cancelProvision := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelProvision()
	qualificationRun(t, provisionCtx, docker, "build", "-t", imageName, appRepository)
	if ctx.Err() != nil {
		t.Fatal("qualification interrupted after Docker build")
	}
	container := qualificationOutput(t, provisionCtx, docker, "run", "-d", "--name", containerName, "--label", "com.docker.compose.project="+project, "-p", "127.0.0.1::8080", imageName)
	finishProvisioning()
	port := qualificationOutput(t, ctx, docker, "port", container, "8080/tcp")
	readyURL := "http://" + strings.TrimSpace(port) + "/health"
	waitQualificationReady(t, readyURL)
	waitQualificationContainerHealthy(t, docker, container)
	finishProvisioning()

	expectation := dockerExpectation(t, ctx, git, docker, appRepository, compose, container)
	configPath := filepath.Join(root, "application-attestation.json")
	writeCanonicalFixture(t, configPath, applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: expectation})
	providerArgv := []string{providerPath, "--docker", docker, "--git", git, "--repository", appRepository, "--compose", compose, "--container", container}
	runnerVersion := playwrightVersion(t, modules)
	t.Setenv("CORVINT_FIXTURE_URL", "http://"+strings.TrimSpace(port))
	runStarted := filepath.Join(root, "playwright-run-started")
	t.Setenv("CORVINT_RUN_STARTED", runStarted)
	cfg := dockerQualificationConfig(testRepository, providerArgv, configPath, readyURL, runnerVersion)

	receipt, err := RunE2E(ctx, cfg)
	if err != nil || receipt.Infrastructure != nil || receipt.Profile != AttestedExternalProfile || len(receipt.Tests) != 1 {
		t.Fatalf("qualified run err=%v infrastructure=%+v receipt=%+v", err, receipt.Infrastructure, receipt)
	}
	if applicationAttestationUnknown(receipt.ApplicationAttestation) || testRepositoryUnknown(receipt.TestRepositoryAtStart, receipt.TestRepositoryAtPublish) {
		t.Fatal("qualified run lost application or test-repository identity")
	}
	if ReceiptTestProjection(receipt, receipt.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified Docker-backed Playwright result did not project passed")
	}

	for name, row := range map[string]struct {
		mutate func(*ApplicationAttestationExpectation)
		want   string
	}{
		"healthy wrong revision": {func(value *ApplicationAttestationExpectation) { value.Repository.Revision = oid('f') }, "application-revision-mismatch"},
		"healthy wrong image":    {func(value *ApplicationAttestationExpectation) { value.Build.Digest = digest('e') }, "application-build-mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			wrong := expectation
			row.mutate(&wrong)
			wrongPath := filepath.Join(root, strings.ReplaceAll(name, " ", "-")+".json")
			writeCanonicalFixture(t, wrongPath, applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: wrong})
			wrongCfg := dockerQualificationConfig(testRepository, providerArgv, wrongPath, readyURL, runnerVersion)
			wrongReceipt, runErr := RunE2E(ctx, wrongCfg)
			if runErr != nil || wrongReceipt.Infrastructure == nil || wrongReceipt.Infrastructure.Reason != row.want || len(wrongReceipt.Tests) != 0 {
				t.Fatalf("wrong identity was not refused before Playwright: err=%v receipt=%+v", runErr, wrongReceipt)
			}
		})
	}

	t.Run("test repository drift", func(t *testing.T) {
		_ = os.Remove(runStarted)
		driftDone := make(chan error, 1)
		go func() {
			if err := waitQualificationFile(runStarted); err != nil {
				driftDone <- err
				return
			}
			driftDone <- os.WriteFile(filepath.Join(testRepository, "unbound.txt"), []byte("changed\n"), 0o600)
		}()
		drifted, runErr := RunE2E(ctx, cfg)
		if driftErr := <-driftDone; driftErr != nil {
			t.Fatal(driftErr)
		}
		if runErr != nil || drifted.ApplicationAttestation == nil || !slicesContain(drifted.ApplicationAttestation.Failures, "test-repository-dirty") || len(drifted.Tests) == 0 || ReceiptTestProjection(drifted, drifted.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
			t.Fatalf("test-repository drift was not retained as infrastructure: err=%v receipt=%+v", runErr, drifted)
		}
		if err := os.WriteFile(filepath.Join(testRepository, "unbound.txt"), []byte("original\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("restarted server", func(t *testing.T) {
		_ = os.Remove(runStarted)
		restartDone := make(chan error, 1)
		go func() {
			if err := waitQualificationFile(runStarted); err != nil {
				restartDone <- err
				return
			}
			command := exec.CommandContext(ctx, docker, "restart", container)
			restartDone <- command.Run()
		}()
		restarted, runErr := RunE2E(ctx, cfg)
		if restartErr := <-restartDone; restartErr != nil {
			t.Fatal(restartErr)
		}
		if runErr != nil || restarted.ApplicationAttestation == nil || !slicesContain(restarted.ApplicationAttestation.Failures, "application-restarted") || len(restarted.Tests) == 0 || ReceiptTestProjection(restarted, restarted.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
			t.Fatalf("restart was not retained as infrastructure: err=%v receipt=%+v", runErr, restarted)
		}
	})

	t.Run("cleanup waits for interrupted provisioning", func(t *testing.T) {
		interruptProject := project + "-interrupt"
		interruptContainer := interruptProject + "-app"
		interruptImage := interruptProject + ":fixture"
		provisioned := make(chan struct{})
		var finishOnce sync.Once
		finish := func() { finishOnce.Do(func() { close(provisioned) }) }
		defer finish()
		cleanup := qualificationDockerCleanup(docker, interruptContainer, interruptImage)
		t.Cleanup(func() { finish(); cleanup() })
		cancelCtx, cancel := context.WithCancel(context.Background())
		cleanupDone := qualificationCleanupAfterCancellation(cancelCtx, provisioned, cleanup)
		cancel()
		interruptProvisionCtx, cancelProvision := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelProvision()
		qualificationRun(t, interruptProvisionCtx, docker, "tag", imageName, interruptImage)
		qualificationRun(t, interruptProvisionCtx, docker, "run", "-d", "--name", interruptContainer, "--label", "com.docker.compose.project="+interruptProject, interruptImage)
		finish()
		select {
		case <-cleanupDone:
		case <-time.After(30 * time.Second):
			t.Fatal("interrupted qualification cleanup did not complete")
		}
		if remaining := strings.TrimSpace(qualificationOutput(t, context.Background(), docker, "ps", "-a", "--filter", "label=com.docker.compose.project="+interruptProject, "-q")); remaining != "" {
			t.Fatalf("interrupted qualification cleanup left containers %s", remaining)
		}
		if !qualificationDockerInventoryExcludes(docker, interruptImage, "image", "ls", "--format", "{{.Repository}}:{{.Tag}}") {
			t.Fatal("interrupted qualification cleanup did not confirm image absence")
		}
	})

	cleanup()
	remaining := strings.TrimSpace(qualificationOutput(t, context.Background(), docker, "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "-q"))
	if remaining != "" {
		t.Fatalf("qualification cleanup left containers %s", remaining)
	}
}

func TestQualificationDockerInventoryErrorsAreUnresolved(t *testing.T) {
	if qualificationDockerInventoryExcludes(filepath.Join(t.TempDir(), "missing-docker"), "fixture", "container", "ls") {
		t.Fatal("Docker inventory failure was treated as confirmed absence")
	}
}

func prepareDockerApplicationFixture(t *testing.T, ctx context.Context, goTool, git, repository string) {
	t.Helper()
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Dockerfile", "compose.yml"} {
		data, err := os.ReadFile(filepath.Join("testdata", "application-attestation", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repository, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	server := filepath.Join(repository, "fixture-server")
	command := exec.CommandContext(ctx, goTool, "build", "-trimpath", "-o", server, "./internal/jstestprovider/testdata/application-attestation/server")
	command.Dir = qualificationRepositoryRoot(t)
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture server: %v: %s", err, output)
	}
	qualificationRun(t, ctx, git, "-C", repository, "init", "-q")
	qualificationRun(t, ctx, git, "-C", repository, "config", "user.email", "fixture@example.invalid")
	qualificationRun(t, ctx, git, "-C", repository, "config", "user.name", "Corvint Fixture")
	qualificationRun(t, ctx, git, "-C", repository, "add", "Dockerfile", "compose.yml", "fixture-server")
	qualificationRun(t, ctx, git, "-C", repository, "commit", "-q", "-m", "fixture")
}

func validateComposeFixture(t *testing.T, path string) {
	t.Helper()
	data := readQualificationFile(t, path)
	var document struct {
		Services struct {
			App struct {
				Build struct {
					Context string `json:"context"`
				} `json:"build"`
				Healthcheck struct {
					Test     []string `json:"test"`
					Interval string   `json:"interval"`
					Timeout  string   `json:"timeout"`
					Retries  int      `json:"retries"`
				} `json:"healthcheck"`
				Ports []string `json:"ports"`
			} `json:"app"`
		} `json:"services"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	canonical, _ := json.Marshal(document)
	canonical = append(canonical, '\n')
	app := document.Services.App
	if !bytes.Equal(data, canonical) || app.Build.Context != "." || len(app.Healthcheck.Test) != 3 || app.Healthcheck.Test[1] != "/fixture-server" || app.Healthcheck.Interval != "250ms" || app.Healthcheck.Timeout != "1s" || app.Healthcheck.Retries != 20 || len(app.Ports) != 1 || app.Ports[0] != "127.0.0.1::8080" {
		t.Fatal("Compose fixture is outside the qualified closed subset")
	}
}

func prepareDockerProvider(t *testing.T, ctx context.Context, goTool, output string) {
	t.Helper()
	command := exec.CommandContext(ctx, goTool, "build", "-trimpath", "-o", output, "./internal/jstestprovider/testdata/application-attestation/provider")
	command.Dir = qualificationRepositoryRoot(t)
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture provider: %v: %s", err, data)
	}
}

func preparePlaywrightRepository(t *testing.T, git, modules, repository, outputDir string) {
	t.Helper()
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	use := "{browserName: 'chromium'}"
	if playwrightVersion(t, modules) == "1.63.0" {
		use = "{browserName: 'chromium', channel: '', launchOptions: {executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'}}"
	}
	config := "module.exports = {testDir: '.', outputDir: " + jsonString(outputDir) + ", projects: [{name: 'chromium', use: " + use + "}]};\n"
	test := "const fs = require('fs');\nconst { test, expect } = require('@playwright/test');\ntest('attested application', async ({page}) => { fs.writeFileSync(process.env.CORVINT_RUN_STARTED, 'started\\n'); await page.waitForTimeout(2000); await page.goto(process.env.CORVINT_FIXTURE_URL); await expect(page.locator('body')).toContainText('attested fixture'); });\n"
	if err := os.WriteFile(filepath.Join(repository, "playwright.config.cjs"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "application.spec.cjs"), []byte(test), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "unbound.txt"), []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(modules, filepath.Join(repository, "node_modules")); err != nil {
		t.Fatal(err)
	}
	qualificationRun(t, context.Background(), git, "-C", repository, "init", "-q")
	qualificationRun(t, context.Background(), git, "-C", repository, "config", "user.email", "fixture@example.invalid")
	qualificationRun(t, context.Background(), git, "-C", repository, "config", "user.name", "Corvint Fixture")
	qualificationRun(t, context.Background(), git, "-C", repository, "add", "playwright.config.cjs", "application.spec.cjs", "unbound.txt", "node_modules")
	qualificationRun(t, context.Background(), git, "-C", repository, "commit", "-q", "-m", "fixture")
}

func dockerExpectation(t *testing.T, ctx context.Context, git, docker, repository, compose, container string) ApplicationAttestationExpectation {
	t.Helper()
	return ApplicationAttestationExpectation{
		Repository: ApplicationRepositoryExpectation{
			RootCommit:  qualificationOutput(t, ctx, git, "-C", repository, "rev-list", "--max-parents=0", "HEAD"),
			Revision:    qualificationOutput(t, ctx, git, "-C", repository, "rev-parse", "HEAD"),
			Tree:        qualificationOutput(t, ctx, git, "-C", repository, "rev-parse", "HEAD^{tree}"),
			DirtyPolicy: "require-clean",
		},
		Build:         ApplicationArtifactIdentity{Kind: "image", Digest: qualificationOutput(t, ctx, docker, "inspect", "--format", "{{.Image}}", container)},
		Configuration: ApplicationArtifactIdentity{Kind: "compose", Digest: "sha256:" + sha256Hex(readQualificationFile(t, compose))},
		InstanceKind:  "docker-container",
	}
}

func dockerQualificationConfig(repository string, providerArgv []string, configPath, readyURL, runnerVersion string) E2EConfig {
	config := filepath.Join(repository, "playwright.config.cjs")
	test := filepath.Join(repository, "application.spec.cjs")
	return E2EConfig{
		Config:         Config{Dir: repository, ConfigFile: config, TestFiles: []string{test}, RunnerName: "playwright", RunnerVersion: runnerVersion, DeclaredEnvKeys: []string{"CORVINT_FIXTURE_URL", "CORVINT_RUN_STARTED"}, Timeout: 45 * time.Second},
		ExternalServer: true, ServerReadyURL: readyURL, ServerReadyLimit: 15 * time.Second, TestArgv: []string{"application.spec.cjs", "--project=chromium"},
		ApplicationAttestation: &ApplicationAttestationProvider{Argv: providerArgv, ConfigFile: configPath, Timeout: 10 * time.Second},
	}
}

func playwrightVersion(t *testing.T, modules string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(modules, "@playwright", "test", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		t.Fatal("invalid Playwright package metadata")
	}
	return pkg.Version
}

func waitQualificationReady(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(address)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Docker fixture did not become ready")
}

func waitQualificationContainerHealthy(t *testing.T, docker, container string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		command := exec.Command(docker, "inspect", "--format", "{{.State.Health.Status}}", container)
		output, err := command.Output()
		if err == nil && strings.TrimSpace(string(output)) == "healthy" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Docker fixture did not report healthy")
}

func waitQualificationFile(path string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", path)
}

func qualificationRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func qualificationExecutable(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("NOT_RUN: %s is unavailable", name)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func qualificationRun(t *testing.T, ctx context.Context, command string, arguments ...string) {
	t.Helper()
	if output, err := exec.CommandContext(ctx, command, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v: %s", command, arguments, err, output)
	}
}

func qualificationOutput(t *testing.T, ctx context.Context, command string, arguments ...string) string {
	t.Helper()
	return strings.TrimSpace(string(qualificationRaw(t, ctx, command, arguments...)))
}

func qualificationRaw(t *testing.T, ctx context.Context, command string, arguments ...string) []byte {
	t.Helper()
	output, err := exec.CommandContext(ctx, command, arguments...).Output()
	if err != nil {
		t.Fatalf("%s %v: %v", command, arguments, err)
	}
	return output
}

func readQualificationFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func jsonString(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func slicesContain(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func qualificationCleanupAfterCancellation(ctx context.Context, provisioningDone <-chan struct{}, cleanup func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		<-provisioningDone
		cleanup()
	}()
	return done
}

func qualificationDockerCleanup(docker, container, image string) func() {
	var mutex sync.Mutex
	cleaned := false
	return func() {
		mutex.Lock()
		defer mutex.Unlock()
		if !cleaned {
			cleaned = qualificationCleanupDocker(docker, container, image)
		}
	}
}

func qualificationCleanupDocker(docker, container, image string) bool {
	deadline := time.Now().Add(10 * time.Second)
	var absentSince time.Time
	for {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = exec.CommandContext(cleanupCtx, docker, "rm", "-f", container).Run()
		_ = exec.CommandContext(cleanupCtx, docker, "image", "rm", "-f", image).Run()
		cancel()
		containerGone := qualificationDockerInventoryExcludes(docker, container, "container", "ls", "-a", "--format", "{{.Names}}")
		imageGone := qualificationDockerInventoryExcludes(docker, image, "image", "ls", "--format", "{{.Repository}}:{{.Tag}}")
		if containerGone && imageGone {
			if absentSince.IsZero() {
				absentSince = time.Now()
			} else if time.Since(absentSince) >= 2*time.Second {
				return true
			}
		} else {
			absentSince = time.Time{}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func qualificationDockerInventoryExcludes(docker, target string, arguments ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, docker, arguments...).Output()
	if err != nil || ctx.Err() != nil {
		return false
	}
	for _, value := range strings.Fields(string(output)) {
		if value == target {
			return false
		}
	}
	return true
}
