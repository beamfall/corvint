//go:build darwin || linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/execfile"
)

// commitFixture adds files (a value starting with "->" is a symlink target)
// to the fixture's target commit, or removes them when the value is empty.
func commitFixture(t *testing.T, o *options, files map[string]string) {
	t.Helper()
	for name, body := range files {
		file := filepath.Join(o.root, name)
		if err := os.RemoveAll(file); err != nil {
			t.Fatal(err)
		}
		if body == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		if target, ok := strings.CutPrefix(body, "->"); ok {
			err = os.Symlink(target, file)
		} else {
			err = os.WriteFile(file, []byte(body), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	for _, args := range [][]string{{"add", "-A", "."}, {"-c", "gc.auto=0", "-c", "maintenance.auto=false", "commit", "--amend", "--no-edit", "-q"}} {
		if _, err := git(ctx, *o, args...); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	if o.target, err = git(ctx, *o, "rev-parse", "HEAD"); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-9012: FULL runs a failing test that lives only in a hidden package,
// only in a symlinked _test.go, or only in a symlinked _test.go of a hidden
// package; Go's own ./... still covers the rest.
func TestFullExecutionHiddenAndSymlinked_AFPV09012(t *testing.T) {
	o, _ := fixture(t)
	commitFixture(t, &o, map[string]string{"a/a_test.go": "package a\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n"})
	ctx := context.Background()
	full := selection{Reason: "planner unavailable", Packages: []string{"./..."}}
	if code, err := execute(ctx, o, full); err != nil || code != 0 {
		t.Fatalf("passing baseline: %d %v", code, err)
	}
	failing := "package %s\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"%s failure\") }\n"
	for _, c := range []struct {
		name, pkg string
		files     map[string]string
	}{
		{"hidden", ".hidden", map[string]string{".hidden/h_test.go": strings.ReplaceAll(failing, "%s", "hidden")}},
		{"symlink-only", "linked", map[string]string{"testdata/fail_test.go": strings.ReplaceAll(failing, "%s", "linked"), "linked/fail_test.go": "->../testdata/fail_test.go"}},
		{"hidden symlink-only", ".linked", map[string]string{"testdata/fail_test.go": strings.ReplaceAll(failing, "%s", "linked"), ".linked/fail_test.go": "->../testdata/fail_test.go"}},
	} {
		commitFixture(t, &o, c.files)
		code, err := execute(ctx, o, full)
		if err != nil || code != 1 {
			t.Fatalf("%s failure skipped: code=%d err=%v", c.name, code, err)
		}
		report, err := os.ReadFile(filepath.Join(o.out, "go.json"))
		if err != nil || !strings.Contains(string(report), "\"Package\":\"example.org/fixture/"+c.pkg+"\"") {
			t.Fatalf("%s package outcome missing: %v", c.name, err)
		}
		if c.name == "hidden" {
			var e execution
			if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
				t.Fatal(err)
			}
			if !equal(e.Args[len(e.Args)-2:], []string{"./...", "./.hidden"}) {
				t.Fatalf("FULL argv: %v", e.Args)
			}
			// The historical row's expected universe names the hidden package,
			// so its missing outcome could never pass as complete.
			universe, err := goUniverse(ctx, o)
			if err != nil || !strings.Contains(strings.Join(universe, " "), "example.org/fixture/.hidden") {
				t.Fatalf("universe lacks hidden package: %v %v", universe, err)
			}
		}
		removed := map[string]string{}
		for name := range c.files {
			removed[name] = ""
		}
		commitFixture(t, &o, removed)
	}
}

// AFP-V0-9012: a hidden package added after the walk is refused, never
// silently skipped. A first run counts the Git status reads; the second adds
// the package just before the last one, the post-walk cleanliness check.
func TestFullSourceDriftHidden_AFPV09012(t *testing.T) {
	o, _ := fixture(t)
	state := t.TempDir()
	bin, counter, threshold, late := t.TempDir(), filepath.Join(state, "count"), filepath.Join(state, "at"), filepath.Join(o.root, ".late", "late_test.go")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	gitProxy := "#!/bin/sh\ncase \"$*\" in *'status --porcelain'*)\nn=$(cat " + quote(counter) + " 2>/dev/null || echo 0)\nn=$((n+1))\nprintf '%s' \"$n\" > " + quote(counter) + "\nif [ \"$n\" = \"$(cat " + quote(threshold) + " 2>/dev/null)\" ]; then\nmkdir -p " + quote(filepath.Dir(late)) + "\nprintf 'package late\\nimport \"testing\"\\nfunc TestFail(t *testing.T) { t.Fatal(\"late failure\") }\\n' > " + quote(late) + "\nfi;; esac\nexec /usr/bin/git \"$@\"\n"
	// closedEnv puts the Go binary's directory first on PATH, so a Go proxy
	// there makes the Git proxy the Git that every read uses.
	realGo, err := exec.LookPath("go")
	if err == nil {
		realGo, err = filepath.EvalSymlinks(realGo)
	}
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"git": gitProxy, "go": "#!/bin/sh\nexec " + quote(realGo) + " \"$@\"\n"} {
		if err := execfile.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	ctx := context.Background()
	full := selection{Reason: "planner unavailable", Packages: []string{"./..."}}
	if code, err := execute(ctx, o, full); err != nil || code != 1 {
		t.Fatalf("counting run: %d %v", code, err)
	}
	reads, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(counter); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(threshold, reads, 0600); err != nil {
		t.Fatal(err)
	}
	code, err := execute(ctx, o, full)
	if _, statErr := os.Stat(late); statErr != nil {
		t.Fatalf("late package never added: %v", statErr)
	}
	if code != 2 || err == nil || !strings.Contains(err.Error(), "source drift") {
		t.Fatalf("late hidden package not refused: %d %v", code, err)
	}
}

// AFP-V0-9012: the universe is listed in the -race execution build context, so
// a package whose only test is `//go:build race` is placed and run under
// sharded FULL, and appears in the universe reruns and shadow rows expect.
func TestFullShardedRaceOnly_AFPV09012(t *testing.T) {
	o, _ := fixture(t)
	commitFixture(t, &o, map[string]string{"raceonly/r_test.go": "//go:build race\n\npackage raceonly\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"race-only failure\") }\n"})
	ctx := context.Background()
	const pkg = "example.org/fixture/raceonly"
	universe, err := goUniverse(ctx, o)
	if err != nil || !strings.Contains(" "+strings.Join(universe, " ")+" ", " "+pkg+" ") {
		t.Fatalf("universe lacks race-only package: %v %v", universe, err)
	}
	o.shards, o.partitionDigest = 2, cishards.ProfileDigest()
	full := selection{Reason: "planner unavailable", Packages: []string{"./..."}}
	ran := 0
	for shard := 0; shard < o.shards; shard++ {
		o.shard = shard
		code, err := execute(ctx, o, full)
		if err != nil {
			t.Fatalf("shard %d: %v", shard, err)
		}
		report, err := os.ReadFile(filepath.Join(o.out, "go.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(report), "\"Package\":\""+pkg+"\"") {
			ran++
			if code != 1 {
				t.Fatalf("race-only failure did not fail shard %d: %d", shard, code)
			}
		}
	}
	if ran != 1 {
		t.Fatalf("race-only package ran on %d shards, want 1", ran)
	}
}
