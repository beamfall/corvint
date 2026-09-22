package worktreeimpact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func fixture(t *testing.T) (string, *contextindex.Index, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                        "module example.test/fixture\n\ngo 1.27.0\n",
		"internal/new/existing.go":      "package newpkg\n\nfunc Existing() {}\n",
		"internal/new/existing_test.go": "package newpkg\n\nfunc TestExisting() {}\n",
		"internal/caller/caller.go":     "package caller\n\nimport _ \"example.test/fixture/internal/new\"\n",
	}
	for name, content := range files {
		writeFixtureFile(t, root, name, []byte(content))
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	added := "internal/new/added.go"
	writeFixtureFile(t, root, added, []byte("package newpkg\n\nimport \"fmt\"\n\nfunc Added() { fmt.Println(Existing()) }\n"))
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, index, added
}

func writeFixtureFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *gokernel.Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error=%v code=%q, want %q", err, typedCode(typed), code)
	}
}

func typedCode(err *gokernel.Error) string {
	if err == nil {
		return ""
	}
	return err.Code
}

func TestCompileBindsExplicitWorkingTreeAndRevisionEvidence(t *testing.T) {
	_, index, added := fixture(t)
	receipt, err := Compile(context.Background(), index, []string{added}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["profile"] != Profile || receipt["state"] != "WORKTREE_EVIDENCE" || receipt["revision"] != index.Revision {
		t.Fatalf("receipt identity=%#v", receipt)
	}
	authority := receipt["authority"].(map[string]any)
	targets := authority["targets"].(map[string]any)
	if targets["source"] != "contained-stable-twice-read-working-tree" || targets["revision_membership"] != "absent" || targets["link_count"] != 1 {
		t.Fatalf("target authority=%#v", targets)
	}
	dirtyDigest, err := digestValue(index.DirtyPaths)
	if err != nil {
		t.Fatal(err)
	}
	mixed := authority["mixed_worktree"].(map[string]any)
	if mixed["dirty_path_count"] != len(index.DirtyPaths) || mixed["dirty_paths_sha256"] != dirtyDigest ||
		mixed["status_sha256"] != index.StatusSHA256 || index.StatusSHA256 == "" {
		t.Fatalf("mixed-worktree authority=%#v", mixed)
	}
	results := receipt["results"].([]any)
	if len(results) < 4 {
		t.Fatalf("results=%#v", results)
	}
	direct := results[0].(map[string]any)
	packageContext := direct["package"].(map[string]any)
	if direct["kind"] != "working-tree-path" || direct["id"] != added ||
		packageContext["name"] != "newpkg" || packageContext["import_path"] != "example.test/fixture/internal/new" ||
		!reflect.DeepEqual(packageContext["imports"], []string{"fmt"}) {
		t.Fatalf("direct result=%#v", direct)
	}
	evidence := direct["evidence"].([]any)[0].(map[string]any)
	for _, key := range []string{"sha256", "identity_sha256", "bytes", "mode", "link_count"} {
		if evidence[key] == nil || evidence[key] == "" {
			t.Fatalf("evidence lacks %s: %#v", key, evidence)
		}
	}
	data, err := os.ReadFile(filepath.Join(index.Root, filepath.FromSlash(added)))
	if err != nil {
		t.Fatal(err)
	}
	if evidence["sha256"] != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatalf("evidence digest=%v", evidence["sha256"])
	}
	coverage := receipt["coverage"].(map[string]any)
	encoded, err := contextindex.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if coverage["packet_bytes"] != len(encoded) || len(coverage["uncertainty"].([]any)) != 3 {
		t.Fatalf("coverage=%#v bytes=%d", coverage, len(encoded))
	}
	second, err := Compile(context.Background(), index, []string{added}, 10)
	if err != nil {
		t.Fatal(err)
	}
	secondEncoded, _ := contextindex.CanonicalJSON(second)
	if !bytes.Equal(encoded, secondEncoded) {
		t.Fatalf("canonical receipt changed\nfirst=%s\nsecond=%s", encoded, secondEncoded)
	}
}

