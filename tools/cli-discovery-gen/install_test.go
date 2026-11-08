// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/versionline"
)

func TestTCLV07703ReleaseLine(t *testing.T) {
	version, err := os.ReadFile("../../VERSION")
	if err != nil || strings.TrimSpace(string(version)) != versionline.Release || versionline.DevelopmentVersion(versionline.Release) != versionline.Development {
		t.Fatalf("release pin: %q %v", version, err)
	}
}

func TestTCLV07703CompleteBoundedInstall(t *testing.T) {
	entries, err := os.ReadDir("../../cmd")
	if err != nil {
		t.Fatal(err)
	}
	source, dest := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := execfile.WriteFile(filepath.Join(source, e.Name()), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	install := func() {
		t.Helper()
		c := exec.Command("bash", "../../script/install-local-bin.sh", "--all", source, dest)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("install all: %v %s", err, out)
		}
	}
	for i := 0; i < 3; i++ {
		install()
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(dest, e.Name())); err != nil || info.Mode()&0111 == 0 {
			t.Errorf("%s missing/nonexecutable: %v", e.Name(), err)
		}
		backups, err := filepath.Glob(filepath.Join(dest, e.Name()+".corvint-prev-*"))
		if err != nil || len(backups) != 1 {
			t.Errorf("%s backups=%v %v", e.Name(), backups, err)
		}
	}
	// Validate all inputs before replacing an installed file.
	if err := os.Remove(filepath.Join(source, "corvint-test")); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("bash", "../../script/install-local-bin.sh", "--all", source, dest)
	if out, err := c.CombinedOutput(); err == nil || !strings.Contains(string(out), "missing-binary") {
		t.Fatalf("incomplete install accepted: %v %s", err, out)
	}
}
