package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ciSources are the fixture files. Every line is distinctive so a report containing any of them
// is detectable (CEM-PILOT-021).
var ciBaseSources = map[string]string{
	"spec.md": "Requirement R-1: session tokens expire after nine hundred seconds.\n" +
		"Requirement R-2: refresh tokens rotate on every successful use.\n",
	"auth.go": "package auth\n\n// tokenLifetimeSeconds is the session token lifetime.\nconst tokenLifetimeSeconds = 600\n",
}

var ciHeadSources = map[string]string{
	"auth.go": "package auth\n\n// tokenLifetimeSeconds is the session token lifetime.\nconst tokenLifetimeSeconds = 900\n",
}

type ciFixture struct {
	repo string
	base string
	work string // head tree without the map
	head string // work plus the map
}

// ciPatchArgs is the documented CEM CI patch profile (docs/CEM-CI.md), run through the git CLI
// independently of the verifier's own derivation.
func ciPatchArgs(base, head string) []string {
	return []string{"-c", "core.quotePath=false", "-c", "diff.algorithm=myers", "-c", "diff.context=3",
		"diff", "--binary", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames",
		"--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3",
		"--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none",
		base, head, "--", ".", ":(exclude).corvint/change.cem.json"}
}

func writeSources(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// newCIFixture commits the base and the change, then builds a supported map citing citePath's
// last line, lets edit alter it, and commits it as the head. A nil edit result commits no map.
func newCIFixture(t *testing.T, citePath string, edit func(*cemMap) []byte) ciFixture {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "CEM Test")
	gitRun(t, dir, "config", "user.email", "cem@example.invalid")
	writeSources(t, dir, ciBaseSources)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	f := ciFixture{repo: dir, base: strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))}
	writeSources(t, dir, ciHeadSources)
	gitRun(t, dir, "commit", "-q", "-am", "change")
	f.work = strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	patch := []byte(gitRun(t, dir, ciPatchArgs(f.base, f.work)...))
	m := ciSupportedMap(t, f, citePath, patch)
	mapBytes := edit(&m)
	f.head = f.work
	if mapBytes == nil {
		return f
	}
	if err := os.MkdirAll(filepath.Join(dir, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".corvint", "change.cem.json"), mapBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".corvint/change.cem.json")
	gitRun(t, dir, "commit", "-q", "-m", "map")
	f.head = strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	return f
}

func ciSupportedMap(t *testing.T, f ciFixture, citePath string, patch []byte) cemMap {
	t.Helper()
	parsed, parseErr := parsePatch(patch)
	if parseErr != nil || len(parsed.hunks) != 1 {
		t.Fatalf("fixture patch: hunks=%d err=%v", len(parsed.hunks), parseErr)
	}
	body := strings.TrimSuffix(ciBaseSources[citePath], "\n")
	e := evidence{Path: citePath, BlobOID: strings.TrimSpace(gitRun(t, f.repo, "rev-parse", f.base+":"+citePath)),
		Span: span{Start: uint64(strings.LastIndexByte(body, '\n') + 1), End: uint64(len(body))}}
	e.SpanSHA256 = shaHex([]byte(body[e.Span.Start:e.Span.End]))
	e.ID = "evidence:sha256:" + shaHex(canonicalEvidence(e))
	ph := parsed.hunks[0]
	return cemMap{Spec: specVersion, BaseRevision: f.base, PatchSHA256: shaHex(patch), Evidence: []evidence{e},
		Hunks: []mappedHunk{{ID: ph.ID, Path: "auth.go", OldRange: ph.OldRange, NewRange: ph.NewRange,
			Disposition: "supported", Reason: "evidence-backed", Basis: []basis{{EvidenceID: e.ID, Relation: "specification"}}}}}
}

func marshalMap(t *testing.T, m *cemMap) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func asIs(t *testing.T) func(*cemMap) []byte {
	return func(m *cemMap) []byte { return marshalMap(t, m) }
}

