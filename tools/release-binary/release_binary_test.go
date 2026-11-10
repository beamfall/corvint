// SPDX-License-Identifier: Apache-2.0
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild"
	"github.com/Beamfall/corvint/internal/execfile"
)

// fixtureCatalog is a release catalog over a tiny module: two core binaries, one independent
// stamped product binary, one independent unstamped (`stamp: none`) product binary and one
// support binary, built for the host target only.
const fixtureCatalog = `{
  "schema": "corvint.release-binaries/1",
  "legalFiles": ["LICENSE", "LICENSING.md"],
  "targets": [{"goos": %q, "goarch": %q, "binarySuffix": %q}],
  "versionSets": {"core": "alpha execs beta"},
  "binaries": [
    {"name": "corvint-alpha", "package": "./cmd/corvint-alpha", "role": "product", "versionSet": "core", "decision": "0594"},
    {"name": "corvint-beta", "package": "./cmd/corvint-beta", "role": "product", "versionSet": "core", "decision": "0594"},
    {"name": "corvint-gamma", "package": "./cmd/corvint-gamma", "role": "product", "decision": "0594"},
    {"name": "corvint-plain", "package": "./cmd/corvint-plain", "role": "product", "stamp": "none", "decision": "0594"},
    {"name": "corvint-tool", "package": "./cmd/corvint-tool", "role": "support", "decision": "0594"}
  ]
}
`

// fixtureMain declares the three stamp variables and prints them, so a built fixture binary
// reports what the release builder stamped.
const fixtureMain = `package main

import "fmt"

var build = "0"
var sourceCommit = ""
var version = ""

func main() { fmt.Println(version, build, sourceCommit) }
`

// fixturePlainMain declares no stamp variable: a binary whose --version another contract owns.
const fixturePlainMain = `package main

import "fmt"

func main() { fmt.Println("receipt") }
`

type fixture struct {
	root    string
	catalog string
	commit  string
}

func hostSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func git(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=test",
		"-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false",
		"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.22\n")
	write("LICENSE", "license text\n")
	write("LICENSING.md", "licensing text\n")
	for _, name := range []string{"corvint-alpha", "corvint-beta", "corvint-gamma", "corvint-tool"} {
		write("cmd/"+name+"/main.go", fixtureMain)
	}
	write("cmd/corvint-plain/main.go", fixturePlainMain)
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture")
	catalog := filepath.Join(t.TempDir(), "binaries.json")
	if err := os.WriteFile(catalog, []byte(sprintfCatalog(runtime.GOOS, runtime.GOARCH, hostSuffix())), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture{root: root, catalog: catalog, commit: git(t, root, "rev-parse", "HEAD")}
}

func sprintfCatalog(goos, goarch, suffix string) string {
	quoted := func(s string) string { raw, _ := json.Marshal(s); return string(raw) }
	return strings.Replace(strings.Replace(strings.Replace(fixtureCatalog, "%q", quoted(goos), 1), "%q", quoted(goarch), 1), "%q", quoted(suffix), 1)
}

func runCLI(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), arguments, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func requireRefusal(t *testing.T, code int, stderr, reason string) {
	t.Helper()
	if code != 2 || !strings.Contains(stderr, reason) {
		t.Fatalf("want exit 2 with %q, got exit %d: %s", reason, code, stderr)
	}
}

// ARTIFACT-BIN-V0-001: the committed catalog is valid and names the product binaries.
func TestEmbeddedCatalogNamesTheProductBinaries(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"corvint", "corvint-docs", "corvint-mcp", "corvint-docs-mcp", "corvint-test-validity-mcp", "corvint-test-runner"} {
		binary, ok := catalog.find(name)
		if !ok || binary.Role != "product" || binary.VersionSet != "core" {
			t.Errorf("%s: want product in the core set, got %+v", name, binary)
		}
	}
	for _, name := range []string{"corvint-analyzer-go", "corvint-go-test-provider", "corvint-update"} {
		binary, ok := catalog.find(name)
		if !ok || binary.Role != "product" || binary.VersionSet != "" {
			t.Errorf("%s: want an independent product binary, got %+v", name, binary)
		}
	}
	if binary, ok := catalog.find("corvint-release-candidate"); !ok || binary.Role != "support" {
		t.Errorf("corvint-release-candidate: want support, got %+v", binary)
	}
	analyzers := 0
	for _, binary := range catalog.Binaries {
		if strings.HasPrefix(binary.Name, "corvint-analyzer-") {
			analyzers++
			if binary.Role != "product" || binary.VersionSet != "" || binary.Stamp != "none" {
				t.Errorf("%s: every analyzer is an independent, unstamped (stamp none) product binary", binary.Name)
			}
		} else if binary.Stamp != "" {
			t.Errorf("%s: only analyzers are unstamped, found stamp %q", binary.Name, binary.Stamp)
		}
	}
	if analyzers == 0 {
		t.Fatal("catalog names no analyzer")
	}
}

