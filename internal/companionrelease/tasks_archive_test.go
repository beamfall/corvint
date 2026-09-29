package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestTasksArchiveAssembly(t *testing.T) {
	t.Run("CAL-V0-042 standalone deterministic Tasks artifact", func(t *testing.T) {
		files := []ArchiveEntry{{Path: "bin/corvint-tasks", Mode: 0o755, Data: []byte("binary")}, {Path: "notices/corvint-tasks/LICENSE", Mode: 0o644, Data: []byte("license")}}
		r := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: supportedTarget, Version: json.RawMessage(`{"version":"unverified"}`), Qualification: "reproducible-build-and-native-version-help-smoke-only"}
		makeEntries := func() ([]ArchiveEntry, error) { return tasksArchiveEntries(files, r) }
		archive, err := buildTarGzTwice(makeEntries)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := makeEntries()
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyTarGz(archive, entries); err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Path == "MANIFEST.json" && !bytes.Contains(e.Data, []byte(`"profile": "corvint-tasks-archive/0"`)) {
				t.Fatal("wrong profile")
			}
			if e.Path == "SHA256SUMS" && !bytes.Contains(e.Data, []byte("bin/corvint-tasks")) {
				t.Fatal("missing binary checksum")
			}
		}
	})
}

func TestTasksArchiveHelpRefusesOldRuntime(t *testing.T) {
	for _, raw := range []string{`{}`, `{"outcome":"OK","items":[{"implemented":["claim"]}]}`} {
		if err := verifyTasksArchiveHelp([]byte(raw)); err == nil {
			t.Fatalf("old runtime admitted: %s", raw)
		}
	}
	raw := `{"outcome":"OK","items":[{"implemented":["claim","plan preview","submit","gate run","complete"]}]}`
	if err := verifyTasksArchiveHelp([]byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestTasksArchiveRefusesOtherTargets(t *testing.T) {
	for _, target := range []string{"", "darwin/amd64", "linux/amd64", "windows/amd64"} {
		if _, err := RunTasksArchive(context.Background(), Options{Target: target}); err == nil {
			t.Fatalf("admitted %q", target)
		}
	}
}

func TestTasksArchiveRequiresSetupResources(t *testing.T) {
	paths := []string{"docs/TASKS-EXTERNAL-AGENTS.md", "internal/tasks/cli/testdata/external-agents/policy.json", "internal/tasks/cli/testdata/external-agents/queue.json", "internal/tasks/cli/testdata/external-agents/ticket-create.json"}
	for missing := -1; missing < len(paths); missing++ {
		source := Export{}
		for i, path := range paths {
			if i != missing {
				source.Files = append(source.Files, SourceFile{Path: path, Data: []byte(path)})
			}
		}
		entries, err := tasksArchiveResources(source)
		if missing >= 0 && err == nil {
			t.Fatalf("missing %s admitted", paths[missing])
		}
		if missing == -1 && (err != nil || len(entries) != 4) {
			t.Fatalf("complete resources: %v, %v", entries, err)
		}
	}
}
