package affected_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TSP-V0-724 (V1-3900): each LANGUAGE_FRONTIER entry names what raised it, so
// a shell unknown names its script and a Go unknown its package, and the
// golden pins both with their sources.
func TestFrontierUnknownsNameTheirSources_TSP_V0_724(t *testing.T) {
	root := t.TempDir()
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/m\n\ngo 1.27.0\n")
	write("lib/lib.go", "package lib\n\nfunc Value() int { return 1 }\n")
	write("lib/lib_linux.go", "//go:build linux\n\npackage lib\n\nfunc linux() int { return 2 }\n")
	write("lib/lib_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n")
	write("script/build.sh", "#!/bin/sh\necho build\n")
	graph, err := affected.Build(context.Background(), root, golang.New(), shell.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"lib/lib_linux.go", "script/build.sh"})
	var frontier []string
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownLanguageFrontier {
			frontier = append(frontier, unknown.Detail)
		}
	}
	got, err := json.Marshal(struct {
		Frontier []string          `json:"frontier"`
		Sources  map[string]string `json:"frontierSources"`
	}{frontier, plan.FrontierSources})
	if err != nil {
		t.Fatal(err)
	}
	const golden = `{"frontier":["go:build-constraint-variants","shell:foreign-invocation-unmodelled"],` +
		`"frontierSources":{"go:build-constraint-variants":"go:example.test/m/lib","shell:foreign-invocation-unmodelled":"shell:script/build.sh"}}`
	if string(got) != golden {
		t.Fatalf("frontier unknowns:\n got %s\nwant %s\nall %+v", got, golden, plan.Unknown)
	}
}
