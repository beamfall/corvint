// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-005: explicitly addressed Go packages below hidden or underscore
// directories remain owned and runnable, including their import dependents.
func TestAZUV0005HiddenPackages(t *testing.T) {
	files := map[string]string{
		"go.mod":                               "module example.test/m\n",
		".github/testconfine/confine.go":       "package testconfine\n",
		".github/testconfine/confine_test.go":  "package testconfine\n",
		".github/testconfine/cmd/main.go":      "package main\nimport _ \"example.test/m/.github/testconfine\"\n",
		".github/testconfine/cmd/main_test.go": "package main\n",
		".vscode/pkg/p.go":                     "package p\n",
		".vscode/pkg/p_test.go":                "package p\n",
		"_private/p.go":                        "package private\n",
		"_private/p_test.go":                   "package private\n",
		"testdata/.hidden/fixture.go":          "invalid fixture\n",
		".git/fake.go":                         "invalid metadata\n",
	}
	root := t.TempDir()
	writeFiles(t, root, files)
	source := fstest.MapFS{}
	for name, body := range files {
		source[name] = &fstest.MapFile{Data: []byte(body)}
	}
	disk, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	immutable, err := affected.BuildFS(source, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if disk.Digest() != immutable.Digest() {
		t.Fatalf("disk/immutable digest differs")
	}
	for _, graph := range []*affected.Graph{disk, immutable} {
		for dirty, want := range map[string][]string{
			".github/testconfine/confine.go":  {".github/testconfine/cmd/main_test.go", ".github/testconfine/confine_test.go"},
			".github/testconfine/cmd/main.go": {".github/testconfine/cmd/main_test.go"},
			"_private/p.go":                   {"_private/p_test.go"},
			".vscode/pkg/p.go":                {".vscode/pkg/p_test.go"},
		} {
			plan := affected.Select(graph, []string{dirty})
			if len(plan.Unknown) != 0 || !reflect.DeepEqual(plan.SelectedTests(), want) {
				t.Fatalf("%s: tests=%v unknown=%v", dirty, plan.SelectedTests(), plan.Unknown)
			}
		}
	}
}

// AZU-V0-007: a runnable declaration preserves the binary-build trigger,
// including mixed source/test changes, and carries the exact root argv.
func TestAZUV0007RunnableConsumer(t *testing.T) {
	root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
	writeFiles(t, root, map[string]string{affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":{"commands":["cmd/tool"],"run":["node","web/e2e.mjs"]}}}`})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range [][]string{{"lib/lib.go"}, {"cmd/tool/main.go"}, {"lib/lib.go", "cmd/tool/main_test.go"}} {
		plan := affected.Select(graph, dirty)
		if unknown := consumerUnknowns(plan); len(unknown) != 0 {
			t.Fatal(unknown)
		}
		found := 0
		for _, selection := range plan.Selected {
			if selection.Kind == "DECLARED_CONSUMER" {
				found++
				if !reflect.DeepEqual(selection.Run, []string{"node", "web/e2e.mjs"}) || !reflect.DeepEqual(selection.Tests, []string{"web/e2e.mjs"}) || selection.Witness.Kind != "DECLARED_CONSUMER" {
					t.Fatalf("selection=%+v", selection)
				}
			}
		}
		if found != 1 {
			t.Fatalf("%v: runnable consumers=%d", dirty, found)
		}
	}
	for _, dirty := range []string{"cmd/tool/main_test.go", "unrelated/unrelated.go"} {
		for _, selection := range affected.Select(graph, []string{dirty}).Selected {
			if selection.Kind == "DECLARED_CONSUMER" {
				t.Fatalf("test-only/unrelated selected consumer: %s", dirty)
			}
		}
	}
	// Run argv is part of immutable graph identity.
	writeFiles(t, root, map[string]string{affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":{"commands":["cmd/tool"],"run":["node","web/other.mjs"]}}}`})
	other, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if graph.Digest() == other.Digest() {
		t.Fatal("run change did not change graph digest")
	}
}

// AZU-V0-007: invalid argv fails closed; legacy entries keep their unknown.
func TestAZUV0007InvalidRun(t *testing.T) {
	for _, run := range []string{`null`, `[]`, `[""]`, `["node","bad\u0000arg"]`, `"node web/e2e.mjs"`} {
		root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
		writeFiles(t, root, map[string]string{affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":{"commands":["cmd/tool"],"run":` + run + `}}}`})
		result, err := golang.New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierBinaryExecsInvalid}) {
			t.Fatalf("run %s: frontier=%v", run, result.Frontier)
		}
	}
}
