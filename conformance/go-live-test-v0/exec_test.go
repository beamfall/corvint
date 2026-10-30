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

func TestMain(m *testing.M) {
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
