// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func releaseRun(t *testing.T, f fixture, arguments ...string) (int, string, string) {
	t.Helper()
	return runCLI(t, append([]string{"release", "--root", f.root, "--binaries", f.catalog}, arguments...)...)
}

// ARTIFACT-BIN-V0-523: one tag becomes a verified output directory holding the selected
// binaries and nothing else; a later tag for one binary carries the earlier records forward.
func TestReleaseTagBuildsOnlyTheSelectedBinariesAndVerifies(t *testing.T) {
	f := newFixture(t)
	output := filepath.Join(t.TempDir(), "core")
	code, stdout, stderr := releaseRun(t, f, "--tag", "core/v1.0.0", "--output", output)
	if code != 0 {
		t.Fatalf("release core/v1.0.0: exit %d: %s", code, stderr)
	}
	var run ReleaseRun
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatalf("release output %q: %v", stdout, err)
	}
	raw, err := os.ReadFile(filepath.Join(output, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if run.Schema != releaseRunSchema || run.Tag != "core/v1.0.0" || run.Scope != "set:core" || run.Version != "1.0.0" ||
		!slices.Equal(run.Binaries, []string{"corvint-alpha", "corvint-beta"}) || !slices.Equal(run.Rechecked, run.Binaries) ||
		run.ManifestSHA256 != digestOf(raw) {
		t.Fatalf("release run %+v (manifest %s)", run, digestOf(raw))
	}
	var names []string
	for _, entry := range listDirectory(t, output) {
		names = append(names, entry)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "corvint-gamma") || strings.HasPrefix(name, "corvint-plain") || strings.HasPrefix(name, "corvint-tool") {
			t.Errorf("a core/ tag built %s", name)
		}
	}
	if !slices.Contains(names, "corvint-alpha.release-entry.json") || !slices.Contains(names, "corvint-beta.SHA256SUMS") || !slices.Contains(names, manifestName+".sha256") {
		t.Fatalf("output %v lacks the set's entries, checksums or manifest digest", names)
	}
	if code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(output, manifestName), "--output", output); code != 0 {
		t.Fatalf("an independent verify of the release output refused: %s", stderr)
	}
	// ARTIFACT-BIN-V0-525: the release's one checksum file lists every archive the run wrote.
	requireReleaseChecksums(t, output, run.ChecksumsSHA256)
	if run.CoreGateReportSHA256 != "" || slices.Contains(names, coreGateReportName) {
		t.Fatalf("a tag that builds no corvint carried a gate report: %+v %v", run, names)
	}

	next := filepath.Join(t.TempDir(), "gamma")
	code, stdout, stderr = releaseRun(t, f, "--tag", "corvint-gamma/v2.1.0", "--output", next, "--base", filepath.Join(output, manifestName))
	if code != 0 {
		t.Fatalf("release corvint-gamma/v2.1.0: %s", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatal(err)
	}
	if run.Scope != "binary" || !slices.Equal(run.Binaries, []string{"corvint-gamma"}) || !slices.Equal(run.Rechecked, []string{"corvint-gamma"}) {
		t.Fatalf("one-binary release run %+v", run)
	}
	var manifest Manifest
	if err := readBoundedJSON(filepath.Join(next, manifestName), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Binaries["corvint-gamma"].Version != "2.1.0" || manifest.Binaries["corvint-alpha"].Version != "1.0.0" || len(manifest.Binaries) != 3 {
		t.Fatalf("manifest after a one-binary release: %+v", manifest.Binaries)
	}
	for _, name := range listDirectory(t, next) {
		if strings.HasPrefix(name, "corvint-alpha") || strings.HasPrefix(name, "corvint-beta") {
			t.Errorf("a one-binary tag rebuilt %s", name)
		}
	}
	requireReleaseChecksums(t, next, run.ChecksumsSHA256)
}

// requireReleaseChecksums checks DIR/SHA256SUMS: one row per archive in DIR, sorted by name,
// each the archive's digest, and the file's own digest equal to the run summary's.
func requireReleaseChecksums(t *testing.T, directory, summaryDigest string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(directory, releaseChecksumsName))
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, name := range listDirectory(t, directory) {
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}
		archive, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		want.WriteString(digestOf(archive) + "  " + name + "\n")
	}
	if want.Len() == 0 || string(raw) != want.String() {
		t.Fatalf("%s:\n%s\nwant:\n%s", releaseChecksumsName, raw, want.String())
	}
	if digestOf(raw) != summaryDigest {
		t.Fatalf("run summary checksumsSha256 %s, file %s", summaryDigest, digestOf(raw))
	}
}

