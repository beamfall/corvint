package gitauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func streamRecord(kind string, body []byte, width int) (string, []byte) {
	oid := hex.EncodeToString(integrityHash(width, append([]byte(fmt.Sprintf("%s %d\x00", kind, len(body))), body...)))
	out := []byte(fmt.Sprintf("%s %s %d\n", oid, kind, len(body)))
	out = append(out, body...)
	return oid, append(out, '\n')
}

func TestCommitStreamSplitFramesAndBoundedMemory(t *testing.T) {
	t.Run("CEM-CB-023 streamed commit identity preserves tree bytes", func(t *testing.T) {
		for _, width := range []int{40, 64} {
			tree, treeRecord := streamRecord("tree", []byte("tree body"), width)
			_, commit := streamRecord("commit", []byte("tree "+tree+"\n\nmessage\n"), width)
			input := append(commit, treeRecord...)
			for split := range len(input) + 1 {
				s := &commitStream{count: 1}
				for _, part := range [][]byte{input[:split], input[split:]} {
					if _, err := s.Write(part); err != nil {
						t.Fatalf("width=%d split=%d: %v", width, split, err)
					}
				}
				if err := s.finish(); err != nil || !bytes.Equal(s.tail, treeRecord) || s.objects[0].tree != tree {
					t.Fatalf("width=%d split=%d: %v", width, split, err)
				}
			}
		}
	})
	t.Run("CEM-CB-023 commit message retention is bounded", func(t *testing.T) {
		tree, tail := streamRecord("tree", []byte("tree body"), 40)
		body := append([]byte("tree "+tree+"\n\n"), bytes.Repeat([]byte{'m'}, 16<<20)...)
		_, input := streamRecord("commit", body, 40)
		s := &commitStream{count: 1}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for len(input) > 0 {
			n := min(len(input), 32<<10)
			if _, err := s.Write(input[:n]); err != nil {
				t.Fatal(err)
			}
			input = input[n:]
		}
		if _, err := s.Write(tail); err != nil {
			t.Fatal(err)
		}
		if err := s.finish(); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 256<<10 {
			t.Fatalf("stream allocated %d bytes for a 16MiB commit", allocated)
		}
		if cap(s.header) > 512 || cap(s.firstLine) > 128 || cap(s.objects) != 1 || len(s.tail) != len(tail) {
			t.Fatalf("retention: header=%d first=%d objects=%d tail=%d", cap(s.header), cap(s.firstLine), cap(s.objects), len(s.tail))
		}
	})
}

func TestCommitStreamRefusesMalformedFrames(t *testing.T) {
	tree := strings.Repeat("a", 40)
	_, good := streamRecord("commit", []byte("tree "+tree+"\n\nmessage\n"), 40)
	_, badHeader := streamRecord("commit", []byte("parent "+tree+"\n"), 40)
	for name, input := range map[string][]byte{
		"truncated-header": good[:20], "truncated-body": good[:len(good)-2],
		"missing-separator": good[:len(good)-1], "invalid-first-header": badHeader,
		"long-header":   bytes.Repeat([]byte{'x'}, 513),
		"size-overflow": []byte(tree + " commit 9223372036854775808\n"),
		"wrong-kind":    []byte(tree + " blob 0\n\n"),
		"hash-mismatch": bytes.Replace(good, []byte("message"), []byte("changed"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			s := &commitStream{count: 1}
			_, err := s.Write(input)
			if err == nil {
				err = s.finish()
			}
			if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("got %v", err)
			}
		})
	}
	t.Run("maximum missing echo and trailing records", func(t *testing.T) {
		request := strings.Repeat("r", 256) + "^{commit}"
		s := &commitStream{count: 1, requests: []string{request}}
		if _, err := s.Write([]byte(request + " missing\n")); err != nil {
			t.Fatal(err)
		}
		if err := s.finish(); err != nil || !s.objects[0].missing {
			t.Fatalf("missing peel: %v", err)
		}
		if _, err := exactBatchRecords(append(good, good...), 1); err == nil {
			t.Fatal("extra record accepted")
		}
		if _, err := exactBatchRecords([]byte(tree+" tree 9223372036854775807\n\n"), 1); err == nil {
			t.Fatal("overflowing tail frame accepted")
		}
		s = &commitStream{count: 1, requests: []string{request}}
		if _, err := s.Write([]byte("different missing\n")); err == nil {
			t.Fatal("wrong missing echo accepted")
		}
	})
}

