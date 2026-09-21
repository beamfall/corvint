package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testGit(t testing.TB, root string, arguments ...string) string {
	t.Helper()
	// Fixture repositories can cross Git's auto-maintenance thresholds when a
	// package runs many tests. Keep any maintenance synchronous and disable
	// auto-gc so no detached writer outlives t.TempDir cleanup.
	prefix := []string{"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-C", root}
	command := exec.Command("git", append(prefix, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func testRepository(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/fixture\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\nfunc MintToken() string { return \"old\" }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "initial")
	return root
}

func writeTestFile(t testing.TB, root, relative, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkBuildColdLargeCorpus(b *testing.B) {
	root := testRepository(b)
	for index := 0; index < 64; index++ {
		path := fmt.Sprintf("internal/corpus/file%03d.go", index)
		content := "package corpus\n\nvar Payload" + strconv.Itoa(index) + " = \"" + strings.Repeat("x", 32*1024) + "\"\n"
		writeTestFile(b, root, path, content)
	}
	testGit(b, root, "add", ".")
	testGit(b, root, "commit", "-qm", "large corpus")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		built, err := Build(context.Background(), root)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(built)
	}
}

func TestBuildRejectsIdentityDriftAcrossAllAttempts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	root := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	format := testGit(t, root, "rev-parse", "--show-object-format")
	commit := testGit(t, root, "rev-parse", "HEAD")
	tree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	alternate := "0" + commit[1:]
	if commit[0] == '0' {
		alternate = "1" + commit[1:]
	}
	bin := t.TempDir()
	counter := filepath.Join(t.TempDir(), "identity-count")
	wrapper := fmt.Sprintf(`#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then
    count=0
    if [ -f %s ]; then count=$(sed -n '1p' %s); fi
    count=$((count + 1))
    printf '%%s\n' "$count" > %s
    commit=%s
    if [ $((count %% 2)) -eq 0 ]; then commit=%s; fi
    printf '%%s\nfalse\n%%s\n%%s\n.git/info/grafts\n' %s "$commit" %s
    exit 0
  fi
done
exec %s "$@"
`, strconv.Quote(counter), strconv.Quote(counter), strconv.Quote(counter),
		strconv.Quote(commit), strconv.Quote(alternate), strconv.Quote(format),
		strconv.Quote(tree), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = Build(context.Background(), root)
	if err == nil || err.Error() != "repository HEAD changed while building context index" {
		t.Fatalf("Build error = %v", err)
	}
	rawCount, readErr := os.ReadFile(counter)
	if readErr != nil || strings.TrimSpace(string(rawCount)) != "6" {
		t.Fatalf("identity probes = %q, err=%v; want 6", rawCount, readErr)
	}
}

func TestBuildExcludesGitLFSPointerAndCarriesItThroughSnapshot(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod": "module example.test/lfs\n\ngo 1.27.0\n",
		"internal/model.go": "version https://git-lfs.github.com/spec/v1\n" +
			"oid sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n" +
			"size 12345\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := Exclusion{Path: "internal/model.go", Reason: "git-lfs pointer, content not in the tree"}
	if _, ok := index.Sources[want.Path]; ok {
		t.Fatalf("IDX-SNAP-V0-013: %s was indexed as pointer text", want.Path)
	}
	if !exclusionIn(index.Exclusions, want) {
		t.Fatalf("IDX-SNAP-V0-013: exclusions = %#v, missing %#v", index.Exclusions, want)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("IDX-SNAP-V0-013: snapshot hit = %v, err = %v", hit, err)
	}
	if !exclusionIn(loaded.Exclusions, want) {
		t.Fatalf("IDX-SNAP-V0-013: loaded exclusions = %#v, missing %#v", loaded.Exclusions, want)
	}
	built, err := receipt(loaded, "query", map[string]any{}, nil, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	samples := mapsFromAny(built["exclusions"].(map[string]any)["samples"])
	if len(samples) != 1 || samples[0]["path"] != want.Path || samples[0]["reason"] != want.Reason {
		t.Fatalf("IDX-SNAP-V0-013: receipt exclusion samples = %#v", samples)
	}
}

func TestReadCleanFileRejectsSymlinkComponents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink and FIFO contract is POSIX-only")
	}
	root := t.TempDir()
	writeTestFile(t, root, "real/value.go", "package real\n")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if data, ok := readCleanFile(root, "linked/value.go"); ok || data != nil {
		t.Fatalf("symlinked parent admitted: ok=%v data=%q", ok, data)
	}
	if err := os.Symlink(filepath.Join(root, "real", "value.go"), filepath.Join(root, "leaf.go")); err != nil {
		t.Fatal(err)
	}
	if data, ok := readCleanFile(root, "leaf.go"); ok || data != nil {
		t.Fatalf("symlinked leaf admitted: ok=%v data=%q", ok, data)
	}
	if generated, ok := verifyCleanFile(root, "linked/value.go", "", "sha1"); ok || generated {
		t.Fatalf("streaming verifier admitted symlinked parent: ok=%v generated=%v", ok, generated)
	}
	if generated, ok := verifyCleanFile(root, "leaf.go", "", "sha1"); ok || generated {
		t.Fatalf("streaming verifier admitted symlinked leaf: ok=%v generated=%v", ok, generated)
	}
}

func TestVerifyCleanFileStreamsGitBlobAndGeneratedHeader(t *testing.T) {
	root := t.TempDir()
	content := []byte("// Code generated by fixture DO NOT EDIT.\npackage fixture\n\nvar Payload = \"" + strings.Repeat("x", 8192) + "\"\n")
	writeTestFile(t, root, "generated.go", string(content))
	for _, format := range []string{"sha1", "sha256"} {
		generated, ok := verifyCleanFile(root, "generated.go", gitBlobHash(content, format), format)
		if !ok || !generated {
			t.Fatalf("verifyCleanFile(%s) = generated:%v ok:%v, want true true", format, generated, ok)
		}
	}
	if generated, ok := verifyCleanFile(root, "generated.go", strings.Repeat("0", 40), "sha1"); ok || generated {
		t.Fatalf("mismatched blob admitted: generated=%v ok=%v", generated, ok)
	}
}

func TestVerifyCleanFileReusesScratchWithoutHeaderStateLeak(t *testing.T) {
	root := t.TempDir()
	generated := []byte("// Code generated by fixture DO NOT EDIT.\npackage fixture\n")
	plain := []byte("package fixture\n")
	writeTestFile(t, root, "generated.go", string(generated))
	writeTestFile(t, root, "plain.go", string(plain))
	var scratch cleanFileVerificationScratch
	if got, ok := verifyCleanFileWithScratch(root, "generated.go", gitBlobHash(generated, "sha1"), "sha1", &scratch); !ok || !got {
		t.Fatalf("generated verification = generated:%v ok:%v, want true true", got, ok)
	}
	if got, ok := verifyCleanFileWithScratch(root, "plain.go", gitBlobHash(plain, "sha1"), "sha1", &scratch); !ok || got {
		t.Fatalf("plain verification after generated = generated:%v ok:%v, want false true", got, ok)
	}
}

func TestBuildAttemptFallsBackToImmutableBlobAfterRestoredMtimeEdit(t *testing.T) {
	root := testRepository(t)
	identity, err := readIdentity(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "internal", "token", "token.go")
	metadata, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	original := "package token\n\nfunc MintToken() string { return \"old\" }\n"
	changed := "package token\n\nfunc MintToken() string { return \"new\" }\n"
	if len(original) != len(changed) {
		t.Fatal("fixture edit must preserve size")
	}
	clean, err := buildAttempt(context.Background(), root, identity, nil, "", (*Index).compile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(clean.Sources["internal/token/token.go"].Data); got != original {
		t.Fatalf("clean source = %q, want committed bytes", got)
	}
	if stringIn(clean.DirtyPaths, "internal/token/token.go") {
		t.Fatalf("clean paths = %v", clean.DirtyPaths)
	}
	if err := os.WriteFile(file, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, metadata.ModTime(), metadata.ModTime()); err != nil {
		t.Fatal(err)
	}

	index, err := buildAttempt(context.Background(), root, identity, nil, "", (*Index).compile)
	if err != nil {
		t.Fatal(err)
	}
	source := index.Sources["internal/token/token.go"]
	if string(source.Data) != original {
		t.Fatalf("indexed mutable bytes: %q", source.Data)
	}
	if len(index.DirtyPaths) != 0 {
		t.Fatalf("status-clean path changed freshness: %v", index.DirtyPaths)
	}
}

func TestBuildHonorsPrecancelledContext(t *testing.T) {
	root := testRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := Build(ctx, root)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("Build error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelled Build took %s", elapsed)
	}
}

func TestBuildTreatsCleanEmptyAndOversizedSourcesAsFresh(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, "internal/token/empty.go", "")
	writeTestFile(t, root, "docs/oversized.md", strings.Repeat("x", maxSourceBytes+1))
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "add source boundary fixtures")

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.DirtyPaths) != 0 {
		t.Fatalf("dirty paths = %v, want none", index.DirtyPaths)
	}
	empty, ok := index.Sources["internal/token/empty.go"]
	if !ok || len(empty.Data) != 0 {
		t.Fatalf("clean empty source = %#v, admitted=%v", empty, ok)
	}
	if _, ok := index.Sources["docs/oversized.md"]; ok {
		t.Fatal("oversized source was admitted")
	}
	want := Exclusion{Path: "docs/oversized.md", Reason: "source exceeds size bound"}
	if !exclusionIn(index.Exclusions, want) {
		t.Fatalf("exclusions = %#v, missing %#v", index.Exclusions, want)
	}
}