// assertSourceFree fails when a report holds any non-empty line of any fixture source version.
func assertSourceFree(t *testing.T, report []byte) {
	t.Helper()
	for _, sources := range []map[string]string{ciBaseSources, ciHeadSources} {
		for name, body := range sources {
			for _, line := range strings.Split(body, "\n") {
				if len(strings.TrimSpace(line)) > 8 && bytes.Contains(report, []byte(line)) {
					t.Fatalf("CEM-PILOT-021 report carries a line of %s: %q", name, line)
				}
			}
		}
	}
}

func TestCIVerdictsAndExits(t *testing.T) {
	t.Parallel()
	absentOID := strings.Repeat("ab", 20)
	cases := []struct {
		name, cite, verdict, code string
		exit                      int
		edit                      func(*testing.T) func(*cemMap) []byte
		head                      func(ciFixture) string
	}{
		{"accepted", "spec.md", "accepted", "accepted", 0, asIs, nil},
		{"unsafe drift", "auth.go", "rejected", "unsafe-drift", 1, asIs, nil},
		{"invalid map", "spec.md", "rejected", "patch-digest", 1, func(t *testing.T) func(*cemMap) []byte {
			return func(m *cemMap) []byte { m.PatchSHA256 = strings.Repeat("0", 64); return marshalMap(t, m) }
		}, nil},
		{"map absent", "spec.md", "missing-evidence", "map-absent", 3, func(*testing.T) func(*cemMap) []byte {
			return func(*cemMap) []byte { return nil }
		}, nil},
		{"unknown hunk", "spec.md", "missing-evidence", "unknown-hunks", 3, func(t *testing.T) func(*cemMap) []byte {
			return func(m *cemMap) []byte {
				m.Evidence, m.Hunks[0].Disposition, m.Hunks[0].Reason, m.Hunks[0].Basis = []evidence{}, "unknown", "no-evidence", []basis{}
				return marshalMap(t, m)
			}
		}, nil},
		{"unsupported profile", "spec.md", "unsupported-profile", "unsupported-profile", 4, func(t *testing.T) func(*cemMap) []byte {
			return func(m *cemMap) []byte { m.Spec = "cem/0.2"; return marshalMap(t, m) }
		}, nil},
		{"map base mismatch", "spec.md", "repository-mismatch", "base-revision-mismatch", 5, func(t *testing.T) func(*cemMap) []byte {
			return func(m *cemMap) []byte { m.BaseRevision = strings.Repeat("cd", 20); return marshalMap(t, m) }
		}, nil},
		{"head not in repository", "spec.md", "repository-mismatch", "head-unavailable", 5, asIs,
			func(ciFixture) string { return absentOID }},
	}
	for _, tc := range cases {
		t.Run("CEM-PILOT-022 "+tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCIFixture(t, tc.cite, tc.edit(t))
			head := f.head
			if tc.head != nil {
				head = tc.head(f)
			}
			argv := []string{"--repository", f.repo, "--base", f.base, "--head", head}
			r := runCI(argv)
			if r.Verdict != tc.verdict || r.Code != tc.code || r.Exit != tc.exit {
				t.Fatalf("got %s/%s/%d, want %s/%s/%d", r.Verdict, r.Code, r.Exit, tc.verdict, tc.code, tc.exit)
			}
			report := encodeCIReport(r)
			if again := encodeCIReport(runCI(argv)); !bytes.Equal(report, again) {
				t.Fatalf("CEM-PILOT-021 report is not deterministic:\n%s\n%s", report, again)
			}
			if bytes.Count(report, []byte("\n")) != 1 || len(report) > maxCIReportBytes {
				t.Fatalf("CEM-PILOT-021 report is not one bounded line: %d bytes", len(report))
			}
			assertSourceFree(t, report)
		})
	}
}

func TestCIAcceptedReportShape(t *testing.T) {
	t.Parallel()
	f := newCIFixture(t, "spec.md", asIs(t))
	r := runCI([]string{"--repository", f.repo, "--base", f.base, "--head", f.head})
	if r.Hunks != (ciHunks{Supported: 1}) || r.Evidence != 1 || len(r.Drift) != 1 || r.Drift[0].Status != "stable" ||
		len(r.MapSHA256) != 64 || len(r.PatchSHA256) != 64 || r.MapPath != ciDefaultMapPath || r.Profile != "cem/0.1" {
		t.Fatalf("CEM-PILOT-021 accepted report: %+v", r)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encodeCIReport(r), &members); err != nil || len(members) != 14 {
		t.Fatalf("CEM-PILOT-021 fixed schema: %d members, err=%v", len(members), err)
	}
}