func TestCommitTreeLinkRejectsValidUnrelatedObjects(t *testing.T) {
	t.Run("CEM-CB-023 verified commit and root must be related", func(t *testing.T) {
		for _, width := range []int{40, 64} {
			a, _ := streamRecord("tree", nil, width)
			blob, _ := streamRecord("blob", []byte("new\n"), width)
			rawOID, _ := hex.DecodeString(blob)
			_, otherTree := streamRecord("tree", append([]byte("100644 f\x00"), rawOID...), width)
			_, commit := streamRecord("commit", []byte("tree "+a+"\n\nmessage\n"), width)
			s := &commitStream{count: 2}
			if _, err := s.Write(append(commit, otherTree...)); err != nil {
				t.Fatal(err)
			}
			if err := s.finish(); err != nil {
				t.Fatal(err)
			}
			if err := requireCommitTree(s.objects[0], s.objects[1].oid); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("unrelated root accepted: %v", err)
			}
		}
	})
}

func TestCanonicalDiffPreservesSymbolicRevisions(t *testing.T) {
	t.Run("CEM-CB-023 symbolic revisions retain Git-owned selection", func(t *testing.T) {
		for _, format := range []string{"sha1", "sha256"} {
			t.Run(format, func(t *testing.T) {
				root, target := pathsRepo(t, format)
				base := integrityOID(t, root, target+"^")
				want := rawCanonicalDiff(t, root, base, target)
				otherWidth := 64
				if format == "sha256" {
					otherWidth = 40
				}
				hexRef := strings.Repeat("a", otherWidth)
				integrityGit(t, root, "update-ref", "refs/heads/"+hexRef, target)
				for _, revision := range []string{"HEAD", hexRef} {
					got, err := open(t, root).CanonicalDiff(context.Background(), base, revision)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("symbolic target %q differs from pinned target: %v", revision, err)
					}
				}
			})
		}
	})
}

func TestVerifiedCommitTreesRejectsUnpinnedCommit(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, target := pathsRepo(t, format)
			base := integrityOID(t, root, target+"^")
			tree := integrityOID(t, root, target+"^{tree}")
			_, commitFrame := streamRecord("commit", integrityGit(t, root, "cat-file", "commit", target), len(target))
			_, treeFrame := streamRecord("tree", integrityGit(t, root, "cat-file", "tree", tree), len(target))
			payload := bytes.Join([][]byte{commitFrame, commitFrame, treeFrame, treeFrame}, nil)
			repo := open(t, root)
			dir := t.TempDir()
			path := filepath.Join(dir, "output")
			if err := os.WriteFile(path, payload, 0600); err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nexec /bin/cat '" + strings.ReplaceAll(path, "'", "'\\''") + "'\n"
			if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if _, err := repo.verifiedCommitTrees(context.Background(), []string{target, target}); err != nil {
				t.Fatalf("honest frame control: %v", err)
			}
			if _, err := repo.verifiedCommitTrees(context.Background(), []string{base, target}); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("different pinned commit accepted: %v", err)
			}
		})
	}
}

