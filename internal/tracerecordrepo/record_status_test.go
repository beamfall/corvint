package tracerecordrepo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func unignoredRecordFixture(t *testing.T) string {
	t.Helper()
	root := newAdapterFixture(t)
	gitAdapterFixture(t, root, "rm", "-q", ".gitignore")
	gitAdapterFixture(t, root, "commit", "-qm", "no ignore prerequisite")
	return root
}
func privateRecordFile(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func privateRecordSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(filepath.Join(root, ".context-corvint"), func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var body []byte
		if info.Mode().IsRegular() {
			body, err = os.ReadFile(path)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			var target string
			target, err = os.Readlink(path)
			body = []byte(target)
		}
		if err != nil {
			return err
		}
		result[strings.TrimPrefix(path, root)] = fmt.Sprintf("%v:%x", info.Mode(), sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestRecordUnignoredUnsafeArtifactsRefuseWithoutMutation(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, string, string)
	}{
		{"nonempty-lock", func(t *testing.T, r, c string) { privateRecordFile(t, r, ".trace-operation.lock", "not a lock") }},
		{"corrupt-canonical", func(t *testing.T, r, c string) { privateRecordFile(t, r, c+".jsonl", "not JSON\n") }},
		{"corrupt-canonical-and-lone-stage", func(t *testing.T, r, c string) {
			privateRecordFile(t, r, c+".jsonl", "not JSON\n")
			privateRecordFile(t, r, ".record-"+c+".tmp", "partial")
		}},
		{"lone-stage", func(t *testing.T, r, c string) { privateRecordFile(t, r, ".record-"+c+".tmp", "partial") }},
		{"unrelated-private-file", func(t *testing.T, r, c string) { privateRecordFile(t, r, "notes.txt", "source") }},
		{"unsafe-member-mode", func(t *testing.T, r, c string) {
			p := privateRecordFile(t, r, ".trace-operation.lock", "")
			if err := os.Chmod(p, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsafe-trace-directory-mode", func(t *testing.T, r, c string) {
			p := privateRecordFile(t, r, ".trace-operation.lock", "")
			if err := os.Chmod(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsafe-parent-mode", func(t *testing.T, r, c string) {
			privateRecordFile(t, r, ".trace-operation.lock", "")
			if err := os.Chmod(filepath.Join(r, ".context-corvint"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"hardlink", func(t *testing.T, r, c string) {
			p := privateRecordFile(t, r, ".trace-operation.lock", "")
			if err := os.Link(p, filepath.Join(t.TempDir(), "outside")); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink-member", func(t *testing.T, r, c string) {
			p := privateRecordFile(t, r, ".trace-operation.lock", "")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), p); err != nil {
				t.Fatal(err)
			}
		}},
		{"tracked-clean-artifact", func(t *testing.T, r, c string) {
			privateRecordFile(t, r, ".trace-operation.lock", "")
			gitAdapterFixture(t, r, "add", ".context-corvint/traces/.trace-operation.lock")
			gitAdapterFixture(t, r, "commit", "-qm", "tracked lock")
		}},
	}
	for _, tc := range cases {
		t.Run("GPK-V0-008 LTPM-V0-008 unsafe "+tc.name, func(t *testing.T) {
			root := unignoredRecordFixture(t)
			commit := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			tc.prepare(t, root, commit)
			before := privateRecordSnapshot(t, root)
			status := gitAdapterFixture(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
			_, err := Record(context.Background(), root, Input{Producer: trace.ProducerCLI, Task: "record", ChangedPaths: []string{"internal/value.go"}, Verification: []string{"true"}, Outcome: "passed"})
			if err == nil {
				t.Fatal("unsafe artifact accepted")
			}
			if after := privateRecordSnapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatalf("failed record mutated bytes/modes/entries: before=%v after=%v error=%v", before, after, err)
			}
			if after := gitAdapterFixture(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all"); after != status {
				t.Fatalf("Git status changed: %q -> %q", status, after)
			}
		})
	}
}
func TestRecordStatusExpansionRequiresExactDigestAndUntrackedArtifacts(t *testing.T) {
	path := ".context-corvint/traces/.trace-operation.lock"
	for _, raw := range []string{" M " + path + "\x00", "A  " + path + "\x00", "R  " + path + "\x00source.go\x00", "?? unrelated.go\x00", "?? .context-corvint/traces/nested/file\x00", "?? " + path, "?? " + path + "\x00\x00"} {
		if _, err := privateStatusPaths([]byte(raw), fmt.Sprintf("%x", sha256.Sum256([]byte(raw))), 40); err == nil {
			t.Errorf("admitted status %q", raw)
		}
	}
	raw := []byte("?? " + path + "\x00")
	if _, err := privateStatusPaths(raw, fmt.Sprintf("%x", sha256.Sum256(nil)), 40); err == nil {
		t.Fatal("status expansion drift accepted")
	}
	if _, err := privateStatusPaths(raw, fmt.Sprintf("%x", sha256.Sum256(raw)), 40); err != nil {
		t.Fatal(err)
	}
}
func TestRecorderStabilityKeepsSourceAndCommitDriftVisible(t *testing.T) {
	for _, dogfood := range []bool{false, true} {
		for _, kind := range []string{"same-tree-commit", "changed-tree", "untracked-source", "tracked-source", "tracked-private"} {
			t.Run(fmt.Sprintf("LTPM-V0-001-dogfood-%v-%s", dogfood, kind), func(t *testing.T) {
				root := unignoredRecordFixture(t)
				if kind == "tracked-private" {
					writeAdapterFixture(t, root, ".context-corvint/source.go", "package private\n")
					gitAdapterFixture(t, root, "add", ".")
					gitAdapterFixture(t, root, "commit", "-qm", "private source")
				}
				base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
				index, err := contextindex.Build(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				var authority *dogfoodAuthority
				if dogfood {
					authority = &dogfoodAuthority{base: base, target: base, candidates: digestPaths(nil), admitted: digestPaths(nil)}
				}
				stable := stabilityCheck(context.Background(), root, index, authority)
				if err := stable(); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "same-tree-commit":
					gitAdapterFixture(t, root, "commit", "--allow-empty", "-qm", "moved HEAD")
				case "changed-tree":
					writeAdapterFixture(t, root, "new.go", "package p\n")
					gitAdapterFixture(t, root, "add", "new.go")
					gitAdapterFixture(t, root, "commit", "-qm", "new tree")
				case "untracked-source":
					writeAdapterFixture(t, root, "unrelated.txt", "new\n")
				case "tracked-source":
					writeAdapterFixture(t, root, "internal/value.go", "package internal\nconst Changed = true\n")
				case "tracked-private":
					writeAdapterFixture(t, root, ".context-corvint/source.go", "package private\nconst Changed = true\n")
				}
				if err := stable(); err == nil {
					t.Fatal("real repository drift accepted")
				}
			})
		}
	}
}
func TestRecordStagingAdmissionRequiresOwnedAppendPhase(t *testing.T) {
	root := unignoredRecordFixture(t)
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	privateRecordFile(t, root, ".trace-operation.lock", "")
	privateRecordFile(t, root, ".record-"+index.CommitRevision+".tmp", "partial")
	before := privateRecordSnapshot(t, root)
	if err := trace.RecoverInterruptedAppend(root, stabilityCheck(context.Background(), root, index, nil), func() error { return nil }); err == nil {
		t.Fatal("post-index injected unowned stage reached recovery")
	}
	if after := privateRecordSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("injected stage changed: before=%v after=%v", before, after)
	}
}

func TestRecordLivePrivateStatusPhases(t *testing.T) {
	for _, dogfood := range []bool{false, true} {
		t.Run(fmt.Sprintf("LTPM-V0-001-dogfood-%v", dogfood), func(t *testing.T) {
			root := unignoredRecordFixture(t)
			index, err := contextindex.Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			var authority *dogfoodAuthority
			if dogfood {
				authority = &dogfoodAuthority{base: index.CommitRevision, target: index.CommitRevision, candidates: digestPaths(nil), admitted: digestPaths(nil)}
			}
			stable := stabilityCheck(context.Background(), root, index, authority)
			privateRecordFile(t, root, ".trace-operation.lock", "")
			if err := stable(); err != nil {
				t.Fatalf("own empty lock rejected: %v", err)
			}
			name := ".record-" + index.CommitRevision + ".tmp"
			path := privateRecordFile(t, root, name, "temporary bytes")
			if err := stable(); err == nil {
				t.Fatal("unowned stage accepted")
			}
			if err := stagingStabilityCheck(context.Background(), root, index, authority)(".context-corvint/traces/" + name); err != nil {
				t.Fatalf("exact owned stage rejected: %v", err)
			}
			if err := stagingStabilityCheck(context.Background(), root, index, authority)(".context-corvint/traces/.record-" + strings.Repeat("0", 40) + ".tmp"); err == nil {
				t.Fatal("another stage authority accepted")
			}
			if err := os.Rename(path, filepath.Join(filepath.Dir(path), index.CommitRevision+".jsonl")); err != nil {
				t.Fatal(err)
			}
			if err := stable(); err != nil {
				t.Fatalf("canonical filename metadata rejected: %v", err)
			}
			// Status admission intentionally does not validate canonical content: Record must.
			before := privateRecordSnapshot(t, root)
			if _, err := Record(context.Background(), root, Input{Producer: trace.ProducerCLI, Task: "record", ChangedPaths: []string{"internal/value.go"}, Outcome: "passed"}); err == nil {
				t.Fatal("status classification bypassed row validation")
			}
			if after := privateRecordSnapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("corrupt row refusal mutated private state")
			}
		})
	}
}

func TestDogfoodRecordFreshUnignoredRepeat(t *testing.T) {
	t.Run("LTPM-V0-011-fresh-repeat", func(t *testing.T) {
		root := unignoredRecordFixture(t)
		base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		writeAdapterFixture(t, root, "internal/value.go", "package internal\nconst Changed = true\n")
		gitAdapterFixture(t, root, "add", "internal/value.go")
		gitAdapterFixture(t, root, "commit", "-qm", "change")
		target := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		input := Input{Producer: trace.ProducerCLI, Task: "record change", Verification: []string{"true"}, Outcome: "passed"}
		first, err := RecordDogfood(context.Background(), root, base, target, input)
		if err != nil {
			t.Fatal(err)
		}
		second, err := RecordDogfood(context.Background(), root, base, target, input)
		if err != nil {
			t.Fatal(err)
		}
		if first.Recorded == nil || second.Recorded == nil || first.Recorded.Record.TraceID != second.Recorded.Record.TraceID {
			t.Fatalf("repeat differs: %v %v", first, second)
		}
		body, err := os.ReadFile(first.Recorded.Store)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(body), "\n") != 1 {
			t.Fatalf("repeat rows=%q", body)
		}
	})
}

func TestReadKeepsExactStatusBindingAfterRecorderRepair(t *testing.T) {
	root := unignoredRecordFixture(t)
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	privateRecordFile(t, root, ".trace-operation.lock", "")
	if _, _, err := Read(context.Background(), root, index); err == nil {
		t.Fatal("read lost exact status binding")
	}
}
