//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type blackBoxAuthorityBundle struct {
	Profile                  string   `json:"profile"`
	VerifierExecutable       string   `json:"verifierExecutable"`
	VerifierExecutableSHA256 string   `json:"verifierExecutableRawSha256"`
	GitExecutable            string   `json:"gitExecutable"`
	GitExecutableSHA256      string   `json:"gitExecutableRawSha256"`
	GoExecutable             string   `json:"goExecutable"`
	GoWorkPath               string   `json:"goWorkPath"`
	GOARCH                   string   `json:"goarch"`
	GOOS                     string   `json:"goos"`
	GOROOT                   string   `json:"goroot"`
	ModuleCacheDirectory     string   `json:"moduleCacheDirectory"`
	ModuleMode               string   `json:"moduleMode"`
	OutputLimitBytes         int64    `json:"outputLimitBytes"`
	Packages                 []string `json:"packages"`
	RepositoryRoot           string   `json:"repositoryRoot"`
	TemporaryParent          string   `json:"temporaryParent"`
	TimeoutMilliseconds      int64    `json:"timeoutMilliseconds"`
}

func TestProductionProviderBlackBoxAcquisitionAndChildExecution(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("GLTP-V0-049: the provider executes only on darwin/arm64")
	}
	root := repositoryRoot(t)
	base := resolvedTemporary(t)
	providerExecutable := testProviderExecutable(t, root)
	repository := filepath.Join(base, "repository")
	temporary := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	for _, path := range []string{repository, temporary, moduleCache} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.test/blackbox\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "blackbox_test.go"), []byte("package blackbox\nimport (\"fmt\"; \"testing\")\nfunc TestBlackBox(t *testing.T) { fmt.Println(\"CORVINT_BLACKBOX_CHILD\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitExecutable := blackBoxGit(t)
	blackBoxGitRun(t, gitExecutable, repository, "init", "-q")
	blackBoxGitRun(t, gitExecutable, repository, "add", "go.mod", "blackbox_test.go")
	blackBoxGitRun(t, gitExecutable, repository, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	if err := os.Chmod(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(moduleCache, 0o700) })
	goRoot, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	goExecutable := filepath.Join(goRoot, "bin", "go")
	providerSHA := blackBoxFileSHA(t, providerExecutable)
	bundle := blackBoxAuthorityBundle{
		Profile: "corvint-go-live-authority-attachment/0", VerifierExecutable: providerExecutable,
		VerifierExecutableSHA256: providerSHA, GitExecutable: gitExecutable,
		GitExecutableSHA256: blackBoxFileSHA(t, gitExecutable), GoExecutable: goExecutable,
		GOARCH: runtime.GOARCH, GOOS: runtime.GOOS, GOROOT: goRoot, ModuleCacheDirectory: moduleCache,
		ModuleMode: "MODULE_READONLY", OutputLimitBytes: 8 << 20, Packages: []string{"example.test/blackbox"},
		RepositoryRoot: repository, TemporaryParent: temporary, TimeoutMilliseconds: 1_800_000,
	}
	bundleBytes, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(base, "bundle.json")
	if err := os.WriteFile(bundlePath, bundleBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	ordinaryRoot := filepath.Join(base, "ordinary")
	for _, name := range []string{"cache", "tmp", "home"} {
		if err := os.MkdirAll(filepath.Join(ordinaryRoot, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ordinaryEnv := []string{
		"CGO_ENABLED=0", "GOARCH=" + runtime.GOARCH, "GOCACHE=" + filepath.Join(ordinaryRoot, "cache"), "GOENV=off",
		"GOFLAGS=-mod=readonly", "GOMODCACHE=" + moduleCache, "GONOPROXY=", "GONOSUMDB=", "GOOS=" + runtime.GOOS,
		"GOPRIVATE=", "GOPROXY=off",
		"GOROOT=" + goRoot, "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTMPDIR=" + filepath.Join(ordinaryRoot, "tmp"),
		"GOVCS=*:off", "GOWORK=off", "HOME=" + filepath.Join(ordinaryRoot, "home"), "TEMP=" + filepath.Join(ordinaryRoot, "tmp"),
		"TMP=" + filepath.Join(ordinaryRoot, "tmp"), "TMPDIR=" + filepath.Join(ordinaryRoot, "tmp"),
	}
	var group sync.WaitGroup
	var listed, ordinary []byte
	var ordinaryErr error
	var transcript []byte
	var providerErr error
	group.Add(2)
	go func() {
		defer group.Done()
		listed, ordinaryErr = blackBoxCommand(goExecutable, repository, ordinaryEnv, "list", "-deps", "-test", "-json="+closedListFields, "example.test/blackbox")
		if ordinaryErr == nil && bytes.Contains(listed, []byte(`"ImportPath": "example.test/blackbox"`)) {
			ordinary, ordinaryErr = blackBoxCommand(goExecutable, repository, ordinaryEnv, "test", "-json", "-count=1", "-vet=off", "example.test/blackbox")
		}
	}()
	go func() {
		defer group.Done()
		command := exec.Command(providerExecutable, "--experimental", "--trusted-local", "--authority-bundle", bundlePath)
		command.Env = []string{}
		transcript, providerErr = command.CombinedOutput()
	}()
	group.Wait()
	if ordinaryErr != nil {
		t.Fatal(ordinaryErr)
	}
	if !bytes.Contains(listed, []byte(`"ImportPath": "example.test/blackbox"`)) {
		t.Fatalf("ordinary acquisition = %q", listed)
	}
	if !bytes.Contains(ordinary, []byte("CORVINT_BLACKBOX_CHILD")) {
		t.Fatalf("ordinary child output missing: %s", ordinary)
	}
	if providerErr != nil {
		t.Fatalf("provider: %v: %s", providerErr, transcript)
	}
	verifyBlackBoxTranscript(t, transcript)
}

func verifyBlackBoxTranscript(t *testing.T, transcript []byte) {
	t.Helper()
	lines := bytes.Split(bytes.TrimSuffix(transcript, []byte{'\n'}), []byte{'\n'})
	if len(lines) < 2 {
		t.Fatalf("short transcript: %s", transcript)
	}
	wantOutput := sha256.Sum256([]byte("CORVINT_BLACKBOX_CHILD\n"))
	foundOutput := false
	for _, line := range lines[:len(lines)-1] {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		fact, _ := event["fact"].(map[string]any)
		output, _ := fact["output"].(map[string]any)
		if output["sha256"] == hex.EncodeToString(wantOutput[:]) {
			foundOutput = true
		}
	}
	var terminal map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &terminal); err != nil {
		t.Fatal(err)
	}
	execution, _ := terminal["execution"].(map[string]any)
	scope, _ := terminal["scope"].(map[string]any)
	terminals, _ := scope["packageTerminals"].([]any)
	foundPackage := false
	for _, value := range terminals {
		row, _ := value.(map[string]any)
		if row["package"] == "example.test/blackbox" && row["status"] == "PASSED" {
			foundPackage = true
		}
	}
	if terminal["profile"] != "go-live-run/0" || execution["status"] != "PASSED" || !foundPackage || !foundOutput {
		t.Fatalf("black-box evidence missing: terminal=%v output=%t", terminal, foundOutput)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}

func resolvedTemporary(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}

func blackBoxGit(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		command := exec.Command("xcrun", "--find", "git")
		command.Env = []string{"PATH=/usr/bin:/bin"}
		if output, err := command.Output(); err == nil {
			return blackBoxResolved(t, strings.TrimSpace(string(output)))
		}
	}
	path, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	return blackBoxResolved(t, path)
}

func blackBoxResolved(t *testing.T, path string) string {
	t.Helper()
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}

func blackBoxGitRun(t *testing.T, executable, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command(executable, append([]string{"-C", repository}, arguments...)...)
	command.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "LC_ALL=C"}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v: %s", arguments, err, output)
	}
}

func blackBoxFileSHA(t *testing.T, path string) string {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func blackBoxCommand(executable, cwd string, environment []string, arguments ...string) ([]byte, error) {
	command := exec.Command(executable, arguments...)
	command.Dir = cwd
	command.Env = append([]string(nil), environment...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err != nil || stderr.Len() != 0 {
		return nil, fmt.Errorf("command %q: %v: stdout=%s stderr=%s", arguments, err, stdout.Bytes(), stderr.Bytes())
	}
	return stdout.Bytes(), nil
}
