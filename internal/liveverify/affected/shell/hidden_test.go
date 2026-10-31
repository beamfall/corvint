package shell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestIndexesGitHubShellScripts_NSAV0800 pins V1-3382: the shared source walk
// skips hidden directories, so a .github/forward-rows/*.sh edit was an owned
// file no unit declared (UNINDEXED_SOURCE_PATH) and forced a full run. The
// plugin now indexes shell files under .github, from a disk and an immutable
// source alike, with repository-relative paths; a hidden or skipped directory
// below .github stays unwalked, and a repository without .github still builds.
func TestIndexesGitHubShellScripts_NSAV0800(t *testing.T) {
	files := map[string]string{
		"script/run.sh":                   "#!/bin/sh\nsh .github/forward-rows/verdict.sh\n",
		".github/forward-rows/verdict.sh": "#!/bin/sh\necho verdict\n",
		".github/forward-rows/bar.jq":     ".\n",
		".github/.cache/skipped.sh":       "#!/bin/sh\n",
		".github/node_modules/skipped.sh": "#!/bin/sh\n",
		".other/unindexed.sh":             "#!/bin/sh\n",
	}
	disk := t.TempDir()
	mapped := fstest.MapFS{}
	for name, body := range files {
		path := filepath.Join(disk, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mapped[name] = &fstest.MapFile{Data: []byte(body), Mode: 0o644}
	}
	for name, source := range map[string]*affected.Source{
		"disk":      affected.DiskSourceContext(context.Background(), disk),
		"immutable": affected.FSSource(mapped),
	} {
		result, err := New().UnitsSource(source)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ids := []string{}
		var caller affected.Unit
		for _, unit := range result.Units {
			ids = append(ids, unit.ID)
			if unit.ID == "shell:script/run.sh" {
				caller = unit
			}
		}
		if want := []string{"shell:.github/forward-rows/verdict.sh", "shell:script/run.sh"}; !slices.Equal(ids, want) {
			t.Errorf("%s: units %q, want %q", name, ids, want)
		}
		if !slices.Contains(caller.Imports, "shell:.github/forward-rows/verdict.sh") {
			t.Errorf("%s: script/run.sh does not depend on the indexed verdict.sh: %+v", name, caller)
		}
	}
	graph, err := affected.Build(context.Background(), disk, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{".github/forward-rows/verdict.sh"})
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnindexedSourcePath {
			t.Errorf("plan has %s %s", unknown.Reason, unknown.Detail)
		}
	}
	if _, err := New().UnitsSource(affected.FSSource(fstest.MapFS{"a.sh": {Data: []byte("#!/bin/sh\n")}})); err != nil {
		t.Errorf("a repository without .github failed: %v", err)
	}
}
