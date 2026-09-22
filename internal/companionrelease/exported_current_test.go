package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Explicit supplemental proof; final nine-component qualification remains a separate gate.
func TestCurrentExportedTasksAndVSIX(t *testing.T) {
	tasks, cache := os.Getenv("CORVINT_PROOF_TASKS_ROOT"), os.Getenv("CORVINT_PROOF_NPM_CACHE")
	if tasks == "" || cache == "" {
		t.Skip("requires explicit exported Tasks and populated npm cache proof inputs")
	}
	t.Run("CRB-V0-017 exported-current-tasks-and-vsix", func(t *testing.T) {
		ctx := context.Background()
		scratch := t.TempDir()
		taskExport, err := exportSource(ctx, "/usr/bin/git", tasks, scratch)
		if err != nil {
			t.Fatal(err)
		}
		staged, err := stageBuildSource(taskExport, filepath.Join(scratch, "tasks"))
		if err != nil {
			t.Fatal(err)
		}
		built, err := buildComponentTwice(ctx, staged, "./cmd/corvint-tasks", "corvint-tasks", supportedTarget, scratch)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Tasks commit=%s tree=%s binary=%s", taskExport.HeadCommit, taskExport.HeadTree, built.SHA256)
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		source, err := exportSource(ctx, "/usr/bin/git", root, scratch)
		if err != nil {
			t.Fatal(err)
		}
		vsix, err := buildVSIXTwice(ctx, source, scratch, cache)
		if err != nil {
			t.Fatal(err)
		}
		members, err := readVSIXMembers(vsix.Data)
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 21 {
			t.Fatalf("members=%d, expected21", len(members))
		}
		for _, name := range []string{"extension/dist/src/configuration.js", "extension/media/corvint.svg"} {
			if len(members[name]) == 0 {
				t.Fatalf("missing %s", name)
			}
		}
		if _, old := members["extension/media/corvint.svg"]; old {
			t.Fatal("legacy icon accepted")
		}
		if vsix.Path != "extensions/corvint-vscode-0.1.0.vsix" {
			t.Fatal(vsix.Path)
		}
		t.Logf("Core commit=%s tree=%s VSIX=%s members=%d", source.HeadCommit, source.HeadTree, sha256Hex(vsix.Data), len(members))
	})
}

func TestCurrentExportedTasksSmokeGenerator(t *testing.T) {
	tasks := os.Getenv("CORVINT_PROOF_TASKS_ROOT")
	if tasks == "" {
		t.Skip("requires explicit exported Tasks source proof input")
	}
	ctx := context.Background()
	scratch := t.TempDir()
	source, err := exportSource(ctx, "/usr/bin/git", tasks, scratch)
	if err != nil {
		t.Fatal(err)
	}
	files, err := generateQueuePolicy(ctx, source, scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(files.Queue) == 0 || len(files.Policy) == 0 {
		t.Fatal("empty smoke fixture")
	}
	t.Logf("Tasks commit=%s tree=%s queue=%s policy=%s", source.HeadCommit, source.HeadTree, sha256Hex(files.Queue), sha256Hex(files.Policy))
}
