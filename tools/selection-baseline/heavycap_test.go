// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// DTL-V0-017: overrides and invalid values cannot accidentally widen a slot's worker share.
func TestDTLV0017ReplayJobs(t *testing.T) {
	for _, tc := range []struct {
		jobs, gmp, heavy string
		n, want          int
	}{
		{"", "2", "7", 20, 2}, {"", "", "3", 20, 3}, {"5", "2", "3", 20, 5},
		{"5", "2", "3", 2, 2}, {"bad", "2", "3", 20, 2}, {"0", "0", "3", 20, 3},
		{"", "", "", 20, max(1, min(defaultReplayJobs(20, runtime.NumCPU(), ""), memoryJobs()))},
		{"1", "2", "3", 0, 1},
	} {
		t.Setenv("SELECTION_BASELINE_JOBS", tc.jobs)
		t.Setenv("GOMAXPROCS", tc.gmp)
		t.Setenv("HEAVY_P", tc.heavy)
		want := tc.want
		if v, err := strconv.Atoi(tc.jobs); err != nil || v <= 0 {
			want = max(1, min(want, memoryJobs())) // ASD-V0-925 memory bound on every default
		}
		if got := replayJobs(tc.n); got != want {
			t.Errorf("%+v: jobs=%d", tc, got)
		}
	}
}

// DTL-V0-017: run the actual replay workers and compare both JSON and ordered diagnostics.
func TestDTLV0017ReplayBytesIndependentOfWorkers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixtures")
	}
	root := t.TempDir()
	write := func(name, body string, mode os.FileMode) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(bytes.TrimSpace(out))
	}
	git("init", "-q")
	write("go.mod", "module m\n", 0600)
	write(".github/cishards/package-costs.json", `{"milliseconds":{"m/a":30,"m/b":70}}`, 0600) //corvint:fixture-path
	write(".github/forward-rows/github-guard.sh", "echo 0\n", 0600)                            //corvint:fixture-path
	write("pkg", "a\n", 0600)
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	set := changeSet{Floor: floor{NarrowMerged: 2}}
	for i, pkg := range []string{"b", "a", "b"} {
		write("pkg", pkg+"\n", 0600)
		write("sequence", string(rune('1'+i)), 0600)
		git("add", ".")
		git("commit", "-qm", "change")
		c := change{ID: string(rune('a' + i)), Kind: "merged", Base: base, Head: git("rev-parse", "HEAD")}
		if i == 2 {
			c.Kind = "failed-ci"
			c.Failures = []failure{{Package: "m/a"}}
		}
		set.Changes = append(set.Changes, c)
	}
	// Executables live outside the clones; git clean must not remove them.
	bin := t.TempDir()
	corvint := filepath.Join(bin, "corvint")
	body := `#!/bin/sh
pkg=$(cat pkg)
case "$1" in
 --version) echo 'fixture 1' ;;
 affected) printf '{"plan":{"scope":"BOUNDED","selected":[{"unitId":"go:m/%s","witness":{"kind":"direct"}}]}}\n' "$pkg" ;;
 test) m=; prev=; for a; do [ "$prev" = --manifest ] && m=$a; prev=$a; done; printf '{"scope":"BOUNDED","units":[{"adapter":"go","unit":"go:m/%s","slice":"whole"}]}\n' "$pkg" > "$m" ;;
 *) exit 9 ;;
esac
`
	if err := execfile.WriteFile(corvint, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(bin, "selector")
	if err := execfile.WriteFile(selector, []byte("#!/bin/sh\nprintf 'run m/%s\\n' \"$(cat pkg)\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SELECTION_BASELINE_CACHE", "off") // every worker count replays; a cached REUSE would differ
	var wantJSON, wantLog []byte
	for _, jobs := range []string{"1", "2", "3", "8"} {
		t.Setenv("SELECTION_BASELINE_JOBS", jobs)
		log, err := os.CreateTemp(t.TempDir(), "stderr")
		if err != nil {
			t.Fatal(err)
		}
		old := os.Stderr
		os.Stderr = log
		result, err := replay(context.Background(), root, set, "changes.json", corvint, selector)
		os.Stderr = old
		log.Close()
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		diagnostics, err := os.ReadFile(log.Name())
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary.Misses != 1 || len(result.Results) != 3 {
			t.Fatalf("lost replay evidence: %+v", result)
		}
		if wantJSON == nil {
			wantJSON, wantLog = data, diagnostics
		} else if !bytes.Equal(data, wantJSON) || !bytes.Equal(diagnostics, wantLog) {
			t.Fatalf("workers=%s changed output\n%s\n%s", jobs, data, diagnostics)
		}
	}
}
