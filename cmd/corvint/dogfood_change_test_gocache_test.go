package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// V1-3918: script/dogfood-change_test.sh builds its driver with the caller's Go
// cache (CORVINT_GOCACHE, then GOCACHE) and falls back to the shared
// /tmp/corvint-go-build-cache only when neither is set, so a lane whose /tmp is
// full keeps the build off it. A copy of the script runs in a minimal tree with
// a fake go that records the GOCACHE it was given and stops the run.
func TestDogfoodChangeTestHonoursTheCallerGoCache(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("dogfood coordinator is a POSIX shell contract")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, dir := range []string{"script", "docs", "bin", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"dogfood-change_test.sh", "test-rg-fallback.sh"} {
		body, err := os.ReadFile(filepath.Join(moduleRoot(t), "script", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(filepath.Join(root, "script", name), body, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	doc := "corvint ocm report --map .corvint/change.ocm.001.json\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "DOGFOOD.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, "gocache.txt")
	fakeGo := "#!/bin/sh\nprintf '%s' \"${GOCACHE-UNSET}\" > " + record + "\nexit 3\n"
	if err := execfile.WriteFile(filepath.Join(root, "bin", "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	var base []string
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "GOCACHE", "CORVINT_GOCACHE", "TMPDIR", "PATH":
		default:
			base = append(base, kv)
		}
	}
	base = append(base, "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+filepath.Join(root, "tmp"))
	elsewhere, corvintCache := filepath.Join(root, "gocache"), filepath.Join(root, "corvint-gocache")
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"GOCACHE", []string{"GOCACHE=" + elsewhere}, elsewhere},
		{"CORVINT_GOCACHE wins", []string{"GOCACHE=" + elsewhere, "CORVINT_GOCACHE=" + corvintCache}, corvintCache},
		{"neither set", nil, "/tmp/corvint-go-build-cache"},
	} {
		_ = os.Remove(record)
		argv := []string{bash, filepath.Join(root, "script", "dogfood-change_test.sh")}
		result := procgroup.Run(t.Context(), procgroup.Spec{
			Argv: argv, Dir: root, Env: append(append([]string(nil), base...), tc.env...),
			Timeout: time.Minute, OutputLimit: 1 << 16,
		})
		got, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("%s: the fake go never ran: %v\n%s", tc.name, err, shellTestFailure(argv, result))
		}
		if string(got) != tc.want {
			t.Fatalf("%s: driver build GOCACHE = %q, want %q", tc.name, got, tc.want)
		}
		if result.ExitStatus == 0 {
			t.Fatalf("%s: the run went on past the stopped build", tc.name)
		}
	}
}
