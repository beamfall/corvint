// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// DCW-V0-335 / V1-4851: identical maps, stdout, stderr and statuses for the
// former per-hunk processes and in-process citation, including failed probes.
func TestDogfoodInProcessCitationsMatchSubprocess(t *testing.T) {
	portable := portableDogfoodRunner(t)
	root, base, _ := cemRepo(t)
	for i := 0; i < 3; i++ {
		cemWrite(t, root, fmt.Sprintf("src/extra%d.txt", i), fmt.Sprintf("extra %d\n", i))
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "more hunks")
	if code, out, errout := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", "HEAD"); code != 0 {
		t.Fatalf("prepare: %d %s %s", code, out, errout)
	}
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	initial, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		code                         int
		out, errout, document, probe string
	}
	commands := [][]string{}
	for i := 1; i <= 4; i++ {
		commands = append(commands, []string{"cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", fmt.Sprint(i), "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification", "--output", ".corvint/probe.json"})
	}
	for i := 1; i <= 4; i++ {
		commands = append(commands, []string{"cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", fmt.Sprint(i), "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification"})
	}
	// A refused later row cannot silently succeed or alter the cited map.
	commands = append(commands,
		[]string{"cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "missing.txt", "--lines", "1:1", "--relation", "specification"},
		[]string{"cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "2", "--evidence-path", "docs/rule.txt", "--lines", "9:1", "--relation", "specification"},
	)
	run := func(runner dogfoodflow.Runner) []result {
		t.Helper()
		if err := os.WriteFile(mapPath, initial, 0600); err != nil {
			t.Fatal(err)
		}
		results := []result{}
		for i, args := range commands {
			var out, errout bytes.Buffer
			code := runner.Run(context.Background(), root, args, &out, &errout)
			if (i < 8 && code != 0) || (i >= 8 && code == 0) {
				t.Fatalf("row %d: %d %s %s", i+1, code, &out, &errout)
			}
			document, err := os.ReadFile(mapPath)
			if err != nil {
				t.Fatal(err)
			}
			probe := ""
			if i < 4 {
				if !bytes.Equal(document, initial) {
					t.Fatal("a prepare probe changed the map")
				}
				data, err := os.ReadFile(filepath.Join(root, ".corvint", "probe.json"))
				if err != nil {
					t.Fatal(err)
				}
				probe = string(data)
				if err := os.Remove(filepath.Join(root, ".corvint", "probe.json")); err != nil {
					t.Fatal(err)
				}
			}
			results = append(results, result{code, out.String(), errout.String(), string(document), probe})
			if i >= 8 && results[i].document != results[i-1].document {
				t.Fatal("a refused row changed the map")
			}
		}
		return results
	}
	want := run(dogfoodflow.Exec(portable.binary))
	inline := dogfoodCitationRunner(portable.binary, portable.binary)
	// Removing the executable after selection proves these rows start no child.
	if err := os.Remove(portable.binary); err != nil {
		t.Fatal(err)
	}
	got := run(inline)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d differs:\n got %+v\nwant %+v", i+1, got[i], want[i])
		}
	}
}

func TestDogfoodInProcessCitationsPreserveSelectedExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-corvint")
	runner := dogfoodCitationRunner(missing, self)
	var out, errout bytes.Buffer
	if code := runner.Run(context.Background(), t.TempDir(), []string{"cem", "cite"}, &out, &errout); code != 127 {
		t.Fatalf("selected missing binary returned %d, want exec failure 127", code)
	}
}
