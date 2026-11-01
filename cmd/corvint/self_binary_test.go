package main

import (
	"bytes"
	"context"
	"io/fs"
	"runtime/debug"
	"strings"
	"testing"
)

// selfBinaryRepository is a one-module repository whose first commit a test
// binary claims as its build commit; it returns that commit.
func selfBinaryRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeAffectedFiles(t, root, map[string]string{
		"go.mod":       "module example.test/self\n\ngo 1.22\n",
		"self.go":      "package self\n\nfunc Self() int { return 1 }\n",
		"self_test.go": "package self\n\nimport \"testing\"\n\nfunc TestSelf(t *testing.T) { _ = Self() }\n",
	})
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "built"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root, strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
}

func selfBinaryInfo(module, commit string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Path: module}, Settings: []debug.BuildSetting{
		{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: commit}, {Key: "vcs.modified", Value: "false"},
	}}
}

func commitSelfBinaryFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	writeAffectedFiles(t, root, files)
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "change")
}

// stubSelfBuildInfo makes this test binary claim info as its own build.
func stubSelfBuildInfo(t *testing.T, info *debug.BuildInfo) {
	saved := selfBuildInfo
	selfBuildInfo = func() (*debug.BuildInfo, bool) { return info, true }
	t.Cleanup(func() { selfBuildInfo = saved })
}

// AFP-V0-846: only a Go source or module change between the stamped commit
// and HEAD, in the binary's own module, makes the binary stale; a commit that
// cannot be compared is unverified, not stale.
func TestSelfBinaryStaleComparesStampedCommitWithHEAD(t *testing.T) {
	ctx := context.Background()
	root, built := selfBinaryRepository(t)
	own := selfBinaryInfo("example.test/self", built)
	if freshness := binaryStaleAgainst(ctx, root, own); freshness.Code != "" {
		t.Fatalf("a binary built at HEAD is judged %+v", freshness)
	}
	commitSelfBinaryFiles(t, root, map[string]string{"self_test.go": "package self\n", "README.md": "docs\n", "testdata/x.go": "package x\n"})
	if freshness := binaryStaleAgainst(ctx, root, own); freshness.Code != "" {
		t.Fatalf("a test, documentation or testdata change is judged %+v", freshness)
	}
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	freshness := binaryStaleAgainst(ctx, root, own)
	if freshness.Code != selfBinaryStaleCode || !strings.Contains(freshness.Detail, "1 Go source paths differ at HEAD: self.go;") || !strings.Contains(freshness.Detail, "go install ./cmd/corvint") {
		t.Fatalf("a Go source change was not reported with its recovery: %+v", freshness)
	}
	for name, info := range map[string]*debug.BuildInfo{
		"another module": selfBinaryInfo("example.test/other", built),
		"no Git stamp":   {Main: debug.Module{Path: "example.test/self"}},
	} {
		if freshness := binaryStaleAgainst(ctx, root, info); freshness.Code != "" {
			t.Errorf("%s: judged %+v", name, freshness)
		}
	}
	missing := selfBinaryInfo("example.test/self", strings.Repeat("ab", 20))
	if freshness := binaryStaleAgainst(ctx, root, missing); freshness.Code != selfBinaryUnverifiedCode || !strings.Contains(freshness.Detail, "could not be compared with HEAD") {
		t.Fatalf("a commit absent here was not reported unverified: %+v", freshness)
	}
}

