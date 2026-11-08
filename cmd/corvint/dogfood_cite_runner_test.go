// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// DCW-V0-335 / V1-4851: identical maps, stdout, stderr and statuses for the
// former per-hunk processes and in-process citation, including failed probes.
func TestDogfoodInProcessCitationsMatchSubprocess(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv(lrfrepo.StableReuseEnv, "")
	if err := os.Unsetenv(lrfrepo.StableReuseEnv); err != nil {
		t.Fatal(err)
	}
	dogfoodCitationParity(t, true)
}

// DCW-V0-8760: environment drift after runner construction must retain Exec's
// locale and run-store contract for both cite and mark, without global mutation.
func TestDogfoodInProcessCitationsHostileEnvironment(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv(lrfrepo.StableReuseEnv, "")
	if err := os.Unsetenv(lrfrepo.StableReuseEnv); err != nil {
		t.Fatal(err)
	}
	dogfoodCitationParity(t, false)
	if os.Getenv("LC_ALL") != "tr_TR.UTF-8" || os.Getenv(lrfrepo.StableReuseEnv) != "/inherited/hostile-store" {
		t.Fatal("runner mutated caller environment")
	}
}

func dogfoodCitationParity(t *testing.T, inlineOnly bool) {
	t.Helper()
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
	commands = append(commands, []string{"cem", "mark", "--map", ".corvint/change.cem.json", "--hunk", "4", "--disposition", "unknown", "--reason", "insufficient-evidence"})
	successes := len(commands)
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
			if (i < successes && code != 0) || (i >= successes && code == 0) {
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
			if i >= successes && results[i].document != results[i-1].document {
				t.Fatal("a refused row changed the map")
			}
		}
		return results
	}
	inline := dogfoodCitationRunner(portable.binary, portable.binary)
	if !inlineOnly {
		t.Setenv("LC_ALL", "tr_TR.UTF-8")
		t.Setenv(lrfrepo.StableReuseEnv, "/inherited/hostile-store")
	}
	want := run(dogfoodflow.Exec(portable.binary))
	// Removing the executable after selection proves these rows start no child.
	if inlineOnly {
		if err := os.Remove(portable.binary); err != nil {
			t.Fatal(err)
		}
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

// DCW-V0-8760: only successful executable lookup plus same-file identity
// admits in-process execution; every unresolved selection stays external.
func TestDogfoodInProcessCitationsResolvePATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixtures")
	}
	cwd, installed, empty := t.TempDir(), t.TempDir(), t.TempDir()
	self := filepath.Join(cwd, "corvint")
	for path, body := range map[string]string{
		self:                                "#!/bin/sh\nexit 24\n",
		filepath.Join(installed, "corvint"): "#!/bin/sh\nprintf 'selected installed binary\\n'\nprintf 'selected stderr\\n' >&2\nexit 23\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(cwd)
	t.Setenv("GODEBUG", "execerrdot=1")
	for _, tc := range []struct {
		name, path string
		code       int
	}{
		{"different-same-named-executable", installed, 23},
		{"missing-path-selection", empty, 127},
		{"unproven-implicit-current-directory", ".", 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			var wantOut, wantErr, gotOut, gotErr bytes.Buffer
			args := []string{"cem", "cite"}
			want := dogfoodflow.Exec("corvint").Run(context.Background(), cwd, args, &wantOut, &wantErr)
			if want != tc.code {
				t.Fatalf("external fixture returned %d, want %d: %s", want, tc.code, &wantErr)
			}
			runner := dogfoodCitationRunner("corvint", self)
			got := runner.Run(context.Background(), cwd, args, &gotOut, &gotErr)
			if got != want || gotOut.String() != wantOut.String() || gotErr.String() != wantErr.String() {
				t.Fatalf("lookup changed execution: got (%d, %q, %q), want (%d, %q, %q)",
					got, &gotOut, &gotErr, want, &wantOut, &wantErr)
			}
		})
	}
}

// DCW-V0-8760: a new argv form is unproven even with identical environment
// and binary identity. It must execute the selected child, never runContext.
func TestDogfoodInProcessCitationsUnknownFormFallsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	t.Setenv("LC_ALL", "C")
	t.Setenv(lrfrepo.StableReuseEnv, "")
	if err := os.Unsetenv(lrfrepo.StableReuseEnv); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "selected")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'selected unknown form\\n'\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"cem", "cite", "--map", "map", "--hunk", "1", "--evidence-path", "rule", "--lines", "1:1", "--relation", "specification", "--future-option", "value"}
	var gotOut, gotErr, wantOut, wantErr bytes.Buffer
	root := t.TempDir()
	want := dogfoodflow.Exec(binary).Run(context.Background(), root, args, &wantOut, &wantErr)
	got := dogfoodCitationRunner(binary, binary).Run(context.Background(), root, args, &gotOut, &gotErr)
	if want != 23 || got != want || gotOut.String() != wantOut.String() || gotErr.String() != wantErr.String() {
		t.Fatalf("unknown form bypassed child: got (%d, %q, %q), want (%d, %q, %q)", got, &gotOut, &gotErr, want, &wantOut, &wantErr)
	}
}

