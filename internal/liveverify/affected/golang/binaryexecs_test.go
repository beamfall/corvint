package golang_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const binaryExecModule = "go:example.test/m/"

// binaryExecRepository is a module with one command (cmd/tool) built from lib,
// a package whose test builds it by a path literal (builder), a package that
// runs a binary it is handed and so names no literal (runner, declared), and a
// package with no tie to the command (unrelated).
func binaryExecRepository(t *testing.T, declaration string) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n",
		"lib/lib.go":                "package lib\n\nfunc Help() string { return \"help\" }\n",
		"cmd/tool/main.go":          "package main\n\nimport \"example.test/m/lib\"\n\nfunc main() { println(lib.Help()) }\n",
		"cmd/tool/main_test.go":     "package main\n\nvar self = \"./cmd/tool\"\n",
		"builder/builder.go":        "package builder\n",
		"builder/builder_test.go":   "package builder\n\nvar build = []string{\"go\", \"build\", \"-o\", \"bin\", \"./cmd/tool\"}\n",
		"runner/runner.go":          "package runner\n\nvar flagName = \"tool\"\n",
		"runner/runner_test.go":     "package runner\n",
		"unrelated/unrelated.go":    "package unrelated\n",
		"unrelated/other_test.go":   "package unrelated\n\nvar data = \"cmd/tool/README.md\"\n",
		"climber/climber_test.go":   "package climber\n\nvar build = \"../cmd/tool\"\n",
		"anchored/anchored_test.go": "package anchored\n\nvar build = \"example.test/m/cmd/tool\"\n",
	})
	if declaration != "" {
		writeFiles(t, root, map[string]string{affected.BinaryExecsPath: declaration})
	}
	return root
}

// AFP-V0-037: a package that runs a command's built binary, by a literal
// naming the command's directory or by the declaration, is selected when a
// dirty path reaches the command's build, and not on a change elsewhere.
func TestBinaryExecConsumerIsSelectedWithTheCommandsBuild_AFPV0037(t *testing.T) {
	root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	tool := binaryExecModule + "cmd/tool"
	execs := map[string][]string{}
	for _, unit := range result.Units {
		if unit.Execs != nil {
			execs[unit.ID] = unit.Execs
		}
	}
	want := map[string][]string{
		binaryExecModule + "anchored": {tool},
		binaryExecModule + "builder":  {tool},
		binaryExecModule + "climber":  {tool},
		binaryExecModule + "runner":   {tool},
	}
	if !reflect.DeepEqual(execs, want) {
		t.Fatalf("execs = %v, want %v", execs, want)
	}
	graph, err := affected.Build(root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	runners := []string{"anchored", "builder", "climber", "runner"}
	for _, dirty := range []string{"lib/lib.go", "cmd/tool/main.go"} {
		plan := affected.Select(graph, []string{dirty})
		for _, name := range runners {
			selection, ok := selected(plan, binaryExecModule+name)
			if !ok {
				t.Errorf("%s: %s not selected; selected %v", dirty, name, plan.SelectedTests())
				continue
			}
			if selection.Witness.Kind != affected.WitnessBinaryExec || selection.Witness.Via[len(selection.Witness.Via)-2] != tool {
				t.Errorf("%s: %s witness = %+v, want BINARY_EXEC through %s", dirty, name, selection.Witness, tool)
			}
		}
	}
	for _, dirty := range []string{"unrelated/unrelated.go", "unrelated/other_test.go"} {
		plan := affected.Select(graph, []string{dirty})
		for _, name := range runners {
			if _, ok := selected(plan, binaryExecModule+name); ok {
				t.Errorf("%s selected %s, which runs only the command's built binary", dirty, name)
			}
		}
	}
	first, err := affected.Select(graph, []string{"lib/lib.go"}).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	again, err := affected.Select(graph, []string{"lib/lib.go"}).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(again) {
		t.Error("plan is not byte-identical across runs")
	}
}

// AFP-V0-037: any defect in a present declaration keeps no declared edge and
// raises the module-level frontier; the literal edges stay.
func TestInvalidBinaryExecDeclarationDeclaresNothing_AFPV0037(t *testing.T) {
	for name, declaration := range map[string]string{
		"not a command":    `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["lib"]}}`,
		"unknown package":  `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"],"nowhere":["cmd/tool"]}}`,
		"unknown member":   `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]},"x":1}`,
		"wrong profile":    `{"profile":"corvint-test-binary-execs/1","packages":{"runner":["cmd/tool"]}}`,
		"empty entries":    `{"profile":"corvint-test-binary-execs/0","packages":{"runner":[]}}`,
		"unsorted entries": `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["lib","cmd/tool"]}}`,
		"self entry":       `{"profile":"corvint-test-binary-execs/0","packages":{"cmd/tool":["cmd/tool"]}}`,
		"escaping entry":   `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["../cmd/tool"]}}`,
		"malformed":        `{`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := golang.New().Units(binaryExecRepository(t, declaration))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierBinaryExecsInvalid}) {
				t.Fatalf("frontier = %v, want %s", result.Frontier, golang.FrontierBinaryExecsInvalid)
			}
			for _, unit := range result.Units {
				switch unit.ID {
				case binaryExecModule + "runner":
					if unit.Execs != nil {
						t.Errorf("runner keeps declared edges %v", unit.Execs)
					}
				case binaryExecModule + "builder":
					if !slices.Equal(unit.Execs, []string{binaryExecModule + "cmd/tool"}) {
						t.Errorf("builder literal edges = %v", unit.Execs)
					}
				}
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		result, err := golang.New().Units(binaryExecRepository(t, ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Frontier) != 0 {
			t.Fatalf("frontier = %v", result.Frontier)
		}
	})
}

func selected(plan affected.Plan, id string) (affected.Selection, bool) {
	for _, selection := range plan.Selected {
		if selection.UnitID == id {
			return selection, true
		}
	}
	return affected.Selection{}, false
}