// coreFixture is the release fixture with a `corvint` binary in its core set, and a gate output
// directory holding the `corvint` archive `build` produces at the fixture commit and a passing
// report that retains it.
func coreFixture(t *testing.T) (fixture, string, Entry) {
	t.Helper()
	f := newFixture(t)
	if err := os.MkdirAll(filepath.Join(f.root, "cmd", "corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "cmd", "corvint", "main.go"), []byte(fixtureMain), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "corvint")
	f.commit = git(t, f.root, "rev-parse", "HEAD")
	raw, err := os.ReadFile(f.catalog)
	if err != nil {
		t.Fatal(err)
	}
	alpha := `    {"name": "corvint-alpha",`
	catalog := strings.Replace(string(raw), alpha, `    {"name": "corvint", "package": "./cmd/corvint", "role": "product", "versionSet": "core", "decision": "0594"},`+"\n"+alpha, 1)
	if catalog == string(raw) {
		t.Fatal("fixture catalog lacks the corvint-alpha row")
	}
	if err := os.WriteFile(f.catalog, []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(t.TempDir(), "core")
	if code, _, stderr := runCLI(t, "build", "--root", f.root, "--binaries", f.catalog, "--binary", "corvint", "--version", "1.0.0", "--output", gate); code != 0 {
		t.Fatalf("build corvint for the gate fixture: %s", stderr)
	}
	var entry Entry
	if err := readBoundedJSON(filepath.Join(gate, "corvint"+entrySuffix), &entry); err != nil {
		t.Fatal(err)
	}
	writeGateReport(t, gate, gateReport(f.commit, entry))
	return f, gate, entry
}

func gateReport(commit string, entry Entry) map[string]any {
	var targets []any
	for _, archive := range entry.Archives {
		targets = append(targets, map[string]any{
			"goos": archive.GOOS, "goarch": archive.GOARCH, "binaryName": "corvint" + hostSuffix(), "archiveName": archive.Name,
			"retainedBinary":  map[string]any{"sha256": archive.BinarySHA256, "bytes": 1},
			"retainedArchive": map[string]any{"sha256": archive.SHA256, "bytes": archive.Bytes},
		})
	}
	return map[string]any{"schema": coreGateReportSchema, "revision": commit, "tree": strings.Repeat("0", 40), "toolchain": entry.GoVersion, "verdict": "PASS", "targets": targets, "reasons": []any{}}
}

func writeGateReport(t *testing.T, gate string, report map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gate, coreGateReportName), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ARTIFACT-BIN-V0-525, -526: a tag that builds corvint needs the archive gate's output; the
// release then carries the gate's report unchanged, binds it to corvint archives byte-identical
// to the gate's, and lists every archive in one SHA256SUMS. Each disagreement refuses and leaves
// the output as found.
func TestReleaseCoreTagBindsTheGateReportAndWritesTheChecksums(t *testing.T) {
	f, gate, entry := coreFixture(t)

	missing := filepath.Join(t.TempDir(), "missing")
	code, _, stderr := releaseRun(t, f, "--tag", "core/v1.0.0", "--output", missing)
	requireRefusal(t, code, stderr, "core-gate-required")
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("a refused run created its output: %v", err)
	}
	code, _, stderr = releaseRun(t, f, "--tag", "corvint-gamma/v1.0.0", "--output", filepath.Join(t.TempDir(), "gamma"), "--core-gate", gate)
	requireRefusal(t, code, stderr, "core-gate-unused")

	refusals := []struct {
		name, want string
		edit       func(map[string]any)
	}{
		{"revision", "revision", func(r map[string]any) { r["revision"] = strings.Repeat("a", 40) }},
		{"verdict", "verdict", func(r map[string]any) { r["verdict"] = "FAIL" }},
		{"reasons", "verdict", func(r map[string]any) { r["reasons"] = []any{map[string]any{"code": "x"}} }},
		{"schema", "schema", func(r map[string]any) { r["schema"] = "corvint.release-go-archive-report.v1" }},
		{"toolchain", "toolchain", func(r map[string]any) { r["toolchain"] = "go1.0" }},
		{"archive digest", "retainedArchive", func(r map[string]any) {
			r["targets"].([]any)[0].(map[string]any)["retainedArchive"] = map[string]any{"sha256": strings.Repeat("b", 64), "bytes": entry.Archives[0].Bytes}
		}},
		{"binary digest", "retainedBinary", func(r map[string]any) {
			r["targets"].([]any)[0].(map[string]any)["retainedBinary"] = map[string]any{"sha256": strings.Repeat("c", 64)}
		}},
		{"no target", "0 report targets", func(r map[string]any) { r["targets"] = []any{} }},
		{"two targets", "2 report targets", func(r map[string]any) {
			r["targets"] = append(r["targets"].([]any), r["targets"].([]any)[0])
		}},
	}
	for _, refusal := range refusals {
		report := gateReport(f.commit, entry)
		refusal.edit(report)
		writeGateReport(t, gate, report)
		empty := t.TempDir()
		code, _, stderr := releaseRun(t, f, "--tag", "core/v1.0.0", "--output", empty, "--core-gate", gate)
		requireRefusal(t, code, stderr, "core-gate-mismatch")
		if !strings.Contains(stderr, refusal.want) {
			t.Errorf("%s: refusal does not name %q: %s", refusal.name, refusal.want, stderr)
		}
		if got := listDirectory(t, empty); len(got) != 0 {
			t.Errorf("%s: a refused run left %v", refusal.name, got)
		}
	}

	// The gate's own retained archive differs from the report row and from the build.
	writeGateReport(t, gate, gateReport(f.commit, entry))
	archivePath := filepath.Join(gate, entry.Archives[0].Name)
	original, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := slices.Clone(original)
	tampered[len(tampered)-1] ^= 0xff
	if err := os.WriteFile(archivePath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = releaseRun(t, f, "--tag", "core/v1.0.0", "--output", filepath.Join(t.TempDir(), "out"), "--core-gate", gate)
	requireRefusal(t, code, stderr, "gate archive sha256 differs")
	if err := os.WriteFile(archivePath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "core")
	code, stdout, stderr := releaseRun(t, f, "--tag", "core/v1.0.0", "--output", output, "--core-gate", gate)
	if code != 0 {
		t.Fatalf("release core/v1.0.0 --core-gate: exit %d: %s", code, stderr)
	}
	var run ReleaseRun
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(run.Binaries, []string{"corvint", "corvint-alpha", "corvint-beta"}) {
		t.Fatalf("release run binaries %v", run.Binaries)
	}
	copied, err := os.ReadFile(filepath.Join(output, coreGateReportName))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(gate, coreGateReportName))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(copied, source) || run.CoreGateReportSHA256 != digestOf(source) {
		t.Fatalf("the release's %s is not the gate's report unchanged (summary %s)", coreGateReportName, run.CoreGateReportSHA256)
	}
	built, err := os.ReadFile(filepath.Join(output, entry.Archives[0].Name))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(built, original) {
		t.Fatalf("%s differs from the gate's retained archive", entry.Archives[0].Name)
	}
	requireReleaseChecksums(t, output, run.ChecksumsSHA256)
	sums, _ := os.ReadFile(filepath.Join(output, releaseChecksumsName))
	for _, name := range []string{"corvint", "corvint-alpha", "corvint-beta"} {
		if !strings.Contains(string(sums), "  "+name+"_"+runtime.GOOS+"_"+runtime.GOARCH) {
			t.Errorf("%s lacks the %s archive row:\n%s", releaseChecksumsName, name, sums)
		}
	}
	if code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(output, manifestName), "--output", output); code != 0 {
		t.Fatalf("an independent verify of the gate-bound release output refused: %s", stderr)
	}
}

// ARTIFACT-BIN-V0-524: a non-empty output directory refuses before anything is built, and a
// failure after the first build leaves the directory as the run found it.
func TestReleaseTagRefusesNonEmptyOutputAndRollsBackOnFailure(t *testing.T) {
	f := newFixture(t)
	occupied := t.TempDir()
	if err := os.WriteFile(filepath.Join(occupied, "corvint-old.release-entry.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := releaseRun(t, f, "--tag", "corvint-gamma/v1.0.0", "--output", occupied)
	requireRefusal(t, code, stderr, "output-not-empty")
	if got := listDirectory(t, occupied); !slices.Equal(got, []string{"corvint-old.release-entry.json"}) {
		t.Fatalf("a refused run changed the output: %v", got)
	}
	for tag, reason := range map[string]string{"corvint-tool/v1.0.0": "support-binary", "v1": "bad-tag"} {
		code, _, stderr := releaseRun(t, f, "--tag", tag, "--output", filepath.Join(t.TempDir(), "out"))
		requireRefusal(t, code, stderr, reason)
	}
	code, _, stderr = releaseRun(t, f, "--tag", "v1.0.0", "--output", filepath.Join(t.TempDir(), "out"), "--host-only")
	requireRefusal(t, code, stderr, "--host-only")

	// corvint-plain joins the core set without `stamp: none`, so it refuses stamp-unbound after
	// corvint-alpha and corvint-beta are already written; every file of the run is removed.
	raw, err := os.ReadFile(f.catalog)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), `"role": "product", "stamp": "none"`, `"role": "product", "versionSet": "core"`, 1)
	if broken == string(raw) {
		t.Fatal("fixture catalog lacks the corvint-plain row")
	}
	if err := os.WriteFile(f.catalog, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(t.TempDir(), "created")
	code, _, stderr = releaseRun(t, f, "--tag", "core/v1.0.0", "--output", created)
	requireRefusal(t, code, stderr, "stamp-unbound")
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("a failed run left the directory it created: %v %v", err, listDirectoryIfPresent(created))
	}
	empty := t.TempDir()
	code, _, stderr = releaseRun(t, f, "--tag", "core/v1.0.0", "--output", empty)
	requireRefusal(t, code, stderr, "stamp-unbound")
	if got := listDirectory(t, empty); len(got) != 0 {
		t.Fatalf("a failed run left files in an existing empty directory: %v", got)
	}
}

func listDirectoryIfPresent(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