// DCW-V0-8760 / V1-4851: exercise inputs-check's normal change.run context,
// including its private reuse store, and count actual external citation calls.
func TestDogfoodInProcessCitationsInputsCheckRunStore(t *testing.T) {
	portable := portableDogfoodRunner(t)
	for _, inline := range []bool{false, true} {
		t.Run(fmt.Sprintf("inline=%t", inline), func(t *testing.T) {
			t.Setenv("LC_ALL", "C")
			t.Setenv(lrfrepo.StableReuseEnv, "/inherited/unused-store")
			t.Setenv("DOGFOOD_TASK", "Answer two.")
			t.Setenv("DOGFOOD_VERIFY", "go test ./fixture")
			root, base := portableDogfoodRepo(t)
			for i := 0; i < 3; i++ {
				cemWrite(t, root, fmt.Sprintf("extra%d.txt", i), "new text\n")
			}
			cemGit(t, root, "add", ".")
			cemGit(t, root, "commit", "-qm", "extra hunks")
			head := cemGit(t, root, "rev-parse", "HEAD")
			_, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n2\tintent.md\t5:5\tspecification\n3\tintent.md\t1:5\tspecification\n4\tintent.md\t5:5\tspecification\n", "intent.md\n")
			flags := map[string]string{"--seal": "no"}
			for i := 0; i < len(args); i += 2 {
				flags[args[i]] = args[i+1]
			}
			children, citationChildren, probes := 0, 0, 0
			external := dogfoodflow.Exec(portable.binary)
			execRun := external.Run
			external.Run = func(ctx context.Context, root string, args []string, out, errout io.Writer) int {
				children++
				if inlineCitationArgs(args) {
					citationChildren++
				}
				return execRun(ctx, root, args, out, errout)
			}
			runner := external
			if inline {
				runner = inlineDogfoodCitationRunner(external, portable.binary)
			}
			run := runner.Run
			runner.Run = func(ctx context.Context, root string, args []string, out, errout io.Writer) int {
				if inlineCitationArgs(args) {
					probes++
					store := ""
					for _, entry := range dogfoodflow.CitationEnvironment(ctx) {
						if value, ok := strings.CutPrefix(entry, lrfrepo.StableReuseEnv+"="); ok {
							store = value
						}
					}
					info, err := os.Stat(store)
					if err != nil || !info.IsDir() || store == os.Getenv(lrfrepo.StableReuseEnv) {
						t.Fatalf("citation has no run-owned reuse directory: %q, %v", store, err)
					}
				}
				return run(ctx, root, args, out, errout)
			}
			var out, errout bytes.Buffer
			code, err := runDogfoodAction(context.Background(), root, "inputs-check", base, flags,
				func(string) dogfoodflow.Runner { return runner }, &out, &errout)
			if err != nil || code != 0 || !strings.Contains(out.String(), "prepare PASS hunks=4") {
				t.Fatalf("inputs-check: %d %v\n%s\n%s", code, err, &out, &errout)
			}
			want := 4
			if inline {
				want = 0
			}
			if probes != 4 || citationChildren != want || children != 1+want {
				t.Fatalf("probes=%d citation children=%d total children=%d; want 4, %d, %d", probes, citationChildren, children, want, 1+want)
			}
			if cemGit(t, root, "rev-parse", "HEAD") != head || cemGit(t, root, "status", "--porcelain", "--untracked-files=all") != "" {
				t.Fatal("inputs-check changed HEAD or left map/probe outputs")
			}
			if os.Getenv("LC_ALL") != "C" || os.Getenv(lrfrepo.StableReuseEnv) != "/inherited/unused-store" {
				t.Fatal("inputs-check changed process environment")
			}
		})
	}
}

// DCW-V0-8760: only the two understood child settings may differ. Any future
// transformation (including removal) leaves the inline handler unproven.
func TestDogfoodInProcessCitationsUnknownEnvironmentFallsBack(t *testing.T) {
	args := []string{"cem", "cite", "--map", "map", "--hunk", "1", "--evidence-path", "rule", "--lines", "1:1", "--relation", "specification"}
	t.Setenv("CORVINT_FUTURE_SETTING", "parent")
	for _, value := range []string{"child", ""} {
		child := dogfoodflow.CitationEnvironment(context.Background())
		filtered := []string{}
		for _, entry := range child {
			if !strings.HasPrefix(entry, "CORVINT_FUTURE_SETTING=") {
				filtered = append(filtered, entry)
			}
		}
		if value != "" {
			filtered = append(filtered, "CORVINT_FUTURE_SETTING="+value)
		}
		var out, errout bytes.Buffer
		if code, handled := runDogfoodCitation(context.Background(), t.TempDir(), args, filtered, &out, &errout); handled || code != 0 || out.Len() != 0 || errout.Len() != 0 {
			t.Fatalf("unknown child transformation handled inline: %d %t %s %s", code, handled, &out, &errout)
		}
	}
}
