package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testFixture = `{
  "format": "atlas-cli-parity-fixture/0",
  "files": [
    {"path":"AGENTS.md","type":"file","mode":"0644","content":"# Fixture authority\n"},
    {"path":"bin","type":"directory","mode":"0755"},
    {"path":"bin/tool","type":"file","mode":"0755","content":"#!/bin/sh\nexit 0\n"},
	{"path":"src/oracle.py","type":"file","mode":"0644","content":"VALUE = 1\n"},
    {"path":"authority","type":"symlink","mode":"0777","target":"AGENTS.md"}
  ]
}`

func TestFixtureMaterializationsAreIndependentAndIdentical(t *testing.T) {
	ctx := context.Background()
	fixtures := writeTestFixture(t, "basic", testFixture)
	first, err := materializeFixture(ctx, fixtures, "basic", filepath.Join(t.TempDir(), "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := materializeFixture(ctx, fixtures, "basic", filepath.Join(t.TempDir(), "second"))
	if err != nil {
		t.Fatal(err)
	}
	if first.CommitRevision != second.CommitRevision || first.TreeRevision != second.TreeRevision || first.FixtureSHA256 != second.FixtureSHA256 {
		t.Fatalf("materializations differ: first=%#v second=%#v", first, second)
	}
	firstSnapshot, err := snapshotRepository(ctx, first.Root)
	if err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := snapshotRepository(ctx, second.Root)
	if err != nil {
		t.Fatal(err)
	}
	if firstSnapshot.SHA256 != secondSnapshot.SHA256 || firstSnapshot.StatusSHA256 != secondSnapshot.StatusSHA256 {
		for index := range firstSnapshot.Entries {
			if index >= len(secondSnapshot.Entries) || digest(firstSnapshot.Entries[index].Content) != digest(secondSnapshot.Entries[index].Content) || firstSnapshot.Entries[index].Path != secondSnapshot.Entries[index].Path || firstSnapshot.Entries[index].Mode != secondSnapshot.Entries[index].Mode {
				t.Logf("first differing entries: first=%#v/%s second=%#v/%s", firstSnapshot.Entries[index], digest(firstSnapshot.Entries[index].Content), secondSnapshot.Entries[index], digest(secondSnapshot.Entries[index].Content))
				break
			}
		}
		t.Fatalf("snapshots differ: first=%s/%s second=%s/%s", firstSnapshot.SHA256, firstSnapshot.StatusSHA256, secondSnapshot.SHA256, secondSnapshot.StatusSHA256)
	}
	if err := applyFixtureMutations(ctx, first.Root, []fixtureMutation{{Path: "AGENTS.md", Content: "changed\n"}}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(second.Root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "# Fixture authority\n" {
		t.Fatalf("second fixture shared state: %q", contents)
	}
}

func TestFixtureNeedsNoPostInitializationGitRewrite(t *testing.T) {
	ctx := context.Background()
	fixtures := writeTestFixture(t, "no-rewrite", testFixture)
	fixture, err := materializeFixture(ctx, fixtures, "no-rewrite", filepath.Join(t.TempDir(), "repository"))
	if err != nil {
		t.Fatal(err)
	}
	git, err := newSanitizedGit(ctx, fixture.Root)
	if err != nil {
		t.Fatal(err)
	}
	assertUnchanged := func(name, path string, command ...string) {
		t.Helper()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := git.run(ctx, command...); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%s changed bytes", name)
		}
	}
	assertUnchanged("config", filepath.Join(fixture.Root, ".git", "config"), "config", "core.filemode", "true")
	assertUnchanged("index", filepath.Join(fixture.Root, ".git", "index"), "read-tree", "HEAD")
}

func TestFixtureFastImportPreservesObservedGitLayout_GPKV0005(t *testing.T) {
	t.Run("GPK-V0-005 exact fixture Git layout", testFixtureFastImportPreservesObservedGitLayout)
}

func testFixtureFastImportPreservesObservedGitLayout(t *testing.T) {
	ctx := context.Background()
	fixtures := writeTestFixture(t, "fast-import-layout", testFixture)
	fixture, err := materializeFixture(ctx, fixtures, "fast-import-layout", filepath.Join(t.TempDir(), "repository"))
	if err != nil {
		t.Fatal(err)
	}
	packEntries, err := os.ReadDir(filepath.Join(fixture.Root, ".git", "objects", "pack"))
	if err != nil {
		t.Fatal(err)
	}
	if len(packEntries) != 0 {
		t.Fatalf("fast-import retained packed objects: %v", packEntries)
	}
	wantReflog := strings.Repeat("0", 40) + " " + fixture.CommitRevision + " Atlas Fixture <fixture@atlas.invalid> 946684800 +0000\n"
	for _, relative := range []string{"logs/HEAD", "logs/refs/heads/main"} {
		raw, err := os.ReadFile(filepath.Join(fixture.Root, ".git", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != wantReflog {
			t.Fatalf("%s=%q, want %q", relative, raw, wantReflog)
		}
	}
}

// TestFixtureInProcessIndexMatchesGitIndex_GPKV0005 admits the in-process index
// only while every manifest fixture, plus nested and empty edge shapes, keeps the
// identities and every snapshot byte that Git's own index construction produced.
func TestFixtureInProcessIndexMatchesGitIndex_GPKV0005(t *testing.T) {
	ctx := context.Background()
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "fixtures")
	repositories := map[string]string{}
	for _, item := range manifest.Cases {
		repositories[item.Repository] = fixtures
	}
	for _, item := range manifest.Refusals {
		repositories[item.Repository] = fixtures
	}
	edges := writeTestFixture(t, "nested", `{"format":"atlas-cli-parity-fixture/0","parentFiles":[{"path":"old","type":"file","mode":"0644","content":"x"}],"files":[
    {"path":"zz/a","type":"file","mode":"0644","content":"1"},{"path":"a-long/b/c","type":"symlink","mode":"0777","target":"d"},
    {"path":"a-long/b/d","type":"file","mode":"0755","content":""},{"path":"b/c","type":"file","mode":"0644","content":"2"},
    {"path":"b.txt","type":"file","mode":"0644","content":"3"},{"path":"b0","type":"file","mode":"0644","content":"4"},
    {"path":"empty","type":"directory","mode":"0755"},{"path":"loose","type":"untracked-file","mode":"0644","content":"5"}]}`)
	repositories["nested"] = edges
	emptyRoot := writeTestFixture(t, "untracked-only", `{"format":"atlas-cli-parity-fixture/0","files":[{"path":"note","type":"untracked-file","mode":"0644","content":"n"}]}`)
	repositories["untracked-only"] = emptyRoot
	if len(repositories) < 3 {
		t.Fatalf("manifest names %d fixtures", len(repositories))
	}
	for id, root := range repositories {
		t.Run(id, func(t *testing.T) {
			parent := t.TempDir()
			got, err := materializeFixture(ctx, root, id, filepath.Join(parent, "in-process"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := materializeFixtureWithGitIndex(ctx, root, id, filepath.Join(parent, "git"))
			if err != nil {
				t.Fatal(err)
			}
			if got.CommitRevision != want.CommitRevision || got.TreeRevision != want.TreeRevision || got.FixtureSHA256 != want.FixtureSHA256 {
				t.Fatalf("identities=%#v, want %#v", got, want)
			}
			gotSnapshot, err := snapshotRepository(ctx, got.Root)
			if err != nil {
				t.Fatal(err)
			}
			wantSnapshot, err := snapshotRepository(ctx, want.Root)
			if err != nil {
				t.Fatal(err)
			}
			if gotSnapshot.SHA256 == wantSnapshot.SHA256 {
				return
			}
			for index, entry := range wantSnapshot.Entries {
				if index >= len(gotSnapshot.Entries) || gotSnapshot.Entries[index].Path != entry.Path || !bytes.Equal(gotSnapshot.Entries[index].Content, entry.Content) || gotSnapshot.Entries[index].Mode != entry.Mode {
					t.Fatalf("first differing snapshot entry %q (in-process has %d entries, Git %d)", entry.Path, len(gotSnapshot.Entries), len(wantSnapshot.Entries))
				}
			}
			t.Fatalf("snapshot=%s status=%q, Git snapshot=%s status=%q", gotSnapshot.SHA256, gotSnapshot.Status, wantSnapshot.SHA256, wantSnapshot.Status)
		})
	}
}

// materializeFixtureWithGitIndex is the retired Git-launched construction: blobs
// from hash-object and the index from update-index before fast-import, then the
// cache-tree from write-tree. It is the byte oracle for writeFixtureIndex.
func materializeFixtureWithGitIndex(ctx context.Context, fixturesRoot, id, destination string) (materializedFixture, error) {
	spec, fixtureSHA256, err := loadFixture(fixturesRoot, id)
	if err != nil {
		return materializedFixture{}, err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return materializedFixture{}, err
	}
	if err := materializeEntries(destination, spec.Files); err != nil {
		return materializedFixture{}, err
	}
	template, err := os.MkdirTemp(filepath.Dir(destination), ".corvint-empty-git-template-")
	if err != nil {
		return materializedFixture{}, err
	}
	defer os.RemoveAll(template)
	if err := os.WriteFile(filepath.Join(template, "config"), []byte(fixtureGitConfig), 0o666); err != nil {
		return materializedFixture{}, err
	}
	git, err := newSanitizedGit(ctx, destination)
	if err != nil {
		return materializedFixture{}, err
	}
	if _, err := git.run(ctx, "init", "--quiet", "--object-format=sha1", "--initial-branch=main", "--template="+template); err != nil {
		return materializedFixture{}, err
	}
	if err := os.WriteFile(filepath.Join(destination, ".git", "config"), []byte(fixtureInitializedGitConfig), 0o666); err != nil {
		return materializedFixture{}, err
	}
	var tracked []fixtureEntry
	for _, entry := range spec.Files {
		if entry.Type == "directory" || entry.Type == "untracked-file" {
			continue
		}
		tracked = append(tracked, entry)
	}
	var indexInfo bytes.Buffer
	for _, entry := range tracked {
		object, err := git.stringInput(ctx, fixtureEntryContent(entry), "hash-object", "-w", "--no-filters", "--stdin")
		if err != nil {
			return materializedFixture{}, err
		}
		fmt.Fprintf(&indexInfo, "%s %s\t%s\x00", fixtureEntryGitMode(entry), object, entry.Path)
	}
	if len(tracked) != 0 {
		if _, err := git.runInput(ctx, indexInfo.Bytes(), "update-index", "--add", "-z", "--index-info"); err != nil {
			return materializedFixture{}, err
		}
	}
	commit, err := importFixtureCommits(ctx, git, spec)
	if err != nil {
		return materializedFixture{}, err
	}
	tree, err := git.string(ctx, "write-tree")
	if err != nil {
		return materializedFixture{}, err
	}
	if err := restoreFixtureReflogs(destination, commit); err != nil {
		return materializedFixture{}, err
	}
	return materializedFixture{Root: destination, CommitRevision: commit, TreeRevision: tree, FixtureSHA256: fixtureSHA256}, nil
}

// TestFixtureBatchedIndexMatchesPerEntryObjects pins the batched index to the
// object, mode and path each entry would receive from its own unfiltered write.
func TestFixtureBatchedIndexMatchesPerEntryObjects(t *testing.T) {
	ctx := context.Background()
	fixtures := writeTestFixture(t, "batched", `{
  "format": "atlas-cli-parity-fixture/0",
  "parentFiles": [
    {"path":"old.txt","type":"file","mode":"0644","content":"parent\r\n"}
  ],
  "files": [
    {"path":".gitattributes","type":"file","mode":"0644","content":"* text eol=crlf\n"},
    {"path":"a b.txt","type":"file","mode":"0644","content":"lf only\n"},
    {"path":"empty","type":"file","mode":"0644","content":""},
    {"path":"link","type":"symlink","mode":"0777","target":"tool"},
    {"path":"notes.md","type":"untracked-file","mode":"0644","content":"untracked\n"},
    {"path":"tool","type":"file","mode":"0755","content":"#!/bin/sh\n"}
  ]
}`)
	fixture, err := materializeFixture(ctx, fixtures, "batched", filepath.Join(t.TempDir(), "repository"))
	if err != nil {
		t.Fatal(err)
	}
	git, err := newSanitizedGit(ctx, fixture.Root)
	if err != nil {
		t.Fatal(err)
	}
	expected := func(entries ...[3]string) string {
		var lines strings.Builder
		for _, entry := range entries {
			object, err := git.stringInput(ctx, []byte(entry[2]), "hash-object", "--stdin")
			if err != nil {
				t.Fatal(err)
			}
			lines.WriteString(entry[0] + " blob " + object + "\t" + entry[1] + "\x00")
		}
		return lines.String()
	}
	for revision, want := range map[string]string{
		"HEAD^": expected([3]string{"100644", "old.txt", "parent\r\n"}),
		"HEAD": expected(
			[3]string{"100644", ".gitattributes", "* text eol=crlf\n"}, [3]string{"100644", "a b.txt", "lf only\n"},
			[3]string{"100644", "empty", ""}, [3]string{"120000", "link", "tool"}, [3]string{"100755", "tool", "#!/bin/sh\n"},
		),
	} {
		got, err := git.run(ctx, "ls-tree", "-r", "-z", revision)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("%s tree=%q, want %q", revision, got, want)
		}
	}
}

func TestFixtureLoaderRejectsUnknownFieldsAndEscapes(t *testing.T) {
	tests := map[string]string{
		"unknown": strings.Replace(testFixture, `"format":`, `"unknown":true,"format":`, 1),
		"dotdot":  strings.Replace(testFixture, `"AGENTS.md"`, `"../AGENTS.md"`, 1),
		"git":     strings.Replace(testFixture, `"AGENTS.md"`, `".git/config"`, 1),
		"escape":  strings.Replace(testFixture, `"target":"AGENTS.md"`, `"target":"../outside"`, 1),
	}
	for name, fixture := range tests {
		t.Run(name, func(t *testing.T) {
			fixtures := writeTestFixture(t, name, fixture)
			if _, _, err := loadFixture(fixtures, name); err == nil {
				t.Fatal("unsafe fixture accepted")
			}
		})
	}
}

func TestFixtureLoaderRejectsProductSource(t *testing.T) {
	for name, fixture := range map[string]string{
		"product-path":           `{"format":"atlas-cli-parity-fixture/0","files":[{"path":"src/corvint_cli.py","type":"file","mode":"0644","content":"pass\n"}]}`,
		"current-product-module": `{"format":"atlas-cli-parity-fixture/0","files":[{"path":"pkg/main.go","type":"file","mode":"0644","content":"package p; import _ \"github.com/Beamfall/corvint/internal/gokernel\"\n"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			fixtures := writeTestFixture(t, name, fixture)
			if _, _, err := loadFixture(fixtures, name); err == nil || !strings.Contains(err.Error(), "product") {
				t.Fatalf("product source error=%v", err)
			}
		})
	}
}

func TestSnapshotDetectsContentAndModeMutation(t *testing.T) {
	ctx := context.Background()
	fixtures := writeTestFixture(t, "basic", testFixture)
	fixture, err := materializeFixture(ctx, fixtures, "basic", filepath.Join(t.TempDir(), "repository"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := snapshotRepository(ctx, fixture.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyFixtureMutations(ctx, fixture.Root, []fixtureMutation{{Path: "AGENTS.md", Mode: "0755", Content: "changed\n"}}); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotRepository(ctx, fixture.Root)
	if err != nil {
		t.Fatal(err)
	}
	if before.SHA256 == after.SHA256 || before.RepositorySHA256 == after.RepositorySHA256 || before.FileModesSHA256 == after.FileModesSHA256 || before.StatusSHA256 == after.StatusSHA256 {
		t.Fatalf("mutation not detected: before=%s/%s after=%s/%s", before.SHA256, before.StatusSHA256, after.SHA256, after.StatusSHA256)
	}
	entry := findSnapshotEntry(t, after.Entries, "worktree/AGENTS.md")
	if entry.Mode != 0o755 || string(entry.Content) != "changed\n" {
		t.Fatalf("mutated entry=%#v", entry)
	}
}

func TestFixtureMaterializationHonorsCancelledContext(t *testing.T) {
	fixtures := writeTestFixture(t, "basic", testFixture)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := materializeFixture(ctx, fixtures, "basic", filepath.Join(t.TempDir(), "repository")); err == nil {
		t.Fatal("cancelled fixture materialization succeeded")
	}
}

func writeTestFixture(t *testing.T, id, contents string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fixtures")
	directory := filepath.Join(root, id)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "fixture.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func findSnapshotEntry(t *testing.T, entries []snapshotEntry, path string) snapshotEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("snapshot entry %q not found", path)
	return snapshotEntry{}
}