func TestBuildPinsTrackedBytesAndOmitsUntrackedContent(t *testing.T) {
	root := testRepository(t)
	trackedPath := "internal/token/token.go"
	trackedCanary := "MutableTrackedCanary"
	untrackedPath := "internal/token/private_secret.go"
	untrackedCanary := "CorvintNeverIndexThisMutableCanary"
	writeTestFile(t, root, trackedPath, "package token\n\nfunc "+trackedCanary+"() {}\n")
	writeTestFile(t, root, untrackedPath, "package token\n\nfunc "+untrackedCanary+"() {}\n")

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{trackedPath, untrackedPath} {
		if !stringIn(index.DirtyPaths, changed) {
			t.Errorf("dirty paths = %v, missing %s", index.DirtyPaths, changed)
		}
	}
	tracked := index.Sources[trackedPath]
	if !strings.Contains(string(tracked.Data), "MintToken") || strings.Contains(string(tracked.Data), trackedCanary) {
		t.Fatalf("tracked source was not pinned to Git bytes: %q", tracked.Data)
	}
	if _, ok := index.Sources[untrackedPath]; ok {
		t.Fatal("untracked source was admitted")
	}
	for path, source := range index.Sources {
		if strings.Contains(string(source.Data), untrackedCanary) {
			t.Fatalf("untracked canary entered source %s", path)
		}
	}
}