// TSP-V0-722: only a change to a package the binary's main package reaches,
// or to the module files, makes it stale; a change to another command or to a
// package nothing it builds imports does not.
func TestSelfBinaryStaleCountsOnlyTheMainBuildGraph(t *testing.T) {
	ctx := context.Background()
	root, _ := selfBinaryRepository(t)
	commitSelfBinaryFiles(t, root, map[string]string{
		"cmd/tool/main.go":  "package main\n\nimport _ \"example.test/self/lib\"\n\nfunc main() {}\n",
		"lib/lib.go":        "package lib\n\nimport _ \"example.test/self/lib/deep\"\n",
		"lib/deep/deep.go":  "//go:build windows\n\npackage deep\n",
		"cmd/other/main.go": "package main\n\nfunc main() {}\n",
		"internal/up/up.go": "package up\n",
	})
	built := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	info := selfBinaryInfo("example.test/self", built)
	info.Path = "example.test/self/cmd/tool"
	commitSelfBinaryFiles(t, root, map[string]string{
		"cmd/other/main.go": "package main\n\nfunc main() { println() }\n",
		"internal/up/up.go": "package up\n\nvar X = 1\n",
		"self.go":           "package self\n\nfunc Self() int { return 3 }\n",
	})
	if freshness := binaryStaleAgainst(ctx, root, info); freshness.Code != "" {
		t.Fatalf("changes outside the main build graph are judged %+v", freshness)
	}
	for name, mainPath := range map[string]string{"main package absent": "example.test/self/cmd/gone", "main package unknown": ""} {
		other := selfBinaryInfo("example.test/self", built)
		other.Path = mainPath
		if freshness := binaryStaleAgainst(ctx, root, other); freshness.Code != selfBinaryStaleCode {
			t.Errorf("%s: judged %+v, want every changed path counted", name, freshness)
		}
	}
	commitSelfBinaryFiles(t, root, map[string]string{"go.work": "go 1.23\n\nuse .\n"})
	if freshness := binaryStaleAgainst(ctx, root, info); freshness.Code != selfBinaryStaleCode || !strings.Contains(freshness.Detail, "cmd/other/main.go") {
		t.Errorf("with go.work: judged %+v, want every changed path counted", freshness)
	}
	affectedGit(t, root, "rm", "-q", "go.work")
	affectedGit(t, root, "commit", "-qm", "drop go.work")
	if freshness := binaryStaleAgainst(ctx, root, info); freshness.Code != "" {
		t.Fatalf("without go.work again: judged %+v", freshness)
	}
	for _, change := range []map[string]string{
		{"lib/deep/deep.go": "//go:build windows\n\npackage deep\n\nvar X = 1\n"},
		{"cmd/tool/main.go": "package main\n\nimport _ \"example.test/self/lib\"\n\nfunc main() { println() }\n"},
		{"go.mod": "module example.test/self\n\ngo 1.23\n"},
	} {
		commitSelfBinaryFiles(t, root, change)
		for changed := range change {
			freshness := binaryStaleAgainst(ctx, root, info)
			if freshness.Code != selfBinaryStaleCode || !strings.Contains(freshness.Detail, changed) {
				t.Fatalf("%s: judged %+v, want stale naming it", changed, freshness)
			}
		}
	}
}

// AFP-V0-846: output the bound truncated drops its possibly partial last path
// and counts "at least" the rest.
func TestSelfBinaryDiffPathsTruncation(t *testing.T) {
	var output bytes.Buffer
	writer := &limitedWriter{buffer: &output, limit: 10}
	_, _ = writer.Write([]byte("a.go\x00b.go\x00cdef.go\x00"))
	paths, count := diffPaths(output.Bytes(), output.Len() == 10)
	if output.String() != "a.go\x00b.go\x00" || len(paths) != 1 || paths[0] != "a.go" || count != "at least 1" {
		t.Fatalf("truncated: %q %v %q", output.String(), paths, count)
	}
	if paths, count = diffPaths([]byte("a.go\x00b.go\x00"), false); len(paths) != 2 || count != "2" {
		t.Fatalf("complete: %v %q", paths, count)
	}
}

// AFP-V0-846: affected names self-binary-stale in its JSON receipt and on
// stderr, and corvint test prints it, when the binary is stale for the root;
// a fresh binary adds neither.
func TestSelfBinaryStaleIsReportedByAffectedAndTest(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 || receipt["degradations"] != nil || strings.Contains(stderr, selfBinaryStaleCode) {
		t.Fatalf("a fresh binary reported a degradation: exit %d %v %s", code, receipt["degradations"], stderr)
	}
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	for _, options := range [][]string{nil, {"--full"}} {
		receipt, _, stderr, code = runAffectedCLI(t, root, options...)
		degradations, _ := receipt["degradations"].([]any)
		if code != 0 || len(degradations) != 1 || degradations[0] != selfBinaryStaleCode {
			t.Fatalf("%v: degradations %v, exit %d: %s", options, receipt["degradations"], code, stderr)
		}
		if !strings.Contains(stderr, "corvint: DEGRADED self-binary-stale: this binary was built from commit "+built[:12]) {
			t.Fatalf("%v: stderr does not name the degradation: %s", options, stderr)
		}
	}
	// AFP-V0-990 (V1-3648 review): --by-path keeps the degradation in both formats.
	byPath, _, stderr, code := runAffectedCLI(t, root, "--by-path", "--path", "self.go")
	if degradations, _ := byPath["degradations"].([]any); code != 0 || len(degradations) != 1 || degradations[0] != selfBinaryStaleCode || !strings.Contains(stderr, "corvint: DEGRADED self-binary-stale") {
		t.Fatalf("--by-path: degradations %v, exit %d: %s", byPath["degradations"], code, stderr)
	}
	var text, textStderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--by-path", "--path", "self.go", "--format", "text"}, strings.NewReader(""), &text, &textStderr); code != 0 || !strings.Contains(text.String(), "\ndegraded self-binary-stale\n") {
		t.Fatalf("--by-path text: exit %d\n%s%s", code, text.String(), textStderr.String())
	}
	var stdout, testStderr bytes.Buffer
	runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &stdout, &testStderr)
	if !strings.Contains(testStderr.String(), "corvint-test: DEGRADED self-binary-stale: ") {
		t.Fatalf("corvint test does not name the degradation: %s", testStderr.String())
	}
}

