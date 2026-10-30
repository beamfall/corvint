//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

func TestCancellingAuthorityCleansNestedGoProcessGroup(t *testing.T) {
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	temporary := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	goRoot := filepath.Join(base, "goroot")
	toolDirectory := filepath.Join(goRoot, "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH)
	for _, path := range []string{repository, temporary, moduleCache, filepath.Join(goRoot, "bin"), filepath.Join(goRoot, "src", "runtime"), toolDirectory} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(repository, "go.mod"):                   "module example.test/nested\n\ngo 1.27.0\n",
		filepath.Join(repository, "nested_test.go"):           "package nested\n",
		filepath.Join(goRoot, "src", "runtime", "runtime.go"): "package runtime\n",
		filepath.Join(toolDirectory, "asm"):                   "asm",
		filepath.Join(toolDirectory, "compile"):               "compile",
		filepath.Join(toolDirectory, "link"):                  "link",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	verifier, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	verifier = resolvedPath(t, verifier)
	goExecutable := filepath.Join(goRoot, "bin", "corvint-nested-go")
	verifierBytes, err := os.ReadFile(verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goExecutable, verifierBytes, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(moduleCache, 0o700) })
	gitExecutable := resolvedGitExecutable(t)
	runFixtureGit(t, gitExecutable, repository, "init", "-q")
	runFixtureGit(t, gitExecutable, repository, "add", "go.mod", "nested_test.go")
	runFixtureGit(t, gitExecutable, repository, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	verifierSHA, err := regularFileDigest(verifier)
	if err != nil {
		t.Fatal(err)
	}
	gitSHA, err := regularFileDigest(gitExecutable)
	if err != nil {
		t.Fatal(err)
	}
	bundle := authorityBundle{
		Profile: attachmentProfile, VerifierExecutable: verifier, VerifierExecutableSHA256: verifierSHA,
		GitExecutable: gitExecutable, GitExecutableSHA256: gitSHA, GoExecutable: goExecutable,
		GOARCH: runtime.GOARCH, GOOS: runtime.GOOS, GOROOT: goRoot, ModuleCacheDirectory: moduleCache,
		ModuleMode: string(provider.ModuleReadonly), OutputLimitBytes: gorunner.MaxOutputBytes,
		Packages: []string{"example.test/nested"}, RepositoryRoot: repository, TemporaryParent: temporary,
		TimeoutMilliseconds: gorunner.MaxRunTime.Milliseconds(),
	}
	request := provider.AuthorityRequest{
		RepositoryRoot: repository, WorkingDirectory: repository, GoExecutable: goExecutable, GOROOT: goRoot,
		ExpectedGoVersion: provider.GoVersion, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		CGOEnabled: "0", EnvironmentProfile: provider.EnvironmentProfile,
		ModuleMode: provider.ModuleReadonly, Packages: []string{"example.test/nested"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		_, acquireErr := (&localAuthority{bundle: bundle, productionVerifier: true}).Acquire(ctx, request)
		done <- acquireErr
	}()
	waitForDirectCommandFile(t, goExecutable+".ready")
	waitForDirectCommandFile(t, goExecutable+".descendant-ready")
	leaderPID := readDirectCommandPID(t, goExecutable+".ready")
	descendantPID := readDirectCommandPID(t, goExecutable+".descendant-ready")
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled authority acquisition succeeded")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancelled authority acquisition did not return within 30s")
	}
	assertDirectCommandProcessGone(t, leaderPID)
	assertDirectCommandProcessGone(t, descendantPID)
}