func TestBuildReportsModeOnlyAndRenameEndpoints(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode and rename fixture is POSIX-only")
	}
	t.Run("mode-only", func(t *testing.T) {
		root := testRepository(t)
		testGit(t, root, "config", "core.filemode", "true")
		path := filepath.Join(root, "internal", "token", "token.go")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !stringIn(index.DirtyPaths, "internal/token/token.go") {
			t.Fatalf("dirty paths = %v", index.DirtyPaths)
		}
	})
	t.Run("rename", func(t *testing.T) {
		root := testRepository(t)
		testGit(t, root, "mv", "internal/token/token.go", "internal/token/renamed token.go")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		for _, endpoint := range []string{"internal/token/token.go", "internal/token/renamed token.go"} {
			if !stringIn(index.DirtyPaths, endpoint) {
				t.Errorf("dirty paths = %v, missing %s", index.DirtyPaths, endpoint)
			}
		}
		if _, ok := index.Sources["internal/token/token.go"]; !ok {
			t.Fatal("committed rename source was not retained")
		}
		if _, ok := index.Sources["internal/token/renamed token.go"]; ok {
			t.Fatal("worktree-only rename destination was admitted")
		}
	})
}

func TestBuildDoesNotObserveIgnoredWorktreeContent(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, ".gitignore", "ignored_secret.go\n")
	testGit(t, root, "add", ".gitignore")
	testGit(t, root, "commit", "-qm", "ignore local secret")
	canary := "CorvintIgnoredContentMustRemainUnobserved"
	writeTestFile(t, root, "ignored_secret.go", "package secret\n\nfunc "+canary+"() {}\n")

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.DirtyPaths) != 0 {
		t.Fatalf("ignored path changed freshness: %v", index.DirtyPaths)
	}
	if _, ok := index.Sources["ignored_secret.go"]; ok {
		t.Fatal("ignored source was admitted")
	}
	for path, source := range index.Sources {
		if strings.Contains(string(source.Data), canary) {
			t.Fatalf("ignored canary entered source %s", path)
		}
	}
}

