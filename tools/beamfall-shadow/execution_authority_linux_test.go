//go:build linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLinuxSubreaperAuthorityIsEstablishedAndVerifiedInProcess(t *testing.T) {
	if err := establishExecutionAuthority(); err != nil {
		t.Fatalf("linux child-subreaper authority unavailable: %v", err)
	}
}

func TestLinuxSubreaperProbeObservesAbsentAuthority(t *testing.T) {
	if os.Getenv("SHADOW_SUBREAPER_ABSENT_HELPER") == "1" {
		if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetChildSubreaper, 0, 0, 0, 0, 0); errno != 0 {
			os.Exit(2)
		}
		enabled, err := linuxSubreaperEnabled()
		if err != nil || enabled {
			os.Exit(3)
		}
		os.Exit(0)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestLinuxSubreaperProbeObservesAbsentAuthority$")
	command.Env = append(os.Environ(), "SHADOW_SUBREAPER_ABSENT_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("absent subreaper probe: %v (%s)", err, output)
	}
}

func TestLinuxMissingSubreaperAuthorityFailsBeforeCandidateStart(t *testing.T) {
	original := linuxPrctl
	linuxPrctl = func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno) {
		return 0, 0, syscall.EPERM
	}
	t.Cleanup(func() { linuxPrctl = original })
	marker := filepath.Join(t.TempDir(), "candidate-ran")
	stub := fixtureScript(t, `#!/bin/sh
printf candidate > "`+marker+`"
`)
	manifestPath := fixtureManifest(t, supportedDogfoodTuples[0], stub)
	manifest, raw, err := readDogfoodManifest(context.Background(), manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := stageExternalArtifact(context.Background(), manifest, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := guard.Close(); err != nil {
			t.Error(err)
		}
	})
	request, err := rawRequest(target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{}}, strings.Repeat("a", 40), "go", nil)
	if err != nil {
		t.Fatal(err)
	}
	runDir := t.TempDir()
	result := invoke(context.Background(), "never-started", runDir, request, time.Second, "missing-subreaper", guard, manifest.identity(raw, "sha256:"+strings.Repeat("0", 64)))
	if result.Status != "UNSUPPORTED_EXECUTION_AUTHORITY" || result.ProcessGroup != "NOT_RUN" {
		t.Fatalf("missing subreaper admitted candidate: result=%#v", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("missing subreaper started candidate: %v", err)
	}
}