func TestCommitVerificationLargeMessagesAndTreeishParity(t *testing.T) {
	t.Run("CEM-CB-023 streamed commit reads preserve admitted treeish inputs", func(t *testing.T) {
		for _, format := range []string{"sha1", "sha256"} {
			root, seed := pathsRepo(t, format)
			tree := integrityOID(t, root, seed+"^{tree}")
			body := []byte("tree " + tree + "\nauthor Fixture <fixture@example.invalid> 1 +0000\ncommitter Fixture <fixture@example.invalid> 1 +0000\n\n" + strings.Repeat("m", 4<<20) + "\n")
			out, err := open(t, root).gitStdin(context.Background(), 256, body, "hash-object", "-w", "-t", "commit", "--stdin")
			if err != nil {
				t.Fatal(err)
			}
			commit := strings.TrimSpace(string(out))
			integrityGit(t, root, "update-ref", "refs/heads/large", commit)
			integrityGit(t, root, "tag", "-am", "tag", "annotated", commit)
			repo := open(t, root)
			for _, revision := range []string{commit, "large", tree, "annotated", integrityOID(t, root, "annotated")} {
				entry, exists, err := repo.LookupTreeEntry(context.Background(), revision, "pkg/d/inner.txt")
				if err != nil || !exists || entry.OID != integrityOID(t, root, seed+":pkg/d/inner.txt") {
					t.Fatalf("%s %s: %v %v", format, revision, exists, err)
				}
				if len(revision) == len(commit) {
					got, err := repo.CommitTree(context.Background(), revision)
					if err != nil || got != tree {
						t.Fatalf("CommitTree %s: %s %v", revision, got, err)
					}
				}
			}
		}
	})
}

func TestCommitVerificationRejectsLooseAndPackedCorruption(t *testing.T) {
	t.Run("CEM-CB-023 commit corruption refuses every verified root reader", func(t *testing.T) {
		for _, format := range []string{"sha1", "sha256"} {
			for _, packed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s-packed=%v", format, packed), func(t *testing.T) {
					root, commit := pathsRepo(t, format)
					raw := integrityGit(t, root, "cat-file", "commit", commit)
					raw = append(raw, []byte("changed message\n")...)
					if packed {
						corruptPackedObject(t, root, commit, raw, 1)
					} else {
						corruptLoose(t, root, commit, "commit", raw)
					}
					repo := open(t, root)
					_, treeErr := repo.CommitTree(context.Background(), commit)
					_, _, pathErr := repo.LookupTreeEntry(context.Background(), commit, "pkg/d/inner.txt")
					_, baseErr := repo.verifiedCommitTrees(context.Background(), []string{commit, commit})
					for _, err := range []error{treeErr, pathErr, baseErr} {
						if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
							t.Fatalf("corruption: %v", err)
						}
					}
				})
			}
		}
	})
}

func TestCommitVerificationPreservesGitChildBudget(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, commit := pathsRepo(t, format)
			repo := open(t, root)
			count := countMemoGit(t)
			if _, err := repo.CommitTree(context.Background(), commit); err != nil || count() != 1 {
				t.Fatalf("CommitTree children=%d: %v", count(), err)
			}
			if _, _, err := repo.LookupTreeEntry(context.Background(), commit, "pkg/d/inner.txt"); err != nil || count() != 2 {
				t.Fatalf("LookupTreeEntry children=%d: %v", count(), err)
			}
			if _, err := repo.verifiedChangeSet(context.Background(), commit, commit); err != nil || count() != 3 {
				t.Fatalf("verifiedChangeSet children=%d: %v", count(), err)
			}
		})
	}
}

func TestCommitTreeStreamsLargeRoot(t *testing.T) {
	root, seed := pathsRepo(t, "sha1")
	blob, _ := hex.DecodeString(integrityOID(t, root, seed+":other.txt"))
	var body bytes.Buffer
	for i := 0; body.Len() <= MaxTreeBytes; i++ {
		fmt.Fprintf(&body, "100644 f%06d\x00", i)
		body.Write(blob)
	}
	repo := open(t, root)
	out, err := repo.gitStdin(context.Background(), 256, body.Bytes(), "hash-object", "-w", "-t", "tree", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(string(out))
	commit := strings.TrimSpace(string(integrityGit(t, root, "commit-tree", tree, "-m", "wide")))
	got, err := repo.CommitTree(context.Background(), commit)
	if err != nil || got != tree {
		t.Fatalf("large root: %s %v", got, err)
	}
	if _, _, err := repo.LookupTreeEntry(context.Background(), commit, "f000000"); cemcode.CodeOf(err) != cemcode.GitOutputExceeded {
		t.Fatalf("lookup tree bound changed: %v", err)
	}
}
