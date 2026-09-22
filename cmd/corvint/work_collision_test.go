package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

func TestDeriveWorkCollisionsResolvesGoModuleImports(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	for name, content := range map[string]string{
		"go.mod": "module example.com/project\n\ngo 1.27.0\n",
		"a/a.go": "package a\n",
		"b/b.go": "package b\n\nimport _ \"example.com/project/a\"\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "module fixture")

	source, err := worksource.Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := source.Close(); err != nil {
			t.Errorf("close work source: %v", err)
		}
	})
	snapshot := &workqueue.Snapshot{
		RepositoryAuthorityID: "repo:corvint",
		QueueAuthorityID:      "queue:corvint:worklist",
		RepositorySource:      source.Identity,
		Tickets: []workqueue.TicketSummary{
			{TicketID: "ticket:corvint:worklist:a", Lifecycle: "READY", TouchPaths: []string{"a/a.go"}},
			{TicketID: "ticket:corvint:worklist:b", Lifecycle: "READY", TouchPaths: []string{"b/b.go"}},
		},
	}
	closure := deriveWorkCollisions(context.Background(), root, snapshot, source.GitPath, source.GitEnvironment)
	if !closure.Complete {
		t.Fatalf("closure incomplete: %#v", closure)
	}
	for _, group := range closure.Groups {
		if group.Path != nil && *group.Path == "b/b.go" && slices.Equal(group.MemberTicketIDs, []string{"ticket:corvint:worklist:a", "ticket:corvint:worklist:b"}) {
			return
		}
	}
	t.Fatalf("importer b/b.go is absent from a/a.go closure: %#v", closure.Groups)
}
