package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestContextMissSpawnsTheObservationPairOnce counts the Git processes a
// snapshot miss costs once the snapshot store exists: the loader's identity and
// status pair opens the build's stability window instead of being spawned
// again, so the pair appears twice in total (opening and closing), not three
// times. The dirty path makes the residual cat-file batch part of the count.
func TestContextMissSpawnsTheObservationPairOnce(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shim uses a POSIX shell")
	}
	root := taskContextRepository(t)
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "index"}, strings.NewReader(""), &out, &stderr); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr.String())
	}
	snapshots, _ := filepath.Glob(filepath.Join(contextindex.SnapshotDirectory(root), "*"))
	for _, snapshot := range snapshots {
		if err := os.Remove(snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "demux_test.go"), []byte("package cache\n\nfunc TestSplit() { _ = Split(\"key\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "spawns")
	shim := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %s\nexec %s \"$@\"\n", strconv.Quote(log), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"--root", root, "context", "--task", "does `Split` keep empty keys", "--subject", "cache/demux.go"}
	if _, stderr, code := runCandidateWithEnvironment(t, "", []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}, arguments...); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	spawned, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(spawned)), "\n") {
		command, ok := spawnedGitCommand(strings.Fields(line))
		if !ok {
			t.Fatalf("cannot parse Git invocation: %s", line)
		}
		counts[command]++
	}
	want := map[string]int{"rev-parse": 2, "status": 2, "ls-tree": 1, "cat-file": 1, "log": 1}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("git spawns on a miss = %v, want %v:\n%s", counts, want, spawned)
	}
}

func spawnedGitCommand(arguments []string) (string, bool) {
	for position := 0; position < len(arguments); {
		switch argument := arguments[position]; {
		case argument == "--no-optional-locks":
			position++
		case argument == "-c" || argument == "-C":
			position += 2
		case strings.HasPrefix(argument, "--git-dir=") || strings.HasPrefix(argument, "--work-tree="):
			position++
		case strings.HasPrefix(argument, "-"):
			return "", false
		default:
			return argument, true
		}
		if position > len(arguments) {
			return "", false
		}
	}
	return "", false
}
