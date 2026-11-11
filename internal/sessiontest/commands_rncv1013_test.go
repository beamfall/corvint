// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const legacyDeclaration = `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": ["-count=1"]}, "languages": {"python": ["make", "py"]}}`

// RNC-V1-013: CommandsPath first; the old path only in Corvint's own
// repository and only while CommandsPath is absent; otherwise the
// built-in defaults. A malformed CommandsPath is Invalid, never a default.
func TestRNCV1013ReadDeclaredResolution(t *testing.T) {
	adopter := writeTree(t, map[string]string{"go.mod": "module example.test/m\n", DeclarationPath: legacyDeclaration})
	if got := ReadDeclared(adopter); !reflect.DeepEqual(got, BuiltInDeclared()) {
		t.Fatalf("adopter with only the old path read it: %#v", got)
	}
	corvint := writeTree(t, map[string]string{"go.mod": "module " + corvintModule + "\n\ngo 1.27\n", DeclarationPath: legacyDeclaration})
	if got := ReadDeclared(corvint); got.Source != DeclarationPath || got.Unreadable != "" || !slices.Equal(got.GoFlags, []string{"-count=1"}) {
		t.Fatalf("Corvint's own old path: %#v", got)
	}
	both := writeTree(t, map[string]string{
		"go.mod": "module " + corvintModule + "\n", DeclarationPath: legacyDeclaration,
		CommandsPath: `{"profile": "corvint-test-commands/0", "runners": {"pytest": ["uv", "run", "pytest", "-p", "no:cacheprovider"]}, "instances": {"jest@apps/web": ["pnpm", "jest"]}}`,
	})
	got := ReadDeclared(both)
	want := Declared{
		GoEnv: []string{}, GoFlags: []string{}, GoDefault: true, Languages: []LanguageCommand{},
		Instances: []RunnerCommand{{Argv: []string{"pnpm", "jest"}, Key: "jest@apps/web"}},
		Runners:   []RunnerCommand{{Argv: []string{"uv", "run", "pytest", "-p", "no:cacheprovider"}, Key: "pytest"}},
		Source:    CommandsPath,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandsPath beside the old path:\n got %#v\nwant %#v", got, want)
	}
	for name, body := range map[string]string{
		"malformed":       `{"profile": "corvint-test-commands/0",`,
		"unknown profile": `{"profile": "corvint-test-commands/9"}`,
		"go runner":       `{"profile": "corvint-test-commands/0", "runners": {"go": ["go", "test"]}}`,
		"unknown runner":  `{"profile": "corvint-test-commands/0", "runners": {"cypress": ["npx", "cypress", "run"]}}`,
		"empty argv":      `{"profile": "corvint-test-commands/0", "runners": {"jest": []}}`,
		"bad instance":    `{"profile": "corvint-test-commands/0", "instances": {"jest@../x": ["npx", "jest"]}}`,
		"instance no at":  `{"profile": "corvint-test-commands/0", "instances": {"jest": ["npx", "jest"]}}`,
	} {
		got := ReadDeclared(writeTree(t, map[string]string{CommandsPath: body, DeclarationPath: legacyDeclaration, "go.mod": "module " + corvintModule + "\n"}))
		if !got.Invalid || !strings.HasPrefix(got.Unreadable, CommandsPath+": ") || got.GoDefault || len(got.Runners) != 0 {
			t.Errorf("%s: %#v, want Invalid", name, got)
		}
	}
}

// RNC-V1-013, RNC-V1-014: an instance's full command is its instance
// entry, else its runner entry, else the built-in default, from its root.
func TestRNCV1013InstanceCommand(t *testing.T) {
	root := writeTree(t, map[string]string{"engine/bin/rails": "#!/bin/sh\n"})
	declared := Declared{
		Instances: []RunnerCommand{{Argv: []string{"pnpm", "jest"}, Key: "jest@apps/web"}},
		Runners:   []RunnerCommand{{Argv: []string{"yarn", "jest"}, Key: "jest"}},
	}
	for _, c := range []struct {
		key, dir, source string
		argv             []string
	}{
		{"jest@apps/web", "apps/web", CommandInstance, []string{"pnpm", "jest"}},
		{"jest@apps/admin", "apps/admin", CommandRunner, []string{"yarn", "jest"}},
		{"pytest@.", ".", CommandDefault, []string{"python", "-m", "pytest"}},
		{"minitest@engine", "engine", CommandDefault, []string{"bin/rails", "test"}},
		{"minitest@.", ".", CommandDefault, []string{"bundle", "exec", "rake", "test"}},
	} {
		argv, dir, source := declared.InstanceCommand(c.key, root)
		if !slices.Equal(argv, c.argv) || dir != c.dir || source != c.source {
			t.Errorf("%s: %q %q %q, want %q %q %q", c.key, argv, dir, source, c.argv, c.dir, c.source)
		}
	}
	if argv, _, _ := declared.InstanceCommand("cypress@.", root); argv != nil {
		t.Fatalf("an unknown runner has a command: %q", argv)
	}
}

// RNC-V1-013: FULL with no declaration runs each declared runner
// instance's built-in default from its root, and the Go default only where
// a Go instance exists; a language with an entry keeps its one command.
func TestRNCV1013FullRunsInstanceDefaults(t *testing.T) {
	pytest := contract.Instance{Runner: "pytest", Root: "services/a", Config: "services/a/pytest.ini"}
	input := Input{
		Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "FALLBACK forced",
		Instances: []contract.Instance{pytest, {Runner: "jest", Root: ".", Config: "jest.config.js"}},
	}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	var units []string
	for _, unit := range m.Units {
		units = append(units, unit.Unit)
	}
	slices.Sort(units)
	if m.Scope != ScopeFull || !slices.Equal(units, []string{"declared:jest@.", "declared:pytest@services/a"}) || len(m.Incomplete) != 0 {
		t.Fatalf("scope %s units %v incomplete %v", m.Scope, units, m.Incomplete)
	}
	root := t.TempDir()
	planned := Invocations(m, input.Declared, 0, root, "")
	if len(planned) != 2 || !slices.Equal(planned[0].Argv, []string{"python", "-m", "pytest"}) || planned[0].Dir != "services/a" ||
		!slices.Equal(planned[1].Argv, []string{"npx", "jest"}) || planned[1].Dir != "" {
		t.Fatalf("invocations %#v", planned)
	}
	if runnerKey(planned[0]) != "declared ./services/a python" {
		t.Fatalf("runner key %q", runnerKey(planned[0]))
	}
	input.Instances = append(input.Instances, contract.Instance{Runner: "go", Root: ".", Config: "go.mod"})
	input.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"make", "py"}}}
	m, err = Build(input)
	if err != nil {
		t.Fatal(err)
	}
	units = nil
	for _, unit := range m.Units {
		units = append(units, unit.Unit)
	}
	slices.Sort(units)
	if !slices.Equal(units, []string{"declared:go", "declared:jest@.", "declared:python"}) {
		t.Fatalf("with a Go instance and a python entry: %v", units)
	}
	input.RunnerProfile = ProfileCI
	if m, err = Build(input); err != nil || !slices.Contains(m.Checks, "jest@. full command") {
		t.Fatalf("ci checks %v %v", m.Checks, err)
	}
}