func TestCIInvocationEchoesNothing(t *testing.T) {
	t.Parallel()
	for _, argv := range [][]string{
		{},
		{"--repository", "/r", "--base", strings.Repeat("a", 40)},
		{"--repository", "/r", "--base", strings.Repeat("a", 40), "--head", strings.Repeat("b", 40), "--map", "../x"},
		{"--repository", "/r", "--base", strings.Repeat("a", 40), "--head", strings.Repeat("b", 40), "--map", "a b"},
		{"--repository", "/r", "--base", "HEAD", "--head", strings.Repeat("b", 40)},
	} {
		r := runCI(argv)
		if r.Verdict != "operational" || r.Code != "invocation" || r.Exit != 2 || r.Base != "" || r.Head != "" || r.MapPath != "" {
			t.Fatalf("CEM-PILOT-022 invocation %q: %+v", argv, r)
		}
	}
}

// TestCIReportWorstCaseBound builds the largest report the verifier's own limits allow.
func TestCIReportWorstCaseBound(t *testing.T) {
	t.Parallel()
	oid := strings.Repeat("f", 64)
	r := ciReport{Schema: ciSchema, Verdict: "missing-evidence", Code: "unknown-hunks", Profile: specVersion,
		Base: oid, Head: oid, MapPath: strings.Repeat("a", 512), MapSHA256: oid, PatchSHA256: oid, Limits: ciLimits,
		Hunks: ciHunks{Supported: maxHunks, Mechanical: maxHunks, Unknown: maxHunks}, Evidence: maxEvidence}
	for i := 0; i < maxEvidence; i++ {
		r.Drift = append(r.Drift, driftItem{EvidenceID: "evidence:sha256:" + oid, Path: strings.Repeat(`"`, 512),
			Status: "relocated", TargetBlobOID: &oid, TargetSpan: &nullableSpan{Start: maxWireInteger, End: maxWireInteger}})
	}
	if got := len(encodeCIReport(r)); got > maxCIReportBytes {
		t.Fatalf("CEM-PILOT-021 worst-case report is %d bytes, bound %d", got, maxCIReportBytes)
	}
}

// buildOffline installs the verifier as the example workflow does, from a module proxy, with the
// Go checksum database off because the proxy is a local directory.
func buildOffline(t *testing.T, proxy, cache, gobin, version string) []byte {
	t.Helper()
	cmd := exec.Command("go", "install", "-trimpath", ciModulePath+"@"+version)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod -modcacherw", "GOPROXY="+proxy, "GOSUMDB=off",
		"GOMODCACHE="+cache, "GOBIN="+gobin, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go install (GOPROXY=%s): %v: %s", proxy, err, out)
	}
	exe, err := os.ReadFile(filepath.Join(gobin, "cem01-go"))
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

const ciModulePath = "github.com/Beamfall/corvint/interop/cem01-go"

// writeFileProxy publishes this module directory as one pseudo-version in a GOPROXY file tree.
func writeFileProxy(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "github.com", "!beamfall", "corvint", "interop", "cem01-go", "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		body, readErr := os.ReadFile(entry.Name())
		w, createErr := zw.Create(ciModulePath + "@" + version + "/" + entry.Name())
		if readErr != nil || createErr != nil {
			t.Fatal(readErr, createErr)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{version + ".zip": archive.Bytes(), version + ".mod": mod, "list": []byte(version + "\n"),
		version + ".info": []byte(`{"Version":"` + version + `","Time":"2026-09-22T00:00:00Z"}`)}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return "file://" + filepath.ToSlash(root)
}

// networkDenied returns a command prefix that runs a child without network access, or nil.
func networkDenied() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"sandbox-exec", "-p", "(version 1)(allow default)(deny network*)"}
	case "linux":
		return []string{"unshare", "--user", "--map-root-user", "--net"}
	}
	return nil
}

