//go:build darwin || linux

package jstestprovider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestApplicationAttestationRelocatedLauncherFailsClosed pins the documented
// exclusion of PWP-V1-009 (V1-0679): the provider executable is staged as a
// lone content copy, so a launcher that finds its runtime beside itself (as
// Homebrew Node finds @rpath/libnode.*.dylib) runs from its original path but
// not from the staged copy, and the staged run is an
// application-attestation-provider-unavailable reason with no observation,
// never a passing one.
func TestApplicationAttestationRelocatedLauncherFailsClosed(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "runtime"), []byte("#!/bin/sh\ncat \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(bin, "launcher")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexec \"$(dirname \"$0\")/runtime\" \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.json")
	statePath := filepath.Join(root, "state.json")
	writeCanonicalFixture(t, configPath, applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: applicationExpectationFixture()})
	writeCanonicalFixture(t, statePath, applicationAttestationFixture())
	if output, err := exec.Command(launcher, statePath).Output(); err != nil || len(output) == 0 {
		t.Fatalf("original launcher must run in place: err=%v output=%q", err, output)
	}
	scratch := filepath.Join(root, "scratch")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	provider, err := prepareApplicationAttestationProvider(ApplicationAttestationProvider{Argv: []string{launcher, statePath}, ConfigFile: configPath}, root, scratch, map[string]string{})
	if err != nil {
		t.Fatalf("prepare refused a readable launcher: %v", err)
	}
	if provider.receipt.Provider.ExecutablePath != launcher {
		t.Fatalf("receipt names %q, want the original launcher %q", provider.receipt.Provider.ExecutablePath, launcher)
	}
	observation, failure := provider.observe(context.Background())
	if observation != nil || failure != "application-attestation-provider-unavailable" {
		t.Fatalf("staged relocated launcher: observation=%+v failure=%q, want no observation and application-attestation-provider-unavailable", observation, failure)
	}
}
