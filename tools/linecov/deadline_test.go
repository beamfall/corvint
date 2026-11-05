// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// LPCV-V0-7603: qualification passes exactly the recorded deadline to mutants.
func TestQualificationDeadline(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	t.Setenv("DEADLINE_LOG", log)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DEADLINE_LOG\"\nprintf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"m/p\"}'\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, _, err := failingTests(dir, "m", []string{"p"}, 17*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "-timeout=17s\n") {
		t.Fatalf("deadline lost: %s", b)
	}
	if _, _, err = failingTests(dir, "m", []string{"p"}, 0); err == nil {
		t.Fatal("unknown deadline admitted")
	}
}