// ARTIFACT-BIN-V0-001: a decision-less, undeclared-set or unclassified binary refuses.
func TestCatalogRefusesDecisionlessUndeclaredSetAndUnclassifiedBinary(t *testing.T) {
	f := newFixture(t)
	if code, _, stderr := runCLI(t, "check", "--root", f.root, "--binaries", f.catalog); code != 0 {
		t.Fatalf("fixture check: %s", stderr)
	}
	mutate := func(from, to, want string) {
		t.Helper()
		raw, err := os.ReadFile(f.catalog)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), from) {
			t.Fatalf("fixture catalog lacks %q", from)
		}
		path := filepath.Join(t.TempDir(), "binaries.json")
		if err := os.WriteFile(path, []byte(strings.Replace(string(raw), from, to, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := runCLI(t, "check", "--root", f.root, "--binaries", path)
		requireRefusal(t, code, stderr, want)
	}
	mutate(`"decision": "0594"}`, `"decision": ""}`, "cites no accepted decision")
	mutate(`"versionSet": "core", "decision": "0594"},
    {"name": "corvint-beta"`, `"versionSet": "edge", "decision": "0594"},
    {"name": "corvint-beta"`, "undeclared version set")
	mutate(`"role": "support"`, `"role": "support", "versionSet": "core"`, "support binary has no version set")
	mutate(`"package": "./cmd/corvint-gamma"`, `"package": "./cmd/other"`, "package must be ./cmd/corvint-gamma")
	mutate(`"stamp": "none"`, `"stamp": "receipt"`, "stamp must be omitted or none")
	// ARTIFACT-BIN-V0-005: a version set named like a binary would make NAME/vX.Y.Z ambiguous.
	raw, err := os.ReadFile(f.catalog)
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := filepath.Join(t.TempDir(), "binaries.json")
	if err := os.WriteFile(ambiguous, []byte(strings.ReplaceAll(string(raw), `"core"`, `"corvint-alpha"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, "select", "--tag", "corvint-alpha/v1.0.0", "--binaries", ambiguous)
	requireRefusal(t, code, stderr, `version set "corvint-alpha" shares a binary's name`)
	mutate(`,
    {"name": "corvint-gamma", "package": "./cmd/corvint-gamma", "role": "product", "decision": "0594"}`, ``, "cmd/corvint-gamma is not classified")
	if err := os.MkdirAll(filepath.Join(f.root, "cmd", "corvint-delta"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "check", "--root", f.root, "--binaries", f.catalog)
	requireRefusal(t, code, stderr, "cmd/corvint-delta is not classified")
}

// ARTIFACT-BIN-V0-005: a tag selects every product binary, one set or one binary.
func TestSelectTagMapsAllSetAndBinaryAndRefusesSupport(t *testing.T) {
	f := newFixture(t)
	cases := map[string]Selection{
		"v1.2.3":               {Scope: "all", Version: "1.2.3", Binaries: []string{"corvint-alpha", "corvint-beta", "corvint-gamma", "corvint-plain"}},
		"core/v1.2.3-rc.1":     {Scope: "set:core", Version: "1.2.3-rc.1", Binaries: []string{"corvint-alpha", "corvint-beta"}},
		"corvint-gamma/v1.2.3": {Scope: "binary", Version: "1.2.3", Binaries: []string{"corvint-gamma"}},
	}
	for tag, want := range cases {
		code, stdout, stderr := runCLI(t, "select", "--tag", tag, "--binaries", f.catalog)
		if code != 0 {
			t.Fatalf("%s: %s", tag, stderr)
		}
		var selection Selection
		if err := json.Unmarshal([]byte(stdout), &selection); err != nil {
			t.Fatal(err)
		}
		if selection.Tag != tag || selection.Scope != want.Scope || selection.Version != want.Version || strings.Join(selection.Binaries, ",") != strings.Join(want.Binaries, ",") {
			t.Errorf("%s: got %+v, want %+v", tag, selection, want)
		}
	}
	for tag, reason := range map[string]string{"corvint-tool/v1.0.0": "support-binary", "corvint-nope/v1.0.0": "unknown-binary", "1.0.0": "bad-tag", "core/v1": "bad-tag"} {
		code, _, stderr := runCLI(t, "select", "--tag", tag, "--binaries", f.catalog)
		requireRefusal(t, code, stderr, reason)
	}
}

func build(t *testing.T, f fixture, binary, version, output string) (int, string) {
	t.Helper()
	code, _, stderr := runCLI(t, "build", "--root", f.root, "--binaries", f.catalog, "--binary", binary, "--version", version, "--output", output, "--host-only")
	return code, stderr
}

func archiveMembers(t *testing.T, path string) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	members := map[string][]byte{}
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range reader.File {
			handle, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			members[file.Name], err = io.ReadAll(handle)
			handle.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		return members
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.ModTime.Unix() != 0 || header.Uid != 0 || header.Gid != 0 {
			t.Errorf("%s: non-canonical metadata %+v", header.Name, header)
		}
		members[header.Name], err = io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
	}
	return members
}

// ARTIFACT-BIN-V0-002, -007: one binary is built, stamped, archived with the corvint layout and
// checksummed; nothing else is compiled; an existing output refuses before any write.
func TestBuildOneBinaryWritesArchiveChecksumsAndEntryAndRefusesReplacement(t *testing.T) {
	f := newFixture(t)
	output := filepath.Join(t.TempDir(), "release")
	if code, stderr := build(t, f, "corvint-alpha", "1.0.0", output); code != 0 {
		t.Fatalf("build: %s", stderr)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	archive := archiveName("corvint-alpha", Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	want := []string{"corvint-alpha.SHA256SUMS", "corvint-alpha.release-entry.json", archive}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("output holds %v, want exactly %v", names, want)
	}
	var entry Entry
	if err := readBoundedJSON(filepath.Join(output, "corvint-alpha.release-entry.json"), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Name != "corvint-alpha" || entry.Version != "1.0.0" || entry.SourceCommit != f.commit || entry.Build != "1" || entry.VersionSet != "core" || len(entry.Archives) != 1 {
		t.Fatalf("entry %+v", entry)
	}
	root := strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip")
	members := archiveMembers(t, filepath.Join(output, archive))
	binaryName := "corvint-alpha" + hostSuffix()
	for _, member := range []string{root + "/" + binaryName, root + "/LICENSE", root + "/LICENSING.md", root + "/SHA256SUMS"} {
		if _, ok := members[member]; !ok {
			t.Errorf("archive lacks %s; members %v", member, keys(members))
		}
	}
	if len(members) != 4 {
		t.Errorf("archive has %d members, want 4: %v", len(members), keys(members))
	}
	if string(members[root+"/SHA256SUMS"]) != digestOf(members[root+"/"+binaryName])+"  "+binaryName+"\n" {
		t.Errorf("inner SHA256SUMS does not bind the binary: %q", members[root+"/SHA256SUMS"])
	}
	checksums, err := os.ReadFile(filepath.Join(output, "corvint-alpha.SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if string(checksums) != entry.Archives[0].SHA256+"  "+archive+"\n" {
		t.Errorf("outer SHA256SUMS %q does not match the entry", checksums)
	}
	// The stamped binary reports the build number and the source commit it was built from.
	extracted := filepath.Join(t.TempDir(), binaryName)
	if err := execfile.WriteFile(extracted, members[root+"/"+binaryName], 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(extracted).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "1 "+f.commit {
		t.Errorf("core-set binary printed %q, want no version stamp, build 1 and commit %s", out, f.commit)
	}
	// The embedded build info binds the commit and target: another commit or target refuses.
	host := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Suffix: hostSuffix()}
	if err := verifyBuildInfo(extracted, host, f.commit, entry.GoVersion); err != nil {
		t.Errorf("the archived binary must carry its own build info: %v", err)
	}
	other := strings.Repeat("0", 40)
	if err := verifyBuildInfo(extracted, host, other, entry.GoVersion); err == nil || !strings.Contains(err.Error(), "buildinfo-mismatch: vcs.revision="+strconv.Quote(f.commit)+" want "+strconv.Quote(other)) {
		t.Errorf("another commit: %v", err)
	}
	if err := verifyBuildInfo(extracted, Target{GOOS: "plan9", GOARCH: "386"}, f.commit, entry.GoVersion); err == nil || !strings.Contains(err.Error(), "buildinfo-mismatch: GOARCH=") || !strings.Contains(err.Error(), "GOOS=") {
		t.Errorf("another target: %v", err)
	}
	code, stderr := build(t, f, "corvint-alpha", "1.0.0", output)
	requireRefusal(t, code, stderr, "output-exists")
}

func keys(members map[string][]byte) []string {
	var names []string
	for name := range members {
		names = append(names, name)
	}
	return names
}

// ARTIFACT-BIN-V0-002: a dirty tree, a support binary, a bad version and a version the tree
// does not compile refuse before building.
func TestBuildRefusesDirtyWorktreeSupportBinaryAndBadVersion(t *testing.T) {
	f := newFixture(t)
	output := filepath.Join(t.TempDir(), "release")
	code, stderr := build(t, f, "corvint-tool", "1.0.0", output)
	requireRefusal(t, code, stderr, "support-binary")
	code, stderr = build(t, f, "corvint-alpha", "1.0", output)
	requireRefusal(t, code, stderr, "bad-version")
	if err := os.WriteFile(filepath.Join(f.root, "VERSION"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "add", "VERSION")
	git(t, f.root, "commit", "-qm", "version")
	code, stderr = build(t, f, "corvint-alpha", "1.0.0", output)
	requireRefusal(t, code, stderr, "version-mismatch: --version 1.0.0 but VERSION compiles 2.0.0")
	// An independent binary is released at its own version, which the build stamps and the
	// binary reports; VERSION is the core set's line, not its.
	independent := filepath.Join(t.TempDir(), "gamma")
	if code, stderr := build(t, f, "corvint-gamma", "1.0.0", independent); code != 0 {
		t.Fatalf("independent binary at its own version: %s", stderr)
	}
	if out := runExtracted(t, independent, "corvint-gamma"); out != "1.0.0 2 "+git(t, f.root, "rev-parse", "HEAD") {
		t.Errorf("independent binary printed %q, want its stamped version, build and commit", out)
	}
	// A `stamp: none` binary builds with no stamp; the manifest alone records its version.
	plain := filepath.Join(t.TempDir(), "plain")
	if code, stderr := build(t, f, "corvint-plain", "3.0.0", plain); code != 0 {
		t.Fatalf("unstamped binary: %s", stderr)
	}
	if out := runExtracted(t, plain, "corvint-plain"); out != "receipt" {
		t.Errorf("unstamped binary printed %q", out)
	}
	// An independent binary whose main package does not declare the stamp refuses before building.
	raw, _ := os.ReadFile(f.catalog)
	unbound := filepath.Join(t.TempDir(), "binaries.json")
	if err := os.WriteFile(unbound, []byte(strings.Replace(string(raw), `"stamp": "none", `, ``, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "build", "--root", f.root, "--binaries", unbound, "--binary", "corvint-plain", "--version", "3.0.0", "--output", filepath.Join(t.TempDir(), "unbound"), "--host-only")
	requireRefusal(t, code, stderr, "stamp-unbound: ./cmd/corvint-plain does not declare main.build, main.sourceCommit, main.version")

	if err := os.WriteFile(filepath.Join(f.root, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr = build(t, f, "corvint-alpha", "1.0.0", output)
	requireRefusal(t, code, stderr, "dirty-worktree")
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Errorf("a refused build must leave no output directory: %v", err)
	}
}

// runExtracted runs the binary inside the one archive build wrote to output and returns its output.
func runExtracted(t *testing.T, output, name string) string {
	t.Helper()
	archive := archiveName(name, Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	root := strings.TrimSuffix(strings.TrimSuffix(archive, ".tar.gz"), ".zip")
	members := archiveMembers(t, filepath.Join(output, archive))
	binaryName := name + hostSuffix()
	extracted := filepath.Join(t.TempDir(), binaryName)
	if err := execfile.WriteFile(extracted, members[root+"/"+binaryName], 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(extracted).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// ARTIFACT-BIN-V0-002: the build compiles the committed bytes alone. An ignored file in the
// working tree passes the clean check but must not reach the compiler, so the build runs from
// an exported copy of HEAD with GOWORK=off.
func TestBuildCompilesTheCommittedTreeNotIgnoredFiles(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, ".gitignore"), []byte("/cmd/corvint-gamma/ignored.go\n/go.work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "add", ".gitignore")
	git(t, f.root, "commit", "-qm", "ignore")
	if err := os.WriteFile(filepath.Join(f.root, "cmd", "corvint-gamma", "ignored.go"), []byte("package main\n\nfunc init() { panic(\"ignored file compiled in\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "go.work"), []byte("go 1.22\n\nuse ./nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := git(t, f.root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the ignored files must be invisible to the clean check, got %q", status)
	}
	output := filepath.Join(t.TempDir(), "gamma")
	if code, stderr := build(t, f, "corvint-gamma", "1.0.0", output); code != 0 {
		t.Fatalf("build from the committed tree: %s", stderr)
	}
	if out := runExtracted(t, output, "corvint-gamma"); !strings.HasPrefix(out, "1.0.0 2 ") {
		t.Errorf("built binary printed %q; the ignored file must not be compiled in", out)
	}
}

func manifest(t *testing.T, f fixture, output, base string, declarations ...string) (int, string, Manifest) {
	t.Helper()
	arguments := []string{"manifest", "--binaries", f.catalog, "--output", output}
	if base != "" {
		arguments = append(arguments, "--base", base)
	}
	for _, declaration := range declarations {
		arguments = append(arguments, "--declare-skew", declaration)
	}
	code, stdout, stderr := runCLI(t, arguments...)
	var result Manifest
	if code == 0 {
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
	}
	return code, stderr, result
}

// ARTIFACT-BIN-V0-003, -004, -007: the manifest records every binary's version and commit,
// carries earlier releases forward, refuses undeclared skew inside a version set, records a
// declared skew until the set agrees again, and verify rechecks the archives.
func TestManifestRecordsVersionsRefusesUndeclaredSkewAndCarriesDeclarations(t *testing.T) {
	f := newFixture(t)
	first := filepath.Join(t.TempDir(), "r1")
	for _, name := range []string{"corvint-alpha", "corvint-beta", "corvint-gamma"} {
		if code, stderr := build(t, f, name, "1.0.0", first); code != 0 {
			t.Fatalf("%s: %s", name, stderr)
		}
	}
	code, stderr, m1 := manifest(t, f, first, "")
	if code != 0 {
		t.Fatalf("first manifest: %s", stderr)
	}
	if len(m1.Binaries) != 3 || m1.Binaries["corvint-beta"].Version != "1.0.0" || m1.Binaries["corvint-beta"].SourceCommit != f.commit || len(m1.DeclaredSkew) != 0 {
		t.Fatalf("first manifest %+v", m1)
	}
	digest, err := os.ReadFile(filepath.Join(first, manifestName+".sha256"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(first, manifestName))
	if string(digest) != digestOf(raw)+"  "+manifestName+"\n" {
		t.Errorf("manifest digest %q does not bind the manifest", digest)
	}
	if code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(first, manifestName), "--output", first); code != 0 {
		t.Fatalf("verify first: %s", stderr)
	}
	code, _, stderr = runCLI(t, "manifest", "--binaries", f.catalog, "--output", first)
	requireRefusal(t, code, stderr, "output-exists")

	// A beta-only release skews the core set against the carried-forward alpha.
	second := filepath.Join(t.TempDir(), "r2")
	if code, stderr := build(t, f, "corvint-beta", "1.0.1", second); code != 0 {
		t.Fatalf("beta 1.0.1: %s", stderr)
	}
	code, stderr, _ = manifest(t, f, second, filepath.Join(first, manifestName))
	requireRefusal(t, code, stderr, "version-skew: set core members disagree: corvint-alpha at 1.0.0@"+f.commit+"; corvint-beta at 1.0.1@"+f.commit)
	if _, err := os.Stat(filepath.Join(second, manifestName)); !os.IsNotExist(err) {
		t.Error("a refused manifest must not be written")
	}
	code, stderr, m2 := manifest(t, f, second, filepath.Join(first, manifestName), "core=beta-only fix, alpha unchanged")
	if code != 0 {
		t.Fatalf("declared skew: %s", stderr)
	}
	if m2.Binaries["corvint-alpha"].Version != "1.0.0" || m2.Binaries["corvint-beta"].Version != "1.0.1" || m2.Binaries["corvint-gamma"].Version != "1.0.0" {
		t.Errorf("second manifest did not carry forward: %+v", m2.Binaries)
	}
	if len(m2.DeclaredSkew) != 1 || m2.DeclaredSkew[0].VersionSet != "core" || m2.DeclaredSkew[0].Reason != "beta-only fix, alpha unchanged" {
		t.Errorf("declared skew not recorded: %+v", m2.DeclaredSkew)
	}
	snapshot := map[string]string{"corvint-alpha": "1.0.0@" + f.commit, "corvint-beta": "1.0.1@" + f.commit}
	if len(m2.DeclaredSkew) == 1 && !maps.Equal(m2.DeclaredSkew[0].Members, snapshot) {
		t.Errorf("declaration must bind the members it was made for: %v, want %v", m2.DeclaredSkew[0].Members, snapshot)
	}
	// verify re-applies the rule on the stored manifest and rechecks the archives present.
	if code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(second, manifestName), "--output", second); code != 0 {
		t.Fatalf("verify second: %s", stderr)
	}

	// An independent gamma release inherits the declaration while core still disagrees.
	third := filepath.Join(t.TempDir(), "r3")
	if code, stderr := build(t, f, "corvint-gamma", "1.0.2", third); code != 0 {
		t.Fatalf("gamma 1.0.2: %s", stderr)
	}
	code, stderr, m3 := manifest(t, f, third, filepath.Join(second, manifestName))
	if code != 0 {
		t.Fatalf("gamma-only release must not re-trip a declared skew: %s", stderr)
	}
	if len(m3.DeclaredSkew) != 1 || !maps.Equal(m3.DeclaredSkew[0].Members, snapshot) {
		t.Errorf("declaration must carry forward unchanged while core still records the declared snapshot: %+v", m3.DeclaredSkew)
	}
	// A new skew in the declared set is not covered by the carried declaration: alpha moves to
	// 1.0.2 while beta stays at 1.0.1, so the release must declare its own skew.
	moved := filepath.Join(t.TempDir(), "r3-moved")
	if code, stderr := build(t, f, "corvint-alpha", "1.0.2", moved); code != 0 {
		t.Fatalf("alpha 1.0.2: %s", stderr)
	}
	code, stderr, _ = manifest(t, f, moved, filepath.Join(third, manifestName))
	requireRefusal(t, code, stderr, "version-skew: set core members disagree: corvint-alpha at 1.0.2@"+f.commit+"; corvint-beta at 1.0.1@"+f.commit)
	if _, err := os.Stat(filepath.Join(moved, manifestName)); !os.IsNotExist(err) {
		t.Error("a carried declaration must not let a new skew write a manifest")
	}
	code, stderr, mMoved := manifest(t, f, moved, filepath.Join(third, manifestName), "core=alpha moved ahead of beta")
	if code != 0 {
		t.Fatalf("re-declared skew: %s", stderr)
	}
	if len(mMoved.DeclaredSkew) != 1 || mMoved.DeclaredSkew[0].Reason != "alpha moved ahead of beta" || !maps.Equal(mMoved.DeclaredSkew[0].Members, map[string]string{"corvint-alpha": "1.0.2@" + f.commit, "corvint-beta": "1.0.1@" + f.commit}) {
		t.Errorf("re-declared skew must bind the new snapshot: %+v", mMoved.DeclaredSkew)
	}
	// verify applies the same binding: a stored declaration for another snapshot does not cover
	// the records, so the manifest refuses as skewed.
	staleRaw, _ := os.ReadFile(filepath.Join(moved, manifestName))
	stale := filepath.Join(t.TempDir(), manifestName)
	if err := os.WriteFile(stale, []byte(strings.Replace(string(staleRaw), `"corvint-alpha": "1.0.2@`, `"corvint-alpha": "1.0.0@`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "verify", "--binaries", f.catalog, "--manifest", stale)
	requireRefusal(t, code, stderr, "stale-skew-declaration: set core is declared for corvint-alpha at 1.0.0@")
	// Rebuilding alpha at beta's identity ends the skew and drops the declaration.
	fourth := filepath.Join(t.TempDir(), "r4")
	if code, stderr := build(t, f, "corvint-alpha", "1.0.1", fourth); code != 0 {
		t.Fatalf("alpha 1.0.1: %s", stderr)
	}
	code, stderr, m4 := manifest(t, f, fourth, filepath.Join(third, manifestName))
	if code != 0 {
		t.Fatalf("agreeing core: %s", stderr)
	}
	if len(m4.DeclaredSkew) != 0 || m4.Binaries["corvint-alpha"].Version != "1.0.1" || m4.Binaries["corvint-gamma"].Version != "1.0.2" {
		t.Errorf("fourth manifest %+v", m4)
	}
	// A changed archive byte is a located refusal, never a repair.
	archive := filepath.Join(fourth, m4.Binaries["corvint-alpha"].Archives[0].Name)
	tampered, _ := os.ReadFile(archive)
	tampered[len(tampered)/2] ^= 0xff
	if err := os.WriteFile(archive, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(fourth, manifestName), "--output", fourth)
	requireRefusal(t, code, stderr, "archive-mismatch: "+filepath.Base(archive)+": sha256 differs")
	// A manifest that names a binary outside the catalog, or skews without a declaration, refuses.
	hostile := filepath.Join(t.TempDir(), manifestName)
	edited := strings.Replace(string(raw), `"corvint-gamma"`, `"corvint-tool"`, 1)
	if err := os.WriteFile(hostile, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "verify", "--binaries", f.catalog, "--manifest", hostile)
	requireRefusal(t, code, stderr, `records "corvint-tool", which is not a product binary`)
}

// ARTIFACT-BIN-V0-006: the tool has no network, tag, signing or upload code. Publication is an
// owner-approved step outside it. The source is parsed: every import is the standard library
// outside net and crypto/tls or one of the two archive packages, every process the tool starts
// is `go build`, `go env` or a git subcommand on the allowlist (none of which pushes, tags or
// transfers), and no string literal names a URL or a release client.
func TestNoPublicationCode(t *testing.T) {
	allowedImports := map[string]bool{
		"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild": true,
		"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire":  true,
	}
	allowedGo := map[string]bool{"build": true, "env": true}
	allowedGit := map[string]bool{"rev-parse": true, "status": true, "rev-list": true, "clone": true, "checkout": true}
	forbiddenLiterals := []string{"http://", "https://", "git push", "git tag", "gh ", "upload", "api.github.com"}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	execCalls := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			standard := !strings.Contains(strings.SplitN(path, "/", 2)[0], ".")
			if allowedImports[path] {
				continue
			}
			if !standard || path == "net" || strings.HasPrefix(path, "net/") || path == "crypto/tls" {
				t.Errorf("%s imports %s", name, path)
			}
		}
		var enclosing string
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				enclosing = n.Name.Name
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					for _, word := range forbiddenLiterals {
						if strings.Contains(n.Value, word) {
							t.Errorf("%s: literal %s mentions %q", fileSet.Position(n.Pos()), n.Value, word)
						}
					}
				}
			case *ast.CallExpr:
				if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
					if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "exec" {
						execCalls++
						checkExecCall(t, fileSet, n, selector.Sel.Name, enclosing, allowedGo)
					}
				}
				if callee, ok := n.Fun.(*ast.Ident); ok && callee.Name == "gitOutput" {
					if len(n.Args) < 3 || !allowedGit[literal(n.Args[2])] {
						t.Errorf("%s: gitOutput runs git %q, not an allowlisted read-only or export subcommand", fileSet.Position(n.Pos()), literal(n.Args[2]))
					}
				}
			}
			return true
		})
	}
	if execCalls == 0 {
		t.Fatal("no exec call found; the scan is not reaching the source")
	}
}

// checkExecCall admits exec.CommandContext(ctx, "go", LIT, ...) with LIT in allowedGo and
// exec.CommandContext(ctx, "git", ...) only inside gitOutput, whose callers are checked by name.
func checkExecCall(t *testing.T, fileSet *token.FileSet, call *ast.CallExpr, method, enclosing string, allowedGo map[string]bool) {
	t.Helper()
	position := fileSet.Position(call.Pos())
	if method != "CommandContext" {
		t.Errorf("%s: exec.%s is not admitted; use exec.CommandContext", position, method)
		return
	}
	if len(call.Args) < 3 {
		t.Errorf("%s: exec.CommandContext needs a literal command and argument", position)
		return
	}
	switch command := literal(call.Args[1]); command {
	case "go":
		if !allowedGo[literal(call.Args[2])] {
			t.Errorf("%s: go %q is not build or env", position, literal(call.Args[2]))
		}
	case "git":
		if enclosing != "gitOutput" {
			t.Errorf("%s: git is run outside gitOutput", position)
		}
	default:
		t.Errorf("%s: command %q is not go or git", position, command)
	}
}

func literal(expression ast.Expr) string {
	basic, ok := expression.(*ast.BasicLit)
	if !ok || basic.Kind != token.STRING {
		return ""
	}
	value, _ := strconv.Unquote(basic.Value)
	return value
}

// ARTIFACT-BIN-V0-002: the Windows archive is a .zip with the same root, the .exe binary at
// 0755, the legal files and the inner SHA256SUMS row for the .exe at 0644, assembled without a
// Windows host.
func TestAssembleArchiveWindowsZip(t *testing.T) {
	target := Target{GOOS: "windows", GOARCH: "amd64", Suffix: ".exe"}
	legal := []archivebuild.File{{Name: "LICENSE", Mode: 0o644, Data: []byte("license\n")}, {Name: "LICENSING.md", Mode: 0o644, Data: []byte("licensing\n")}}
	binary := []byte("MZ fake windows binary")
	name := archiveName("corvint-alpha", target)
	if name != "corvint-alpha_windows_amd64.zip" {
		t.Fatalf("archive name %q", name)
	}
	raw, err := assembleArchive("corvint-alpha", target, "corvint-alpha.exe", binary, legal)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]os.FileMode{
		"corvint-alpha_windows_amd64/corvint-alpha.exe": 0o755,
		"corvint-alpha_windows_amd64/LICENSE":           0o644,
		"corvint-alpha_windows_amd64/LICENSING.md":      0o644,
		"corvint-alpha_windows_amd64/SHA256SUMS":        0o644,
	}
	if len(reader.File) != len(want) {
		t.Fatalf("zip has %d members, want %d", len(reader.File), len(want))
	}
	for _, file := range reader.File {
		mode, known := want[file.Name]
		if !known {
			t.Errorf("unexpected member %s", file.Name)
			continue
		}
		if file.Mode().Perm() != mode || file.Method != zip.Store {
			t.Errorf("%s: mode %o method %d, want %o stored", file.Name, file.Mode().Perm(), file.Method, mode)
		}
		if file.ModifiedDate != 0x21 || file.ModifiedTime != 0 {
			t.Errorf("%s: non-canonical timestamp date %#x time %#x", file.Name, file.ModifiedDate, file.ModifiedTime)
		}
		handle, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(handle)
		handle.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(file.Name, "/SHA256SUMS") && string(content) != digestOf(binary)+"  corvint-alpha.exe\n" {
			t.Errorf("inner SHA256SUMS %q does not bind corvint-alpha.exe", content)
		}
		if strings.HasSuffix(file.Name, ".exe") && !bytes.Equal(content, binary) {
			t.Errorf("binary member differs")
		}
	}
}

// ARTIFACT-BIN-V0-003, -004: an entry under another binary's file name, an entry for a support
// binary, a version set the catalog does not give the binary, the wrong archive count or an
// archive under another name refuses the manifest; verify refuses undeclared skew in a stored
// manifest.
func TestManifestRefusesMalformedEntriesAndVerifyRefusesUndeclaredSkew(t *testing.T) {
	f := newFixture(t)
	release := filepath.Join(t.TempDir(), "r1")
	for _, name := range []string{"corvint-alpha", "corvint-beta"} {
		if code, stderr := build(t, f, name, "1.0.0", release); code != 0 {
			t.Fatalf("%s: %s", name, stderr)
		}
	}
	raw, err := os.ReadFile(filepath.Join(release, "corvint-alpha"+entrySuffix))
	if err != nil {
		t.Fatal(err)
	}
	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	archive := entry.Archives[0].Name
	attempt := func(fileName string, mutate func(*Entry), want string) {
		t.Helper()
		directory := t.TempDir()
		for _, name := range []string{"corvint-alpha" + entrySuffix, "corvint-beta" + entrySuffix, archive, "corvint-alpha.SHA256SUMS"} {
			copyFile(t, filepath.Join(release, name), filepath.Join(directory, name))
		}
		edited := entry
		edited.Archives = append([]ArchiveEntry{}, entry.Archives...)
		mutate(&edited)
		encoded, err := canonicalJSON(edited)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, fileName), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stderr, _ := manifest(t, f, directory, "")
		requireRefusal(t, code, stderr, want)
		if _, err := os.Stat(filepath.Join(directory, manifestName)); !os.IsNotExist(err) {
			t.Errorf("%s: a refused manifest must not be written", want)
		}
	}
	attempt("corvint-gamma"+entrySuffix, func(e *Entry) {}, "entry-name-mismatch")
	attempt("corvint-tool"+entrySuffix, func(e *Entry) { e.Name = "corvint-tool"; e.VersionSet = "" }, `entry names "corvint-tool", which is not a product binary`)
	attempt("corvint-gamma"+entrySuffix, func(e *Entry) { e.Name = "corvint-gamma" }, `entry corvint-gamma names version set "core"; the catalog says ""`)
	attempt("corvint-alpha"+entrySuffix, func(e *Entry) { e.Archives = append(e.Archives, e.Archives[0]) }, "2 archives recorded; the catalog has 1 targets")
	attempt("corvint-alpha"+entrySuffix, func(e *Entry) { e.Archives[0].Name = "corvint-alpha_other.tar.gz" }, `archive row 0 is named "corvint-alpha_other.tar.gz"`)
	attempt("corvint-alpha"+entrySuffix, func(e *Entry) { e.Archives[0].GOOS = "plan9" }, "archive row 0 is plan9/")

	// verify re-applies the skew rule to a stored manifest: alpha moved without a declaration.
	if code, stderr, _ := manifest(t, f, release, ""); code != 0 {
		t.Fatalf("manifest: %s", stderr)
	}
	stored, err := os.ReadFile(filepath.Join(release, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(stored, &m); err != nil {
		t.Fatal(err)
	}
	record := m.Binaries["corvint-alpha"]
	record.Version = "1.0.5"
	m.Binaries["corvint-alpha"] = record
	skewed, err := canonicalJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), manifestName)
	if err := os.WriteFile(path, skewed, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", path)
	requireRefusal(t, code, stderr, "version-skew: set core members disagree: corvint-alpha at 1.0.5@"+f.commit+"; corvint-beta at 1.0.0@"+f.commit)
}

// ARTIFACT-BIN-V0-001: AGENTS.md names the product binaries; the catalog is the authority, so
// the two must name the same set.
func TestAgentsInvariantNamesTheCatalogProductBinaries(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "The product binaries are ")
	if start < 0 {
		t.Fatal("AGENTS.md no longer says which binaries are the product binaries")
	}
	end := strings.Index(text[start:], ";")
	if end < 0 {
		t.Fatal("the product-binaries sentence has no end")
	}
	sentence := text[start : start+end]
	named := map[string]bool{}
	for _, match := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(sentence, -1) {
		if match[1] == "corvint-analyzer-*" {
			for _, binary := range catalog.Binaries {
				if binary.Role == "product" && strings.HasPrefix(binary.Name, "corvint-analyzer-") {
					named[binary.Name] = true
				}
			}
			continue
		}
		named[match[1]] = true
	}
	products := map[string]bool{}
	for _, name := range catalog.products("") {
		products[name] = true
	}
	if !maps.Equal(named, products) {
		t.Fatalf("AGENTS.md names %v; the catalog's product binaries are %v", slices.Sorted(maps.Keys(named)), slices.Sorted(maps.Keys(products)))
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func listDirectory(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// ARTIFACT-BIN-V0-007: verify --output selects the binaries to recheck from the manifest's
// records and the archives present, not from entry files, so a publication directory holding
// archives, checksums and the manifest alone is rechecked in full; it checks the manifest digest
// file and each NAME.SHA256SUMS, and refuses a partial archive set, an unrecorded archive and a
// directory where nothing was rechecked.
func TestVerifyOutputSelectsByManifestAndArchivesPresent(t *testing.T) {
	f := newFixture(t)
	release := filepath.Join(t.TempDir(), "r1")
	for _, name := range []string{"corvint-alpha", "corvint-beta", "corvint-gamma"} {
		if code, stderr := build(t, f, name, "1.0.0", release); code != 0 {
			t.Fatalf("%s: %s", name, stderr)
		}
	}
	if code, stderr, _ := manifest(t, f, release, ""); code != 0 {
		t.Fatalf("manifest: %s", stderr)
	}
	// The publication directory: archives, NAME.SHA256SUMS, the manifest and its digest; no entries.
	published := t.TempDir()
	for _, name := range listDirectory(t, release) {
		if !strings.HasSuffix(name, entrySuffix) {
			copyFile(t, filepath.Join(release, name), filepath.Join(published, name))
		}
	}
	verify := func(directory string) (int, string, string) {
		return runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(directory, manifestName), "--output", directory)
	}
	code, stdout, stderr := verify(published)
	if code != 0 || !strings.Contains(stdout, "3 archive sets rechecked") || !strings.Contains(stdout, "corvint-alpha corvint-beta corvint-gamma") {
		t.Fatalf("published directory: exit %d %q %q", code, stdout, stderr)
	}
	archive := archiveName("corvint-beta", Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	tampered, _ := os.ReadFile(filepath.Join(published, archive))
	tampered[len(tampered)/2] ^= 0xff
	if err := os.WriteFile(filepath.Join(published, archive), tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = verify(published)
	requireRefusal(t, code, stderr, "archive-mismatch: "+archive+": sha256 differs from the manifest")
	copyFile(t, filepath.Join(release, archive), filepath.Join(published, archive))
	// NAME.SHA256SUMS must list the recorded archives.
	if err := os.WriteFile(filepath.Join(published, "corvint-beta.SHA256SUMS"), []byte(strings.Repeat("0", 64)+"  "+archive+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = verify(published)
	requireRefusal(t, code, stderr, "corvint-beta.SHA256SUMS: rows differ from the manifest")
	copyFile(t, filepath.Join(release, "corvint-beta.SHA256SUMS"), filepath.Join(published, "corvint-beta.SHA256SUMS"))
	// An archive the manifest does not record refuses.
	if err := os.WriteFile(filepath.Join(published, "corvint-delta_linux_amd64.tar.gz"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = verify(published)
	requireRefusal(t, code, stderr, "corvint-delta_linux_amd64.tar.gz: not recorded in the manifest")
	os.Remove(filepath.Join(published, "corvint-delta_linux_amd64.tar.gz"))
	// The digest file must bind the verified manifest.
	if err := os.WriteFile(filepath.Join(published, manifestName+".sha256"), []byte(strings.Repeat("0", 64)+"  "+manifestName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = verify(published)
	requireRefusal(t, code, stderr, "manifest-digest-mismatch")
	os.Remove(filepath.Join(published, manifestName+".sha256"))
	code, _, stderr = verify(published)
	requireRefusal(t, code, stderr, "manifest-digest-mismatch")
	copyFile(t, filepath.Join(release, manifestName+".sha256"), filepath.Join(published, manifestName+".sha256"))
	// A directory with the manifest alone rechecked nothing and says so.
	empty := t.TempDir()
	copyFile(t, filepath.Join(release, manifestName), filepath.Join(empty, manifestName))
	copyFile(t, filepath.Join(release, manifestName+".sha256"), filepath.Join(empty, manifestName+".sha256"))
	code, _, stderr = verify(empty)
	requireRefusal(t, code, stderr, "nothing-rechecked")
	// A binary with only some of its archives present is refused, never partly rechecked.
	raw, _ := os.ReadFile(filepath.Join(release, manifestName))
	var stored Manifest
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	partial := t.TempDir()
	first := stored.Binaries["corvint-alpha"].Archives[0]
	copyFile(t, filepath.Join(release, first.Name), filepath.Join(partial, first.Name))
	copyFile(t, filepath.Join(release, "corvint-alpha.SHA256SUMS"), filepath.Join(partial, "corvint-alpha.SHA256SUMS"))
	copyFile(t, filepath.Join(release, manifestName+".sha256"), filepath.Join(partial, manifestName+".sha256"))
	twoTargets := stored
	twoTargets.Binaries = map[string]Record{"corvint-alpha": stored.Binaries["corvint-alpha"]}
	record := twoTargets.Binaries["corvint-alpha"]
	record.Archives = append([]ArchiveEntry{first}, ArchiveEntry{GOOS: "plan9", GOARCH: "386", Name: "corvint-alpha_plan9_386.tar.gz", SHA256: first.SHA256, Bytes: first.Bytes, BinarySHA256: first.BinarySHA256})
	twoTargets.Binaries["corvint-alpha"] = record
	if _, err := verifyOutput(twoTargets, raw, partial); err == nil || !strings.Contains(err.Error(), "corvint-alpha: 1 of 2 archives present") {
		t.Fatalf("partial archive set: %v", err)
	}
}

// ARTIFACT-BIN-V0-007: output lives outside the repository and a refused or failed run leaves
// no partial file: build and manifest refuse a DIR under the root before touching it, manifest
// refuses when either of its two outputs exists before writing the other, and a finished run
// leaves no temporary sibling behind.
func TestOutputsAreWrittenOutsideTheRepositoryWithoutPartialFiles(t *testing.T) {
	f := newFixture(t)
	inside := filepath.Join(f.root, "dist")
	code, stderr := build(t, f, "corvint-alpha", "1.0.0", inside)
	requireRefusal(t, code, stderr, "output-inside-repository")
	if status := git(t, f.root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("a refused build must leave the repository clean, got %q", status)
	}
	code, _, stderr = runCLI(t, "manifest", "--binaries", f.catalog, "--root", f.root, "--output", f.root)
	requireRefusal(t, code, stderr, "output-inside-repository")

	release := filepath.Join(t.TempDir(), "r1")
	if code, stderr := build(t, f, "corvint-alpha", "1.0.0", release); code != 0 {
		t.Fatalf("build: %s", stderr)
	}
	// The digest file already exists: the manifest must not be written first and then abandoned.
	if err := os.WriteFile(filepath.Join(release, manifestName+".sha256"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr, _ = manifest(t, f, release, "")
	requireRefusal(t, code, stderr, "output-exists")
	if _, err := os.Stat(filepath.Join(release, manifestName)); !os.IsNotExist(err) {
		t.Fatalf("refused manifest left %s behind: %v", manifestName, err)
	}
	os.Remove(filepath.Join(release, manifestName+".sha256"))
	if code, stderr, _ := manifest(t, f, release, ""); code != 0 {
		t.Fatalf("manifest: %s", stderr)
	}
	archive := archiveName("corvint-alpha", Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	want := []string{"corvint-alpha.SHA256SUMS", "corvint-alpha.release-entry.json", archive, manifestName, manifestName + ".sha256"}
	if got := listDirectory(t, release); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("release directory holds %v, want exactly %v", got, want)
	}
	// A write that cannot complete removes what the run wrote: a path that appears after the
	// pre-check fails its write, and discard removes the earlier output and every temporary.
	blocked := t.TempDir()
	files := &outputs{directory: blocked}
	if err := files.reserve("first", "second"); err != nil {
		t.Fatal(err)
	}
	if err := files.write("first", []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(blocked, "second"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.write("second", []byte("2\n"), 0o644); err == nil || !strings.Contains(err.Error(), "output-exists") {
		t.Fatalf("write over an appeared path: %v", err)
	}
	files.discard()
	if got := listDirectory(t, blocked); strings.Join(got, " ") != "second" {
		t.Fatalf("discard left %v; want only the foreign directory", got)
	}
	// Another run's file that took the name after the pre-check is neither replaced nor removed.
	raced := t.TempDir()
	files = &outputs{directory: raced}
	if err := files.reserve("out"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raced, "out"), []byte("other run\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := files.write("out", []byte("this run\n"), 0o644); err == nil || !strings.Contains(err.Error(), "output-exists") {
		t.Fatalf("write over another run's file: %v", err)
	}
	files.discard()
	if raw, err := os.ReadFile(filepath.Join(raced, "out")); err != nil || string(raw) != "other run\n" {
		t.Fatalf("another run's file was replaced or removed: %q %v", raw, err)
	}
	if got := listDirectory(t, raced); strings.Join(got, " ") != "out" {
		t.Fatalf("a refused write left %v", got)
	}
}
