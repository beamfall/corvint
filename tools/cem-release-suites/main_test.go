package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

func build(path, revision, modified string) *debug.BuildInfo {
	info := &debug.BuildInfo{Path: path, Main: debug.Module{Path: corvintModule}}
	if path == cem01Main {
		info.Main.Path = cem01Main
	}
	if revision != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs", Value: "git"}, debug.BuildSetting{Key: "vcs.revision", Value: revision})
	}
	if modified != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.modified", Value: modified})
	}
	return info
}

// TestCEMV1572ReleaseBuildsMustMatch pins the provenance refusal: the two
// release binaries must be corvint and cem01-go built from one clean commit.
func TestCEMV1572ReleaseBuildsMustMatch(t *testing.T) {
	const a, b = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	cases := []struct {
		name           string
		corvint, cem01 *debug.BuildInfo
		refusal        string
	}{
		{"same clean commit", build(corvintMain, a, "false"), build(cem01Main, a, "false"), ""},
		{"different commits", build(corvintMain, a, "false"), build(cem01Main, b, "false"), "builds differ"},
		{"modified corvint", build(corvintMain, a, "true"), build(cem01Main, a, "false"), "modified worktree"},
		{"modified unknown", build(corvintMain, a, "false"), build(cem01Main, a, ""), "modified worktree"},
		{"no vcs stamp", build(corvintMain, "", ""), build(cem01Main, a, "false"), "no Git vcs.revision"},
		{"wrong corvint main", build(corvintModule+"/cmd/corvint-tasks", a, "false"), build(cem01Main, a, "false"), "not " + corvintMain},
		{"binaries swapped", build(cem01Main, a, "false"), build(corvintMain, a, "false"), "not " + corvintMain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			revision, err := sameBuild(c.corvint, c.cem01)
			if c.refusal == "" {
				if err != nil || revision != a {
					t.Fatalf("sameBuild = %q, %v; want %s", revision, err, a)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.refusal) {
				t.Fatalf("sameBuild error %v, want %q", err, c.refusal)
			}
		})
	}
}

// TestCEMV1572GoVersionIsTheOnlyMaskedValue pins the consumer comparison: only
// runtime.goVersion is masked, so a structuralQualification difference between
// the binaries, or against a sealed line, is still a difference.
func TestCEMV1572GoVersionIsTheOnlyMaskedValue(t *testing.T) {
	sealed := []byte(`{"axes":{"a":"B"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"NOT_REQUIRED"},"repositoryEnvelope":"x","verdictSha256":"d"}`)
	otherToolchain := []byte(`{"axes":{"a":"B"},"runtime":{"goVersion":"go1.27.3","structuralQualification":"NOT_REQUIRED"},"repositoryEnvelope":"x","verdictSha256":"d"}`)
	otherQualification := []byte(`{"axes":{"a":"B"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"QUALIFIED_EXACT_TUPLE"},"repositoryEnvelope":"x","verdictSha256":"d"}`)
	if got := string(withoutGoVersion(sealed)); got != `{"axes":{"a":"B"},"runtime":{"goVersion":"","structuralQualification":"NOT_REQUIRED"},"repositoryEnvelope":"x","verdictSha256":"d"}` {
		t.Fatalf("withoutGoVersion = %s", got)
	}
	if !bytes.Equal(withoutGoVersion(sealed), withoutGoVersion(otherToolchain)) {
		t.Fatal("a goVersion difference must be masked")
	}
	if bytes.Equal(withoutGoVersion(sealed), withoutGoVersion(otherQualification)) {
		t.Fatal("a structuralQualification difference was masked")
	}
}

