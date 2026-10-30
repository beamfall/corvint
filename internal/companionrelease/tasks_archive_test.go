package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
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

// UPD-V0-017: each Tasks archive publishes its own per-platform report bound to
// the archive digest, so one release can qualify both platforms.
func TestUPDV0017TasksReportAssetPerPlatform(t *testing.T) {
	for target, want := range map[string]string{supportedTarget: "build-verification_darwin_arm64.json", tasksCrossTarget: "build-verification_linux_amd64.json"} {
		r := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: target, Build: "401", ArchivePath: "/host/local/path", ArchiveSHA256: strings.Repeat("a", 64), Qualification: "q"}
		e, err := tasksReportAsset(r)
		if err != nil {
			t.Fatal(err)
		}
		if e.Path != want {
			t.Fatalf("%s: asset %q, want %q", target, e.Path, want)
		}
		var got map[string]any
		if err := json.Unmarshal(e.Data, &got); err != nil {
			t.Fatal(err)
		}
		if got["profile"] != "corvint-tasks-archive/0" || got["target"] != target || got["build"] != "401" || got["archiveSha256"] != r.ArchiveSHA256 {
			t.Fatalf("%s: report %v", target, got)
		}
		if _, ok := got["archivePath"]; ok {
			t.Fatalf("%s: host-local archive path published", target)
		}
	}
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
	for _, target := range []string{"", "darwin/amd64", "linux/arm64", "windows/amd64"} {
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

func TestTasksArchiveCrossTargetIsLinuxAmd64Only(t *testing.T) {
	// CAL-V0-320: linux/amd64 is admitted for the Tasks archive on a darwin host, never for the
	// full bundle, and the native-host restriction still refuses every other foreign target.
	native, err := admitTasksTarget("linux/amd64", "darwin/arm64")
	if err != nil || native {
		t.Fatalf("linux/amd64 on darwin/arm64: native=%v err=%v", native, err)
	}
	if native, err := admitTasksTarget("darwin/arm64", "darwin/arm64"); err != nil || !native {
		t.Fatalf("native target: %v %v", native, err)
	}
	for _, target := range []string{"linux/arm64", "darwin/amd64", "windows/amd64", ""} {
		if _, err := admitTasksTarget(target, "darwin/arm64"); err == nil {
			t.Fatalf("admitted %q", target)
		}
	}
	// darwin/arm64 requested on a foreign host keeps the native-host restriction.
	if _, err := admitTasksTarget("darwin/arm64", "linux/amd64"); err == nil || !strings.Contains(err.Error(), "native target host") {
		t.Fatalf("foreign-host darwin build: %v", err)
	}
	if err := validateTarget("linux/amd64"); err == nil {
		t.Fatal("full companion bundle admitted linux/amd64")
	}
}

func TestTasksArchiveCrossBuildExecutesNothing(t *testing.T) {
	// The extract directory does not exist: any attempt to run the target binary would fail.
	version, qual, err := tasksArchiveSmoke(context.Background(), false, t.TempDir()+"/absent", nil)
	if err != nil || version != nil || qual != tasksCrossQualification {
		t.Fatalf("cross smoke: %q %q %v", version, qual, err)
	}
	if _, _, err := tasksArchiveSmoke(context.Background(), true, t.TempDir()+"/absent", nil); err == nil {
		t.Fatal("native smoke ran without a binary")
	}
	r := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: "linux/amd64", Version: json.RawMessage(version), Qualification: qual}
	entries, err := tasksArchiveEntries(nil, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Path == "MANIFEST.json" && !bytes.Contains(e.Data, []byte(`"qualification": "`+tasksCrossQualification+`"`)) {
			t.Fatalf("manifest: %s", e.Data)
		}
	}
}

func TestTasksArchiveCrossManifestStatesTheLimit(t *testing.T) {
	r := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: "linux/amd64", Qualification: tasksCrossQualification}
	entries, err := tasksArchiveEntries([]ArchiveEntry{{Path: "bin/corvint-tasks", Mode: 0o755, Data: []byte("b")}}, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Path == "MANIFEST.json" && (!bytes.Contains(e.Data, []byte(tasksCrossQualification)) || bytes.Contains(e.Data, []byte(`"version"`))) {
			t.Fatalf("manifest: %s", e.Data)
		}
		if e.Path == "README.md" && !bytes.Contains(e.Data, []byte("NOT_OBSERVED")) {
			t.Fatal("README omits the runtime limit")
		}
	}
}
