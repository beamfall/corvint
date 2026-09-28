package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Explicit supplemental proof; final nine-component qualification remains a separate gate.
func TestCurrentExportedTasksAndVSIX(t *testing.T) {
	cache := os.Getenv("CORVINT_PROOF_NPM_CACHE")
	if cache == "" {
		t.Skip("requires populated npm cache proof input")
	}
	t.Run("CRB-V0-017 exported-current-tasks-and-vsix", func(t *testing.T) {
		ctx := context.Background()
		scratch := t.TempDir()
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		source, err := exportSource(ctx, "/usr/bin/git", root, scratch)
		if err != nil {
			t.Fatal(err)
		}
		taskExport := tasksExport(source)
		staged, err := stageBuildSource(taskExport, filepath.Join(scratch, "tasks"))
		if err != nil {
			t.Fatal(err)
		}
		built, err := buildComponentTwice(ctx, staged, "./cmd/corvint-tasks", "corvint-tasks", supportedTarget, scratch)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Tasks commit=%s tree=%s binary=%s", taskExport.HeadCommit, taskExport.HeadTree, built.SHA256)
		vsix, err := buildVSIXTwice(ctx, source, scratch, cache)
		if err != nil {
			t.Fatal(err)
		}
		members, err := readVSIXMembers(vsix.Data)
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != len(vsixMembers) {
			t.Fatalf("members=%d, expected %d", len(members), len(vsixMembers))
		}
		// The count plus every current member makes the set exact, so no legacy member is accepted.
		for _, name := range vsixMembers {
			if _, ok := members[name]; !ok {
				t.Fatalf("missing %s", name)
			}
		}
		for _, name := range []string{"extension/dist/src/configuration.js", "extension/media/corvint.svg"} {
			if len(members[name]) == 0 {
				t.Fatalf("empty %s", name)
			}
		}
		if vsix.Path != "extensions/corvint-vscode-0.1.0.vsix" {
			t.Fatal(vsix.Path)
		}
		t.Logf("Core commit=%s tree=%s VSIX=%s members=%d", source.HeadCommit, source.HeadTree, sha256Hex(vsix.Data), len(members))
	})
}

func TestCurrentExportedTasksSmokeGenerator(t *testing.T) {
	ctx := context.Background()
	scratch := t.TempDir()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source, err := exportSource(ctx, "/usr/bin/git", root, scratch)
	if err != nil {
		t.Fatal(err)
	}
	files, err := generateQueuePolicy(ctx, tasksExport(source), scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(files.Queue) == 0 || len(files.Policy) == 0 {
		t.Fatal("empty smoke fixture")
	}
	t.Logf("Tasks commit=%s tree=%s queue=%s policy=%s", source.HeadCommit, source.HeadTree, sha256Hex(files.Queue), sha256Hex(files.Policy))
}

// This archive-only regression runs without npm or a VSIX cache. It uses the
// shipped tar bytes, not stageBuildSource's parallel materialization path.
func TestCurrentTasksSourceArchiveBuildsOffline(t *testing.T) {
	t.Run("PUB-V0-012 PUB-V0-013 actual Tasks archive dependency closure", func(t *testing.T) {
		ctx := context.Background()
		scratch := t.TempDir()
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		source, err := exportSource(ctx, "/usr/bin/git", root, scratch)
		if err != nil {
			t.Fatal(err)
		}
		subset := tasksExport(source)
		for _, name := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
			original, ok := findExportFile(source, name)
			if !ok {
				t.Fatalf("source notice missing: %s", name)
			}
			retained, ok := findExportFile(subset, name)
			if !ok || original.OID != retained.OID || string(original.Data) != string(retained.Data) {
				t.Fatalf("notice changed: %s", name)
			}
		}
		if subset.HeadCommit != source.HeadCommit || subset.HeadTree != source.HeadTree {
			t.Fatal("subset changed immutable identity")
		}
		extract := func(export Export, dir string) string {
			t.Helper()
			files, member, digest, err := assembleModuleSource("corvint-tasks", export)
			if err != nil {
				t.Fatal(err)
			}
			bundle := &VerifiedRetainedBundle{entries: files}
			if err := bundle.ExtractSourceArchive(member, dir); err != nil {
				t.Fatal(err)
			}
			if err := verifyStagedTree(export, dir); err != nil {
				t.Fatal(err)
			}
			return digest
		}
		directory := filepath.Join(scratch, "source")
		archiveDigest := extract(subset, directory)
		goPath, err := goBinary()
		if err != nil {
			t.Fatal(err)
		}
		env := closedGoEnv(filepath.Join(scratch, "home"), "")
		built := runWithStdin(ctx, directory, env, buildTimeout, nil, goPath, "build", "./cmd/corvint-tasks")
		if built.err != nil {
			t.Fatalf("offline archive rebuild: %v", built.err)
		}
		t.Logf("Tasks commit=%s tree=%s archive=%s files=%d; offline go build ./cmd/corvint-tasks PASS", subset.HeadCommit, subset.HeadTree, archiveDigest, len(subset.Files))
		// diagnostic is transitive through contextindex, not a direct Tasks import.
		broken := subset
		broken.Files = nil
		for _, f := range subset.Files {
			if !strings.HasPrefix(f.Path, "internal/diagnostic/") {
				broken.Files = append(broken.Files, f)
			}
		}
		if len(broken.Files) == len(subset.Files) {
			t.Fatal("negative control removed no dependency")
		}
		brokenDir := filepath.Join(scratch, "missing-dependency")
		extract(broken, brokenDir)
		failed := runWithStdin(ctx, brokenDir, env, buildTimeout, nil, goPath, "build", "./cmd/corvint-tasks")
		if failed.err == nil || !strings.Contains(failed.err.Error(), "internal/diagnostic") {
			t.Fatalf("ineffective missing-dependency control: %v", failed.err)
		}
	})
}