func TestCEMV1572RuntimeConsistent(t *testing.T) {
	cases := []struct {
		name, line, refusal string
	}{
		{"no structural hunk", `{"axes":{"structuralProof":"NOT_REQUIRED"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"NOT_REQUIRED"}}`, ""},
		{"proven tuple", `{"axes":{"structuralProof":"PROVEN_DECLARED_FILE_PREDICATES"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"QUALIFIED_EXACT_TUPLE"}}`, ""},
		{"unsupported runtime", `{"axes":{"structuralProof":"UNSUPPORTED_RUNTIME"},"runtime":{"goVersion":"go1.26.0","structuralQualification":"UNSUPPORTED"}}`, ""},
		{"qualified without a structural hunk", `{"axes":{"structuralProof":"NOT_REQUIRED"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"QUALIFIED_EXACT_TUPLE"}}`, "contradicts"},
		{"unsupported qualification on a proof", `{"axes":{"structuralProof":"PROVEN_DECLARED_FILE_PREDICATES"},"runtime":{"goVersion":"go1.26.0","structuralQualification":"UNSUPPORTED"}}`, "contradicts"},
		{"extra runtime member", `{"axes":{"structuralProof":"NOT_REQUIRED"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"NOT_REQUIRED","host":"x"}}`, "is not one"},
		{"empty goVersion", `{"axes":{"structuralProof":"NOT_REQUIRED"},"runtime":{"goVersion":"","structuralQualification":"NOT_REQUIRED"}}`, "is not one"},
		{"no runtime", `{"axes":{"structuralProof":"NOT_REQUIRED"}}`, "is not one"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var result map[string]any
			if err := json.Unmarshal([]byte(c.line), &result); err != nil {
				t.Fatal(err)
			}
			err := runtimeConsistent(result)
			if c.refusal == "" && err != nil || c.refusal != "" && (err == nil || !strings.Contains(err.Error(), c.refusal)) {
				t.Fatalf("runtimeConsistent = %v, want %q", err, c.refusal)
			}
		})
	}
}

