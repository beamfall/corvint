package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Portable stable routing (CEM-V1-PSR-001..003): the default verify path and the ci mode peek the
// declared spec of a strictly parsed map, so an exact cem/1.0 map is refused as unsupported and ci
// names the stable verifier, while every other profile and malformed map keeps its earlier result.

// sealedStableMap returns the public stable positive map bytes, or skips outside the repository.
func sealedStableMap(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "repository-envelope-packet", "inherited", "fixtures", "maps", "sha1-positive-sealed.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("NOT_RUN: stable packet absent outside the Corvint repository: %v", err)
	}
	return b
}

func TestDefaultPathRefusesStableSpec(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string][]byte{
		"minimal": []byte(`{"spec":"cem/1.0"}`),
		"escaped": []byte(`{"spec":"cem\/1.0","baseRevision":7}`),
		"sealed":  sealedStableMap(t),
	} {
		if _, e := decodeMap(raw); e == nil || e.code != "unsupported-spec" || e.operational {
			t.Fatalf("CEM-V1-PSR-001 %s: %+v", name, e)
		}
	}
	// The peek precedes the patch read, so an absent patch cannot hide the profile refusal.
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "change.cem.json")
	if err := os.WriteFile(mapFile, sealedStableMap(t), 0o600); err != nil {
		t.Fatal(err)
	}
	_, e := verify(cliArgs{repository: dir, mapFile: mapFile, patchFile: filepath.Join(dir, "absent.patch")})
	if e == nil || e.code != "unsupported-spec" || e.operational {
		t.Fatalf("CEM-V1-PSR-001 verify: %+v", e)
	}
}

func TestDefaultPathKeepsOtherProfileRefusals(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		`{"spec":"cem/0.2"}`:                        "version-shape",
		`{"spec":"cem/1.0-experimental.1"}`:         "version-shape",
		`{"spec":"cem/1.0 "}`:                       "version-shape",
		`{"spec":"CEM/1.0"}`:                        "version-shape",
		`{"spec":"cem/0.1","spec":"cem/1.0"}`:       "invalid-json",
		`{"spec":"cem/1.0","spec":"cem/1.0"}`:       "invalid-json",
		`{"spec":1}`:                                "invalid-json",
		`["cem/1.0"]`:                               "invalid-json",
		`{"spec":"cem/1.0"} {}`:                     "invalid-json",
		"{\"spec\":\"cem/1.0\",\"x\":\"\xff\"}":     "invalid-json",
		`{"spec":"cem/1.0","x":"\ud800"}`:           "invalid-json",
		`{"spec":null}`:                             "version-shape",
		`{"spec":"cem/0.1","criterionLinks":[]}`:    "invalid-json",
		`{"criterionLinks":[],"spec":"cem/0.1"}`:    "invalid-json",
		`{"baseRevision":"cem/1.0","spec":"other"}`: "version-shape",
		`{"SPEC":"cem/1.0"}`:                        "version-shape",
	} {
		if _, e := decodeMap([]byte(raw)); e == nil || e.code != want {
			t.Fatalf("CEM-V1-PSR-003 %q: got %+v, want %s", raw, e, want)
		}
	}
}

func TestCIRoutesStableSpecToStableVerifier(t *testing.T) {
	t.Parallel()
	sealed := sealedStableMap(t)
	cases := []struct {
		name, code string
		body       func(*cemMap) []byte
	}{
		{"sealed", ciStableRouteCode, func(*cemMap) []byte { return sealed }},
		{"minimal", ciStableRouteCode, func(*cemMap) []byte { return []byte(`{"spec":"cem/1.0"}`) }},
		{"candidate", "unsupported-profile", func(*cemMap) []byte { return []byte(`{"spec":"cem/1.0-experimental.1"}`) }},
		{"0.2", "unsupported-profile", func(m *cemMap) []byte { m.Spec = "cem/0.2"; return marshalMap(t, m) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCIFixture(t, "spec.md", tc.body)
			r := runCI([]string{"--repository", f.repo, "--base", f.base, "--head", f.head})
			if r.Verdict != "unsupported-profile" || r.Exit != 4 || r.Code != tc.code || r.Profile != specVersion {
				t.Fatalf("CEM-V1-PSR-002 got %s/%s/%d profile %s, want unsupported-profile/%s/4", r.Verdict, r.Code, r.Exit, r.Profile, tc.code)
			}
			var members map[string]json.RawMessage
			if err := json.Unmarshal(encodeCIReport(r), &members); err != nil || len(members) != 14 {
				t.Fatalf("CEM-PILOT-021 fixed schema: %d members, err=%v", len(members), err)
			}
		})
	}
	if ciStableRouteCode != "use-verify-stable-canonical" {
		t.Fatalf("CEM-V1-PSR-002 route code %q does not name the stable verify command", ciStableRouteCode)
	}
}

