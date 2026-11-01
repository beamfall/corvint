// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var providerBuild struct {
	once       sync.Once
	executable string
	directory  string
	err        error
	output     []byte
}

// TestMain detaches every git the tests spawn from the account's global and
// system configuration, as testsupport.IsolateGit does (V1-2635); this
// conformance package imports no Corvint package, so the list is inline.
func TestMain(m *testing.M) {
	for _, variable := range [][2]string{
		{"GIT_CONFIG_GLOBAL", os.DevNull},
		{"GIT_CONFIG_NOSYSTEM", "1"},
		{"GIT_CONFIG_PARAMETERS", "'maintenance.auto'='false' 'gc.auto'='0'"},
		{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.test"},
		{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.test"},
	} {
		if err := os.Setenv(variable[0], variable[1]); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	if providerBuild.directory != "" {
		_ = os.RemoveAll(providerBuild.directory)
	}
	os.Exit(code)
}

func testProviderExecutable(t *testing.T, root string) string {
	t.Helper()
	providerBuild.once.Do(func() {
		providerBuild.directory, providerBuild.err = os.MkdirTemp("", "go-live-test-v0-")
		if providerBuild.err != nil {
			return
		}
		providerBuild.executable = filepath.Join(providerBuild.directory, "corvint-go-test-provider")
		build := exec.Command("go", "build", "-trimpath", "-o", providerBuild.executable, "./cmd/corvint-go-test-provider")
		build.Dir = root
		build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
		providerBuild.output, providerBuild.err = build.CombinedOutput()
	})
	if providerBuild.err != nil {
		t.Fatalf("build provider: %v: %s", providerBuild.err, providerBuild.output)
	}
	return providerBuild.executable
}

func testCommand(t *testing.T, root, output string) *exec.Cmd {
	t.Helper()
	command := exec.Command("go", "build", "-trimpath", "-o", output, "./conformance/go-live-test-v0")
	command.Dir = root
	return command
}