// TestCEMV1572ProvenanceIsReportedNotProven pins the provenance line: build
// stamps are named as reported and unauthenticated, and each binary is pinned
// by the SHA-256 of its bytes, so a forged stamp cannot read as verified.
func TestCEMV1572ProvenanceIsReportedNotProven(t *testing.T) {
	dir := t.TempDir()
	corvint, cem01 := filepath.Join(dir, "corvint"), filepath.Join(dir, "cem01-go")
	if err := os.WriteFile(corvint, []byte("a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cem01, []byte("b"), 0o755); err != nil {
		t.Fatal(err)
	}
	line, err := provenanceReport("1111111111111111111111111111111111111111", corvint, cem01)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"report clean vcs.revision 1111111111111111111111111111111111111111",
		"unauthenticated build stamps, not verified build provenance",
		"corvint sha256 ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
		"cem01-go sha256 3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("provenance line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "built from") {
		t.Fatalf("provenance line %q claims how the binaries were built", line)
	}
	if _, err := provenanceReport("x", filepath.Join(dir, "missing"), cem01); err == nil {
		t.Fatal("a missing binary was not refused")
	}
}

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(withoutGitSelection(os.Environ()), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// TestCEMV1572SuitesReadCommittedObjects pins the same-commit read: inherited
// GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE cannot redirect the checkout check,
// and a worktree edit hidden by skip-worktree or assume-unchanged cannot reach
// the materialized suite tree.
func TestCEMV1572SuitesReadCommittedObjects(t *testing.T) {
	repo := t.TempDir()
	gitT(t, repo, "init", "-q")
	sealed := filepath.Join(repo, "protocol", "cem-1.0", "stable", "sealed.json")
	tool := filepath.Join(repo, "tool.sh")
	if err := os.MkdirAll(filepath.Dir(sealed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sealed, []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "seal")
	gitT(t, repo, "update-index", "--skip-worktree", "protocol/cem-1.0/stable/sealed.json")
	gitT(t, repo, "update-index", "--assume-unchanged", "tool.sh")
	if err := os.WriteFile(sealed, []byte("forged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nforged\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	decoy := t.TempDir()
	gitT(t, decoy, "init", "-q")
	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(decoy, "index"))

	head, err := gitOutput(repo, "", nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse under an inherited GIT_DIR: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "src")
	if err := materialize(repo, head, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "protocol", "cem-1.0", "stable", "sealed.json")); err != nil || string(got) != "committed\n" {
		t.Fatalf("materialized sealed file = %q, %v; want the committed bytes", got, err)
	}
	info, err := os.Stat(filepath.Join(dst, "tool.sh"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("materialized tool.sh = %v, %v; want the committed executable", info, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "tool.sh")); string(got) != "#!/bin/sh\n" {
		t.Fatalf("materialized tool.sh = %q; want the committed bytes", got)
	}
	if err := materialize(repo, "0000000000000000000000000000000000000000", filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("an absent revision was materialized")
	}
}

func TestCEMV1572MaterializeRefusesNonRegularEntries(t *testing.T) {
	repo := t.TempDir()
	gitT(t, repo, "init", "-q")
	if err := os.Symlink("target", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "link")
	head, err := gitOutput(repo, "", nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := materialize(repo, head, filepath.Join(t.TempDir(), "src")); err == nil || !strings.Contains(err.Error(), "not a regular committed file") {
		t.Fatalf("materialize symlink = %v", err)
	}
}

// TestCEMV1572EnvelopeSuitesRunInTheCommittedTree pins where the source-level
// repository-envelope tests run: inside the materialized tree, never the
// mutable checkout.
func TestCEMV1572EnvelopeSuitesRunInTheCommittedTree(t *testing.T) {
	src := t.TempDir()
	t.Setenv("GIT_DIR", "/nonexistent")
	cmds := sourceSuites(src)
	if len(cmds) != 2 {
		t.Fatalf("sourceSuites = %d commands, want 2", len(cmds))
	}
	for _, cmd := range cmds {
		rel, err := filepath.Rel(src, cmd.Dir)
		if err != nil || !filepath.IsLocal(rel) && rel != "." {
			t.Fatalf("%v runs in %s, outside %s", cmd.Args, cmd.Dir, src)
		}
		for _, kv := range cmd.Env {
			if strings.HasPrefix(kv, "GIT_") {
				t.Fatalf("%v inherits %s", cmd.Args, kv)
			}
		}
	}
	if cmds[0].Dir != src || cmds[1].Dir != filepath.Join(src, "interop", "cem01-go") {
		t.Fatalf("sourceSuites dirs = %s, %s", cmds[0].Dir, cmds[1].Dir)
	}
}

// TestCEMV1572PrivateDirIsResolved pins the case-repository location: a TMPDIR
// reached through a symbolic link, as on macOS, must not reach the verifiers.
func TestCEMV1572PrivateDirIsResolved(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "tmp-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", link)
	dir, err := privateDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
		t.Fatalf("privateDir = %s, resolves to %s (%v); want a path without symbolic links", dir, resolved, err)
	}
}

// TestCEMV1602ObservedRowsNeverPassUnrunRows pins what one passing run
// observes: every binary suite on both binaries, the source-level envelope,
// the binary-level envelope as NOT_RUN, the reported platform and no external
// row.
func TestCEMV1602ObservedRowsNeverPassUnrunRows(t *testing.T) {
	runner, platforms, external := observedRows("linux/amd64")
	if len(runner) != 11 || runner["source:repository-envelope"] != "PASSED" || runner["corvint:repository-envelope"] != "NOT_RUN" || runner["cem01-go:repository-envelope"] != "NOT_RUN" {
		t.Fatalf("runner rows = %v", runner)
	}
	for _, suite := range releaseSuites {
		if runner["corvint:"+suite] != "PASSED" || runner["cem01-go:"+suite] != "PASSED" {
			t.Fatalf("suite %s rows = %v", suite, runner)
		}
	}
	if len(platforms) != 1 || platforms["linux/amd64"] != "PASSED" || len(external) != 0 {
		t.Fatalf("platform rows %v, external rows %v", platforms, external)
	}
	if _, platforms, _ := observedRows(""); len(platforms) != 0 {
		t.Fatalf("an unreported platform is observed: %v", platforms)
	}
}

// TestCEMV1602BuildPlatform pins the platform row: both builds must report
// the same GOOS/GOARCH, or no platform is observed.
func TestCEMV1602BuildPlatform(t *testing.T) {
	stamp := func(goos, goarch string) *debug.BuildInfo {
		info := &debug.BuildInfo{}
		if goos != "" {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: "GOOS", Value: goos})
		}
		if goarch != "" {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: "GOARCH", Value: goarch})
		}
		return info
	}
	for _, c := range []struct {
		a, b *debug.BuildInfo
		want string
	}{
		{stamp("linux", "amd64"), stamp("linux", "amd64"), "linux/amd64"},
		{stamp("linux", "amd64"), stamp("darwin", "arm64"), ""},
		{stamp("linux", ""), stamp("linux", "amd64"), ""},
		{stamp("", "amd64"), stamp("", "amd64"), ""},
	} {
		if got := buildPlatform(c.a, c.b); got != c.want {
			t.Fatalf("buildPlatform = %q, want %q", got, c.want)
		}
	}
}