// TestWorktreeImpactFreshnessScopeIsDistinctFromDirtyCacheReuse pins DIRTY-CACHE-012: the
// working-tree-untracked authority profile binds real working-tree bytes as evidence, so its
// receipt MUST use the distinct freshness.scope=git+working-tree value, never the plain
// DIRTY-CACHE-005 dirty-cache-reuse value "git".
func TestWorktreeImpactFreshnessScopeIsDistinctFromDirtyCacheReuse(t *testing.T) {
	_, index, added := fixture(t)
	receipt, err := Compile(context.Background(), index, []string{added}, 10)
	if err != nil {
		t.Fatal(err)
	}
	freshness := receipt["freshness"].(map[string]any)
	if freshness["scope"] != "git+working-tree" {
		t.Fatalf("freshness.scope=%#v, want git+working-tree", freshness["scope"])
	}
	if freshness["scope"] == "git" {
		t.Fatal("working-tree-untracked receipt must not reuse the DIRTY-CACHE-005 scope=git value")
	}
}

func TestCompileExcludesExternalTestPackageFromSamePackageEvidence(t *testing.T) {
	root, _, added := fixture(t)
	external := "internal/new/external_test.go"
	writeFixtureFile(t, root, external, []byte("package newpkg_test\n\nfunc TestExternal() {}\n"))
	for _, arguments := range [][]string{{"add", external}, {"commit", "-qm", "external test package"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Compile(context.Background(), index, []string{added}, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range receipt["results"].([]any) {
		result := raw.(map[string]any)
		if result["id"] == external {
			t.Fatalf("external test package admitted as same-package evidence: %#v", result)
		}
	}
}

func TestCompileRejectsTrackedIgnoredAndInvalidRequests(t *testing.T) {
	root, index, added := fixture(t)
	for _, test := range []struct {
		name  string
		paths []string
		limit int
		code  string
	}{
		{"tracked", []string{"internal/new/existing.go"}, 10, "unsupported-working-tree-impact-path"},
		{"escape", []string{"../outside.go"}, 10, "invalid-working-tree-impact-path"},
		{"absolute", []string{"/outside.go"}, 10, "invalid-working-tree-impact-path"},
		{"backslash", []string{`internal\\new\\added.go`}, 10, "invalid-working-tree-impact-path"},
		{"not normalized", []string{"internal/new/./added.go"}, 10, "invalid-working-tree-impact-path"},
		{"root package", []string{"added.go"}, 10, "unsupported-working-tree-impact-path"},
		{"bad limit", []string{added}, 0, "invalid-working-tree-impact-limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(context.Background(), index, test.paths, test.limit)
			requireCode(t, err, test.code)
		})
	}
	writeFixtureFile(t, root, ".gitignore", []byte("internal/ignored/\n"))
	writeFixtureFile(t, root, "internal/ignored/value.go", []byte("package ignored\n"))
	ignoredIndex, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(context.Background(), ignoredIndex, []string{"internal/ignored/value.go"}, 10)
	requireCode(t, err, "unsupported-working-tree-impact-path")
}

func TestCompileValidatesSuffixBeforeRepositoryCondition_GPKV0029(t *testing.T) {
	for _, test := range []struct {
		name, path, code, message string
	}{
		{
			name:    "non-Go path",
			path:    "internal/new/added.py",
			code:    "unsupported-working-tree-impact-path",
			message: "untracked-path impact is implemented for `.go` files only; commit or `git add -N` the file to use tracked-path impact",
		},
		{
			name:    "Go path",
			path:    "internal/new/added.go",
			code:    "unsupported-working-tree-impact-repository",
			message: "working-tree impact requires a slash-qualified Go module",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(context.Background(), &contextindex.Index{}, []string{test.path}, 10)
			requireCode(t, err, test.code)
			if err.Error() != test.message {
				t.Fatalf("error=%q, want %q", err, test.message)
			}
		})
	}
}

func TestCompileRejectsOversizeInvalidGoAndStatusRace(t *testing.T) {
	root, _, _ := fixture(t)
	oversize := "internal/new/oversize.go"
	writeFixtureFile(t, root, oversize, append([]byte("package newpkg\n"), bytes.Repeat([]byte(" "), maxFileBytes)...))
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(context.Background(), index, []string{oversize}, 10)
	requireCode(t, err, "working-tree-impact-too-large")

	invalid := "internal/new/invalid.go"
	writeFixtureFile(t, root, invalid, []byte("package newpkg\nfunc broken(\n"))
	index, err = contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(context.Background(), index, []string{invalid}, 10)
	requireCode(t, err, "invalid-working-tree-impact-source")

	before, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "internal/new/status-race.go", []byte("package newpkg\n"))
	err = confirmIndexSnapshot(context.Background(), before, []string{"internal/new/added.go"})
	requireCode(t, err, "stale-working-tree-impact")

	revisionRoot, revisionBefore, revisionTarget := fixture(t)
	writeFixtureFile(t, revisionRoot, "internal/new/existing.go", []byte("package newpkg\n\nfunc Existing() { println(\"changed\") }\n"))
	for _, arguments := range [][]string{{"add", "internal/new/existing.go"}, {"commit", "-qm", "advance revision"}} {
		command := exec.Command("git", arguments...)
		command.Dir = revisionRoot
		if output, commandErr := command.CombinedOutput(); commandErr != nil {
			t.Fatalf("git %v: %v\n%s", arguments, commandErr, output)
		}
	}
	err = confirmIndexSnapshot(context.Background(), revisionBefore, []string{revisionTarget})
	requireCode(t, err, "stale-working-tree-impact")
}