// stableRouteFrozenTuple is the only Go runtime on which the stable canonical verifier admits the
// S0e fixtures' structural hunks (cem.go verifyStable03 runtime stage).
const stableRouteFrozenTuple = "go1.27.1"

type exampleStable struct {
	script, exe, mapFile string
	run                  *s0eRun
	env                  []string
}

// newExampleStable builds the verifier with the running toolchain and prepares the public S0e
// repository and artifact root for examples/cem/verify-portable.sh (CEM-V1-PSR-004).
func newExampleStable(t *testing.T) *exampleStable {
	t.Helper()
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
	exe := filepath.Join(t.TempDir(), "cem01-go")
	build := exec.Command("go", "build", "-trimpath", "-o", exe, ".")
	// The built verifier must run on the same toolchain as this test, so the tuple gate below
	// describes the executable the script runs.
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN="+runtime.Version(), "GOPROXY=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	r := newS0eRun(t)
	mapFile := filepath.Join(t.TempDir(), "change.cem.json")
	if err := os.WriteFile(mapFile, r.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return &exampleStable{script: script, exe: exe, mapFile: mapFile, run: r, env: []string{
		"CEM_REPOSITORY=" + r.repo, "CEM_BASE_SHA=" + s0eBase, "CEM_HEAD_SHA=" + r.target,
		"CEM_VERIFIER=" + exe, "CEM_VERIFIER_SHA256=" + shaHex(body), "CEM_ARTIFACTS=" + r.artifacts, "CEM_PROFILE=cem/1.0"}}
}

func (x *exampleStable) with(extra ...string) []string {
	return append(append([]string{}, x.env...), extra...)
}

// direct runs verify-stable-canonical on the same executable and inputs the script uses.
func (x *exampleStable) direct() (int, []byte) {
	code, stdout, _ := runWith(nil, nil, x.exe, "verify-stable-canonical", "--repository", x.run.repo, "--map", x.mapFile,
		"--expected-base", s0eBase, "--target", x.run.target, "--artifacts", x.run.artifacts)
	return code, stdout
}

// script runs the example and requires its stdout and exit to equal the direct invocation.
func (x *exampleStable) runScript(t *testing.T, label string) stableResult {
	t.Helper()
	wantCode, want := x.direct()
	code, stdout, stderr := runWith(nil, x.env, x.script)
	if code != wantCode || !bytes.Equal(stdout, want) {
		t.Fatalf("CEM-V1-PSR-004 %s: exit %d want %d\nstdout %s\nwant   %s\nstderr %s", label, code, wantCode, stdout, want, stderr)
	}
	var res stableResult
	if err := json.Unmarshal(stdout, &res); err != nil || res.Profile != "cem-stable-verification/1" || res.Spec != stableSpec {
		t.Fatalf("CEM-V1-PSR-004 %s did not emit the stable result (%v): %s", label, err, stdout)
	}
	return res
}

func strOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// TestExampleStableBranchVerifies proves the opt-in branch reaches repository and artifact
// verification: the sealed S0e change is accepted, and a tampered artifact is rejected at the
// artifact stage (CEM-V1-PSR-004, migration row C19).
func TestExampleStableBranchVerifies(t *testing.T) {
	if runtime.Version() != stableRouteFrozenTuple {
		t.Skipf("NOT_RUN: the stable S0e fixtures need the %s structural tuple; running %s", stableRouteFrozenTuple, runtime.Version())
	}
	x := newExampleStable(t)
	res := x.runScript(t, "positive")
	if res.Outcome != "ACCEPT" || res.Accept == nil || !*res.Accept {
		t.Fatalf("CEM-V1-PSR-004 positive: outcome %s stage %s code %s", res.Outcome, res.Stage, strOf(res.Code))
	}
	if err := os.WriteFile(filepath.Join(x.run.artifacts, "runner-plan.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = x.runScript(t, "tampered artifact")
	if res.Outcome != "REJECT" || res.Stage != "artifacts" || !slices.Contains(res.IssueCodes, "artifact-digest-mismatch") {
		t.Fatalf("CEM-V1-PSR-004 tampered artifact: outcome %s stage %s issues %v", res.Outcome, res.Stage, res.IssueCodes)
	}
}

// TestExampleStableBranchRuntimeRefusal shows an unqualified runtime is passed through as the
// verifier's own UNSUPPORTED result, never masked or weakened by the script.
func TestExampleStableBranchRuntimeRefusal(t *testing.T) {
	if runtime.Version() == stableRouteFrozenTuple {
		t.Skipf("NOT_RUN: %s is the qualified tuple; TestExampleStableBranchVerifies covers it", stableRouteFrozenTuple)
	}
	x := newExampleStable(t)
	res := x.runScript(t, "unqualified runtime")
	if res.Outcome != "UNSUPPORTED" || res.Stage != "runtime" || strOf(res.Code) != "unsupported-structural-runtime" {
		t.Fatalf("CEM-V1-PSR-004 runtime refusal: outcome %s stage %s code %s", res.Outcome, res.Stage, strOf(res.Code))
	}
}

// TestExampleStableBranchRefusals covers the script's own refusals, which precede the verifier and
// do not depend on the runtime tuple, and the unchanged default path.
func TestExampleStableBranchRefusals(t *testing.T) {
	x := newExampleStable(t)
	for name, extra := range map[string][]string{
		"artifacts unset":    {"CEM_ARTIFACTS="},
		"artifacts relative": {"CEM_ARTIFACTS=artifacts"},
		"artifacts missing":  {"CEM_ARTIFACTS=" + filepath.Join(x.run.B, "absent")},
		"unknown profile":    {"CEM_PROFILE=cem/0.2"},
		"digest mismatch":    {"CEM_VERIFIER_SHA256=" + strings.Repeat("0", 64)},
	} {
		if code, stdout, _ := runWith(nil, x.with(extra...), x.script); code != 2 || len(stdout) != 0 {
			t.Fatalf("CEM-V1-PSR-004 %s: exit %d stdout %q", name, code, stdout)
		}
	}
	if code, stdout, _ := runWith(nil, x.with("CEM_MAP_PATH=.corvint/absent.cem.json"), x.script); code != 3 || len(stdout) != 0 {
		t.Fatalf("CEM-V1-PSR-004 absent map: exit %d stdout %q", code, stdout)
	}

	// A directory selector or tree entry never stands in for the map, even when the directory
	// holds exactly one regular file whose bytes the verifier would otherwise read.
	repo := filepath.Join(t.TempDir(), "repo")
	gitIn := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(filepath.Join(repo, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "maps", "one.cem.json"), x.run.raw, 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn("add", "maps")
	gitIn("commit", "-qm", "maps")
	head := gitIn("rev-parse", "HEAD")
	for _, selector := range []string{"maps/", "maps"} {
		code, stdout, stderr := runWith(nil, x.with("CEM_REPOSITORY="+repo, "CEM_BASE_SHA="+head, "CEM_HEAD_SHA="+head,
			"CEM_MAP_PATH="+selector), x.script)
		if code != 2 || len(stdout) != 0 || !bytes.Contains(stderr, []byte("cem-ci: CEM_MAP_PATH")) {
			t.Fatalf("CEM-V1-PSR-004 directory selector %q: exit %d stdout %q stderr %q", selector, code, stdout, stderr)
		}
	}

	// Without the explicit profile the default ci path runs unchanged and routes the stable map.
	code, stdout, _ := runWith(nil, x.with("CEM_PROFILE="), x.script)
	wantCI := encodeCIReport(runCI([]string{"--repository", x.run.repo, "--base", s0eBase, "--head", x.run.target}))
	if code != 4 || !bytes.Equal(stdout, wantCI) || !bytes.Contains(stdout, []byte(`"code":"`+ciStableRouteCode+`"`)) {
		t.Fatalf("CEM-V1-PSR-004 default profile: exit %d\nstdout %s\nwant   %s", code, stdout, wantCI)
	}
}
