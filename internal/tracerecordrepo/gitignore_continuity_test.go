package tracerecordrepo

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

func TestGitignoreRecordReadContinuity(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("GIT_DEFAULT_HASH", format)
			for _, path := range []string{".gitignore", "config/local/.gitignore"} {
				t.Run("GPK-V0-050 "+path, func(t *testing.T) {
					ctx := context.Background()
					root := newAdapterFixture(t)
					writeAdapterFixture(t, root, path, ".context-corvint/\n*.tmp\n")
					gitAdapterFixture(t, root, "add", path)
					gitAdapterFixture(t, root, "commit", "-qm", "ignore rules")
					result, err := Record(ctx, root, Input{Producer: trace.ProducerCLI, Task: "ignore rules", ChangedPaths: []string{path}, Outcome: "passed"})
					if err != nil {
						t.Fatal(err)
					}
					for _, stage := range []string{"current", "same-tree ancestor", "historical tree"} {
						if stage == "same-tree ancestor" {
							gitAdapterFixture(t, root, "commit", "--allow-empty", "-qm", stage)
						} else if stage == "historical tree" {
							writeAdapterFixture(t, root, "internal/value.go", "package internal\nconst Value = 2\n")
							gitAdapterFixture(t, root, "add", "internal/value.go")
							gitAdapterFixture(t, root, "commit", "-qm", stage)
						}
						t.Run(stage, func(t *testing.T) {
							index, err := contextindex.Build(ctx, root)
							if err != nil {
								t.Fatal(err)
							}
							if _, indexed := index.Sources[path]; indexed {
								t.Fatalf("%s entered index sources", path)
							}
							for _, symbol := range index.Symbols {
								if symbol.Path == path {
									t.Fatalf("%s contributed symbol %+v", path, symbol)
								}
							}
							assertGitignoreRead(t, root, index, result.Record)
							if _, err := contextindex.WriteSnapshot(index); err != nil {
								t.Fatal(err)
							}
							loaded, hit, err := contextindex.LoadSnapshot(ctx, root)
							if err != nil || !hit {
								t.Fatalf("snapshot hit=%v error=%v", hit, err)
							}
							opening, err := contextindex.Observe(ctx, root)
							if err != nil {
								t.Fatal(err)
							}
							records, state, err := ReadObserved(ctx, root, loaded, opening)
							if err != nil || state != "ready" || !reflect.DeepEqual(records, []trace.Record{result.Record}) {
								t.Fatalf("observed records=%+v state=%q error=%v", records, state, err)
							}
						})
					}
				})
			}
		})
	}
}

func assertGitignoreRead(t *testing.T, root string, index *contextindex.Index, want trace.Record) {
	t.Helper()
	records, state, err := Read(context.Background(), root, index)
	if err != nil || state != "ready" || !reflect.DeepEqual(records, []trace.Record{want}) {
		t.Fatalf("records=%+v state=%q error=%v", records, state, err)
	}
}