func TestCompileRejectsAggregateAbove64MillionBytes(t *testing.T) {
	root, _, _ := fixture(t)
	prefix := []byte("package aggregate\n\n//")
	data := append(prefix, bytes.Repeat([]byte{'x'}, maxFileBytes-len(prefix))...)
	paths := make([]string, 65)
	for index := range paths {
		paths[index] = fmt.Sprintf("internal/aggregate/value_%02d.go", index)
		writeFixtureFile(t, root, paths[index], data)
	}
	repositoryIndex, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(context.Background(), repositoryIndex, paths, 10)
	requireCode(t, err, "working-tree-impact-too-large")
}

func TestConfirmIndexSnapshotRejectsSamePathStatusTransition(t *testing.T) {
	root, before, added := fixture(t)
	command := exec.Command("git", "add", added)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	after, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.DirtyPaths, after.DirtyPaths) {
		t.Fatalf("dirty paths changed: before=%v after=%v", before.DirtyPaths, after.DirtyPaths)
	}
	if before.StatusSHA256 == after.StatusSHA256 {
		t.Fatal("raw status digest did not distinguish ?? from staged A")
	}
	requireCode(
		t,
		confirmIndexSnapshot(context.Background(), before, []string{added}),
		"stale-working-tree-impact",
	)
}

func TestReadStableTargetDetectsByteRace(t *testing.T) {
	rootName, _, added := fixture(t)
	root, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, err = readStableTarget(root, added, func() {
		writeFixtureFile(t, rootName, added, []byte("package newpkg\n\nfunc Changed() {}\n"))
	})
	requireCode(t, err, "stale-working-tree-impact")
}

func TestReadStableTargetDetectsIdentityRaceWithEqualBytes(t *testing.T) {
	rootName, _, added := fixture(t)
	original, err := os.ReadFile(filepath.Join(rootName, filepath.FromSlash(added)))
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, err = readStableTarget(root, added, func() {
		replacement := filepath.Join(rootName, "internal", "new", "replacement.go")
		if writeErr := os.WriteFile(replacement, original, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
		if renameErr := os.Rename(replacement, filepath.Join(rootName, filepath.FromSlash(added))); renameErr != nil {
			t.Fatal(renameErr)
		}
	})
	requireCode(t, err, "stale-working-tree-impact")
}

func TestConfirmRepositoryRootRejectsReplacement(t *testing.T) {
	rootName, _, _ := fixture(t)
	root, err := os.OpenRoot(rootName)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	moved := rootName + "-moved"
	if err := os.Rename(rootName, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rootName, 0o755); err != nil {
		t.Fatal(err)
	}
	requireCode(t, confirmRepositoryRoot(root, rootName), "stale-working-tree-impact")
}

func TestCompileDoesNotMutateRepositoryStatus(t *testing.T) {
	root, index, added := fixture(t)
	before := gitStatus(t, root)
	if _, err := Compile(context.Background(), index, []string{added}, 10); err != nil {
		t.Fatal(err)
	}
	after := gitStatus(t, root)
	if before != after {
		t.Fatalf("status changed\nbefore=%q\nafter=%q", before, after)
	}
}

func gitStatus(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(output), "\\", "/")
}