func runWith(prefix []string, env []string, name string, args ...string) (int, []byte, []byte) {
	argv := append(append([]string{}, prefix...), append([]string{name}, args...)...)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.Bytes(), stderr.Bytes()
	}
	if err != nil {
		return -1, stdout.Bytes(), []byte(err.Error())
	}
	return 0, stdout.Bytes(), stderr.Bytes()
}

// TestCIHelperDial is a child-process helper: it dials CEM_CI_DIAL and exits 0 only on success.
func TestCIHelperDial(t *testing.T) {
	addr := os.Getenv("CEM_CI_DIAL")
	if addr == "" {
		return
	}
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		os.Exit(7)
	}
	_ = conn.Close()
	os.Exit(0)
}

// TestCIPortableWorkflowOffline follows the example workflow: fetch and build a pinned verifier,
// check its digest before execution, then run it with network access denied (CEM-PILOT-020).
func TestCIPortableWorkflowOffline(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "examples", "cem", "verify-portable.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("NOT_RUN: example script absent outside the Corvint repository: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("NOT_RUN: the example script is POSIX-only")
	}
	version := "v0.0.0-20260922000000-000000000000"
	proxy := writeFileProxy(t, version)
	fetched := buildOffline(t, proxy, t.TempDir(), t.TempDir(), version)
	rebuilt := buildOffline(t, proxy, t.TempDir(), t.TempDir(), version)
	if shaHex(fetched) != shaHex(rebuilt) {
		t.Fatal("CEM-PILOT-020 two fresh-cache installs of the pinned version differ, so no digest can pin it")
	}
	exeDir := t.TempDir()
	exe := filepath.Join(exeDir, "cem01-go")
	if err := os.WriteFile(exe, fetched, 0o755); err != nil {
		t.Fatal(err)
	}

	f := newCIFixture(t, "spec.md", asIs(t))
	gitRun(t, f.repo, "branch", "base-only", f.base)
	baseCheckout := filepath.Join(t.TempDir(), "base")
	gitRun(t, f.repo, "clone", "-q", "--no-local", "--single-branch", "--branch", "base-only", f.repo, baseCheckout)
	env := []string{"CEM_REPOSITORY=" + baseCheckout, "CEM_HEAD_REPOSITORY=" + f.repo, "CEM_BASE_SHA=" + f.base,
		"CEM_HEAD_SHA=" + f.head, "CEM_VERIFIER=" + exe, "CEM_VERIFIER_SHA256=" + shaHex(fetched)}
	want := encodeCIReport(runCI([]string{"--repository", f.repo, "--base", f.base, "--head", f.head}))

	marker := filepath.Join(t.TempDir(), "executed")
	stub := filepath.Join(exeDir, "stub")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\n: >'"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubEnv := append(append([]string{}, env...), "CEM_VERIFIER="+stub, "CEM_VERIFIER_SHA256="+strings.Repeat("0", 64))
	if code, stdout, _ := runWith(nil, stubEnv, script); code != 2 || len(stdout) != 0 {
		t.Fatalf("CEM-PILOT-020 digest mismatch: exit %d stdout %q", code, stdout)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("CEM-PILOT-020 a verifier whose digest mismatched was executed")
	}

	prefix := networkDenied()
	if prefix == nil {
		t.Skipf("NOT_RUN: no network sandbox for %s", runtime.GOOS)
	}
	if code, _, stderr := runWith(prefix, nil, "true"); code != 0 {
		t.Skipf("NOT_RUN: network sandbox unavailable here: %s", stderr)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	dial := []string{"CEM_CI_DIAL=" + listener.Addr().String()}
	if code, _, stderr := runWith(nil, dial, os.Args[0], "-test.run=^TestCIHelperDial$"); code != 0 {
		t.Fatalf("control dial failed: exit %d: %s", code, stderr)
	}
	if code, _, _ := runWith(prefix, dial, os.Args[0], "-test.run=^TestCIHelperDial$"); code == 0 {
		t.Fatal("the network sandbox allowed a TCP connection")
	}
	code, stdout, stderr := runWith(prefix, env, script)
	if code != 0 || !bytes.Equal(stdout, want) {
		t.Fatalf("CEM-PILOT-020 network-denied run: exit %d\nstdout %s\nwant   %s\nstderr %s", code, stdout, want, stderr)
	}
	assertSourceFree(t, stdout)
}