func TestBuildFallsBackToGitWhenTrackedFileBecomesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is POSIX-only")
	}
	root := testRepository(t)
	external := filepath.Join(t.TempDir(), "external-secret.go")
	canary := "CorvintExternalSymlinkCanary"
	if err := os.WriteFile(external, []byte("package secret\n\nfunc "+canary+"() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "internal", "token", "token.go")
	if err := os.Remove(tracked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, tracked); err != nil {
		t.Fatal(err)
	}

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !stringIn(index.DirtyPaths, "internal/token/token.go") {
		t.Fatalf("dirty paths = %v", index.DirtyPaths)
	}
	source := index.Sources["internal/token/token.go"]
	if !strings.Contains(string(source.Data), "MintToken") || strings.Contains(string(source.Data), canary) {
		t.Fatalf("symlink target entered index: %q", source.Data)
	}
}

// TestBuildCarriesWebSymbolsIntoTheIndexAndMatchesBuildEval pins the fix for
// "contextindex: web symbols exist only on the query eval path": a .ts
// declaration used to reach Symbols only through EvalQuery's private copy of
// the index, so Build, BuildEval and every other consumer never saw it. The
// extractor now lives in the build itself (symbolExtractors in
// langsymbols.go), so Build's own Symbols must carry it, and BuildEval --
// which shares the same compileSources walk -- must see exactly the same set.
func TestBuildCarriesWebSymbolsIntoTheIndexAndMatchesBuildEval(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, "web/session.ts", "export function ignoredSessionRevocation() { return true }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "add a TypeScript declaration")

	full, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, symbol := range full.Symbols {
		if symbol.Path == "web/session.ts" && symbol.Name == "ignoredSessionRevocation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Build did not carry the .ts declaration into Symbols: %v", full.Symbols)
	}

	eval, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full.Symbols, eval.Symbols) {
		t.Fatalf("BuildEval.Symbols differs from Build.Symbols:\nBuild:     %v\nBuildEval: %v", full.Symbols, eval.Symbols)
	}
}

func exclusionIn(exclusions []Exclusion, want Exclusion) bool {
	for _, exclusion := range exclusions {
		if exclusion == want {
			return true
		}
	}
	return false
}

// TestForbiddenPathScreenIsTheAcceptedSet pins IDX-SNAP-V0-018: the component
// set, the protected prefixes, the generated-path pattern, their reasons, and
// the first-match order. A change here without the clause is a spec drift.
func TestForbiddenPathScreenIsTheAcceptedSet(t *testing.T) {
	components := []string{".git", "vendor", "node_modules", "app-dist", "dist", "build", "coverage", ".next", ".cache", "target", ".claude"}
	accepted := make(map[string]bool, len(components))
	for _, component := range components {
		accepted[component] = true
	}
	if !reflect.DeepEqual(forbiddenParts, accepted) {
		t.Fatalf("forbiddenParts = %v, IDX-SNAP-V0-018 names %v", forbiddenParts, components)
	}
	const pattern = `(?i)(?:^|/)(?:generated|gen)(?:/|$)|(?:\.gen\.|_generated\.)|(?:^|/)docs/api/(?:openapi\.json|reference\.md|llms(?:-full)?\.txt)$`
	if generatedPath.String() != pattern {
		t.Fatalf("generatedPath = %q, IDX-SNAP-V0-018 names %q", generatedPath.String(), pattern)
	}
	cases := map[string]string{
		"pkg/target/main.go":                     "vendor/build excluded",
		"internal/store/migrate/vendor/0001.sql": "vendor/build excluded",
		"internal/store/migrate/0001.sql":        "protected path",
		"internal/conformance/testdata/gen/a.go": "protected path",
		"api/gen/client.go":                      "generated path",
		"docs/api/openapi.json":                  "generated path",
		"internal/claude/token.go":               "",
		"builds/main.go":                         "",
		"Vendor/lib.go":                          "",
	}
	for _, component := range components {
		cases[component+"/file.go"] = "vendor/build excluded"
	}
	for value, want := range cases {
		if got := forbiddenPath(value); got != want {
			t.Errorf("forbiddenPath(%q) = %q, want %q", value, got, want)
		}
	}
}

// TestReceiptCountsUnsupportedSuffixExclusions pins GPK-V0-063: a tracked path
// no index suffix admits (the fixture's `.gitignore`) is never read, so the
// receipt's `exclusions.count` includes it on the eager, selective-query, and
// event-snapshot paths alike. It has no Exclusion row, so it adds no sample.
func TestReceiptCountsUnsupportedSuffixExclusions(t *testing.T) {
	root := authorityRepository(t)
	eager, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(eager); err != nil {
		t.Fatal(err)
	}
	event, hit, err := LoadEventSnapshot(context.Background(), root, false)
	if err != nil || !hit {
		t.Fatalf("event load: hit=%v err=%v", hit, err)
	}
	for name, index := range map[string]*Index{"eager": eager, "selective": selective, "event": event} {
		built, err := QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
		if err != nil {
			t.Fatal(err)
		}
		exclusions := built["exclusions"].(map[string]any)
		if exclusions["count"] != 1 || len(mapsFromAny(exclusions["samples"])) != 0 {
			t.Fatalf("GPK-V0-063 %s: exclusions = %#v, want count 1 and no samples", name, exclusions)
		}
	}
}