// DCW-V0-396: a dogfood flow run by a stale binary, or handed one through a
// change-step or verifier flag, refuses before it binds or checks anything and
// names the recovery; the base verifier and propose-citations are not judged,
// and an unverified binary is a note.
func TestDogfoodFlowRefusesStaleBinary(t *testing.T) {
	ctx := context.Background()
	root, built := selfBinaryRepository(t)
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	for _, args := range [][]string{{"change", built}, {"finish-lane", "--base", built}, {"inputs-check", "--base", built}, {"check", built}, {"seal", built}, {"batch-check", built}} {
		var stdout, stderr bytes.Buffer
		code := runDogfoodFlow(ctx, root, args, &stdout, &stderr)
		want := "dogfood-" + args[0] + ": REFUSE self-binary-stale "
		if code != 2 || !strings.HasPrefix(stderr.String(), want) || !strings.Contains(stderr.String(), "unset CORVINT_BIN") {
			t.Fatalf("%s: exit %d, stderr %q", args[0], code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	runDogfoodFlow(ctx, root, []string{"propose-citations", built}, &stdout, &stderr)
	if strings.Contains(stderr.String(), "self-binary") {
		t.Fatalf("propose-citations judged the binary: %s", stderr.String())
	}

	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", head))
	executables := map[string]*debug.BuildInfo{
		"/stale":      selfBinaryInfo("example.test/self", built),
		"/unverified": selfBinaryInfo("example.test/self", strings.Repeat("ab", 20)),
		"/fresh":      selfBinaryInfo("example.test/self", head),
		// The running binary is judged once as itself; a flag naming it is
		// skipped even though this stub would call it stale.
		"/self": selfBinaryInfo("example.test/self", built),
	}
	saved := readExecutableBuildInfo
	readExecutableBuildInfo = func(path string) (*debug.BuildInfo, error) {
		if info, ok := executables[path]; ok {
			return info, nil
		}
		return nil, fs.ErrNotExist
	}
	t.Cleanup(func() { readExecutableBuildInfo = saved })
	for _, name := range []string{"--corvint-bin", "--tree-verifier", "--override-verifier", "--verifier"} {
		refusal, notes := dogfoodStaleBinary(ctx, root, "/self", map[string]string{name: "/stale"})
		if !strings.HasPrefix(refusal, "self-binary-stale "+name+" /stale: ") || len(notes) != 0 {
			t.Errorf("%s: refusal %q notes %v", name, refusal, notes)
		}
	}
	if refusal, notes := dogfoodStaleBinary(ctx, root, "/self", map[string]string{"--base-verifier": "/stale", "--tree-verifier": "/fresh", "--verifier": "/self"}); refusal != "" || len(notes) != 0 {
		t.Errorf("the base verifier, a fresh binary or the running binary was judged: %q %v", refusal, notes)
	}
	refusal, notes := dogfoodStaleBinary(ctx, root, "/self", map[string]string{"--corvint-bin": "/unverified"})
	if refusal != "" || len(notes) != 1 || !strings.HasPrefix(notes[0], "self-binary-unverified --corvint-bin /unverified: ") {
		t.Errorf("an unverified binary must be a note: %q %v", refusal, notes)
	}
}
