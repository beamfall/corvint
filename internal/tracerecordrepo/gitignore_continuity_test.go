package tracerecordrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracemigraterepo"
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

func TestGitignoreLegacyMigrationContinuity(t *testing.T) {
	t.Run("GPK-V0-050 legacy root and nested ignore rules retain original bytes", func(t *testing.T) {
		ctx := context.Background()
		root := newAdapterFixture(t)
		writeAdapterFixture(t, root, "nested/.gitignore", "*.tmp\n")
		gitAdapterFixture(t, root, "add", "nested/.gitignore")
		gitAdapterFixture(t, root, "commit", "-qm", "nested ignore rules")
		commit := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		tree := gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
		paths := []string{".gitignore", "nested/.gitignore"}
		input := trace.Input{Producer: trace.ProducerCLI, Revision: tree, TreeRevision: tree, Task: "legacy ignore rules", ChangedPaths: paths, Outcome: "passed"}
		legacy, err := trace.NewRecord(input, paths)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := trace.Encode(legacy)
		if err != nil {
			t.Fatal(err)
		}
		directory := filepath.Join(root, ".context-corvint", "traces")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(directory, tree+".jsonl")
		if err := os.WriteFile(source, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		dry, err := tracemigraterepo.Evaluate(ctx, root, tracemigraterepo.Options{})
		if err != nil || dry.Mutates || dry.LegacyTraceFiles != 1 || dry.TraceRows != 1 {
			t.Fatalf("dry=%+v error=%v", dry, err)
		}
		preserved, err := os.ReadFile(source)
		entries, listErr := os.ReadDir(directory)
		if err != nil || listErr != nil || !bytes.Equal(preserved, raw) || len(entries) != 1 {
			t.Fatalf("dry-run changed trace directory: read=%v list=%v entries=%v", err, listErr, entries)
		}
		applied, err := tracemigraterepo.Evaluate(ctx, root, tracemigraterepo.Options{Apply: true, PlanDigest: &dry.PlanDigest})
		if err != nil || !applied.Mutates || applied.LegacyTraceFiles != 1 {
			t.Fatalf("apply=%+v error=%v", applied, err)
		}
		archived, err := os.ReadFile(filepath.Join(root, ".context-corvint", "legacy-traces", tree+".jsonl"))
		if err != nil || !bytes.Equal(archived, raw) {
			t.Fatalf("original bytes not retained: %v", err)
		}
		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Fatalf("legacy source remains: %v", err)
		}
		input.Revision = commit
		want, err := trace.NewRecord(input, paths)
		if err != nil {
			t.Fatal(err)
		}
		index, err := contextindex.Build(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		assertGitignoreRead(t, root, index, want)
	})
}
