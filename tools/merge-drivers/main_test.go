// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Git invokes the test binary as a driver, exercising run without nested Go builds.
func TestDriverHelper(t *testing.T) {
	if os.Getenv("CORVINT_MERGE_DRIVER_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(run(os.Args[i+1:], os.Stderr))
		}
	}
	os.Exit(2)
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func write(t *testing.T, root, path, data string) {
	t.Helper()
	file := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGitDriversDWCV07703(t *testing.T) {
	t.Setenv("CORVINT_MERGE_DRIVER_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, base, ours, theirs string
		conflict                       bool
	}{
		{"append", "docs/specs/fixture.md", "anchor\n", "anchor\n## Ours\nA\n", "anchor\n## Theirs\nB\n", false},
		{"edit", "docs/specs/fixture.md", "old\n", "ours\n", "theirs\n", true},
		{"json", ".corvint/test-read-scopes.json", `{"packages":{}}`, `{"packages":{"a":[]}}`, `{"packages":{"b":[]}}`, false},                               //corvint:fixture-path
		{"json-divergence", ".corvint/test-read-scopes.json", `{"packages":{"a":[]}}`, `{"packages":{"a":["ours"]}}`, `{"packages":{"a":["theirs"]}}`, true}, //corvint:fixture-path
		{"projection", "docs/specs/README.md", "base\n", "ours\n", "theirs\n", false},                                                                        //corvint:fixture-path
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			git(t, root, "init", "-q", "-b", "main")
			for _, file := range []string{".gitattributes", "Makefile"} {
				data, err := os.ReadFile(filepath.Join("../..", file))
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, file, string(data))
			}
			// Two installs must leave exactly the same local config bytes.
			var installed []byte
			for i := 0; i < 2; i++ {
				cmd := exec.Command("make", "merge-drivers")
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("install: %v %s", err, out)
				}
				config, err := os.ReadFile(filepath.Join(root, ".git/config"))
				if err != nil || (i > 0 && !bytes.Equal(installed, config)) {
					t.Fatalf("non-idempotent config: %v", err)
				}
				installed = config
			}
			for name, mode := range map[string]string{"specappend": "append-union", "registryunion": "json-union", "specgenerated": "keep-ours"} {
				want := "GOTOOLCHAIN=local go run ./tools/merge-drivers " + mode + ` "%O" "%A" "%B" %L`
				if got := git(t, root, "config", "--local", "merge."+name+".driver"); got != want {
					t.Fatalf("driver %s: %q, want %q", name, got, want)
				}
				driver := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "' -test.run=^TestDriverHelper$ -- " + mode + ` "%O" "%A" "%B" %L`
				git(t, root, "config", "--local", "merge."+name+".driver", driver)
			}
			write(t, root, tc.path, tc.base)
			git(t, root, "add", "-A")
			git(t, root, "commit", "-qm", "base")
			git(t, root, "checkout", "-qb", "other")
			write(t, root, tc.path, tc.theirs)
			git(t, root, "commit", "-qam", "theirs")
			git(t, root, "checkout", "-q", "main")
			write(t, root, tc.path, tc.ours)
			git(t, root, "commit", "-qam", "ours")
			cmd := exec.Command("git", "-C", root, "merge", "--no-commit", "other")
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.conflict {
				t.Fatalf("merge err=%v want conflict=%v: %s", err, tc.conflict, out)
			}
			result, err := os.ReadFile(filepath.Join(root, tc.path))
			if err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "append":
				if string(result) != "anchor\n## Ours\nA\n\n## Theirs\nB\n" || git(t, root, "diff", "--name-only", "--diff-filter=U") != "" {
					t.Fatalf("append merge: %q", result)
				}
			case "json":
				if !bytes.Contains(result, []byte(`"a": []`)) || !bytes.Contains(result, []byte(`"b": []`)) {
					t.Fatalf("lost key: %s", result)
				}
			case "projection", "json-divergence":
				if string(result) != tc.ours {
					t.Fatalf("ours changed: %q", result)
				}
			case "edit":
				if !bytes.Contains(result, []byte("||||||| base\nold\n")) {
					t.Fatalf("edit conflict lost: %q", result)
				}
			}
		})
	}
}

// This is the real --check used by make spec-requirements-check, without the
// target's additional Go tests. A keep-ours merge must not bypass its stale check.
func TestProjectionDWCV07701StaleCheck(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	script, err := os.ReadFile("../../script/gen-spec-requirements.sh")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "script/gen-spec-requirements.sh", string(script))
	write(t, root, "docs/specs/README.md", "# Specs\n\n| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n|---|---|---|---|---|\n| Fixture | [Fixture](fixture.md) | accepted | experimental | A fixture. |\n") //corvint:fixture-path
	spec := "# Fixture\n\nIntent status: accepted\nDelivery status: experimental\n\n## Agent digest\n- Claim: A fixture.\n- Status: accepted / experimental\n\n## Requirements\n\n- `FIX-V0-001`: One.\n"
	write(t, root, "docs/specs/fixture.md", spec)
	git(t, root, "add", "-A")
	runGenerator := func(mode string) ([]byte, error) {
		cmd := exec.Command("sh", "script/gen-spec-requirements.sh", mode)
		cmd.Dir = root
		return cmd.CombinedOutput()
	}
	if out, err := runGenerator("--write"); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	git(t, root, "add", "-A")
	if out, err := runGenerator("--check"); err != nil {
		t.Fatalf("fresh failed: %v %s", err, out)
	}
	write(t, root, "docs/specs/fixture.md", spec+"- `FIX-V0-002`: Two.\n")
	git(t, root, "add", "-A")
	if out, err := runGenerator("--check"); err == nil || !bytes.Contains(out, []byte("REQUIREMENTS.tsv")) {
		t.Fatalf("stale passed: %v %s", err, out)
	}
}