// TestCEMV1603JudgeReadiness pins the verdict and exit code over the sealed
// neutral-criterion ACCEPT: the committed declaration is refused with exit 3
// and every unobserved row printed, and only a run whose declared rows are all
// observed PASSED is READY.
func TestCEMV1603JudgeReadiness(t *testing.T) {
	accept, err := os.ReadFile("../../protocol/cem-1.0/stable/neutral-criterion/results/accept.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("READINESS.json")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := companionrelease.ReadStableReleaseDeclaration(raw)
	if err != nil {
		t.Fatal(err)
	}
	runner, platforms, external := observedRows("linux/amd64")
	var out bytes.Buffer
	if code := judgeReadiness(committed, [][]byte{accept}, runner, platforms, external, &out); code != 3 {
		t.Fatalf("committed declaration exit %d, want 3: %s", code, out.String())
	}
	for _, want := range []string{"readiness: runner corvint:repository-envelope NOT_RUN", "readiness: runner cem01-go:neutral-criterion PASSED", "readiness: platform darwin/arm64 NOT_OBSERVED", "readiness: platform linux/amd64 PASSED", "readiness: external independent-external-consumer NOT_OBSERVED", "readiness: REFUSED (1 accepted stable results read): release-runner-incomplete"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
	narrow := companionrelease.StableReleaseDeclaration{Profile: companionrelease.StableReleaseDeclarationProfile, Runner: []string{"corvint:neutral-criterion"}, Platform: []string{"linux/amd64"}, External: []string{"independent-external-consumer"}}
	out.Reset()
	if code := judgeReadiness(narrow, [][]byte{accept}, runner, platforms, external, &out); code != 3 || !strings.Contains(out.String(), "release-external-incomplete") {
		t.Fatalf("self-observed run exit %d, want 3 at external: %s", code, out.String())
	}
	out.Reset()
	if code := judgeReadiness(narrow, nil, runner, platforms, external, &out); code != 3 || !strings.Contains(out.String(), "stable-result-not-accepted") {
		t.Fatalf("run without an accepted result exit %d: %s", code, out.String())
	}
	out.Reset()
	if code := judgeReadiness(narrow, [][]byte{accept}, runner, platforms, map[string]string{"independent-external-consumer": "PASSED"}, &out); code != 0 || !strings.Contains(out.String(), "readiness: READY (1 accepted stable results read)") {
		t.Fatalf("fully observed run exit %d: %s", code, out.String())
	}
}

// TestCEMV1603ReadinessPathIsRepositoryRelative pins the usage refusal: the
// declaration path names a file inside the committed tree.
func TestCEMV1603ReadinessPathIsRepositoryRelative(t *testing.T) {
	for _, path := range []string{"/etc/READINESS.json", "../READINESS.json", "a/../READINESS.json", ".", "..", "a//b", "a\\b", "./a"} {
		var stderr bytes.Buffer
		if code := run([]string{"--corvint", "c", "--cem01", "p", "--readiness", path}, io.Discard, &stderr); code != 2 {
			t.Fatalf("--readiness %q exit %d, want 2: %s", path, code, stderr.String())
		}
	}
	if !repositoryRelative("tools/cem-release-suites/READINESS.json") {
		t.Fatal("the committed declaration path is refused")
	}
}
