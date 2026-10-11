package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestExampleStableBranch drives examples/cem/verify-portable.sh with CEM_PROFILE=cem/1.0 over the
// public stable positive repository and compares its output and exit with a direct
// verify-stable-canonical invocation of the same executable (CEM-V1-PSR-004).
func TestExampleStableBranch(t *testing.T) {
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
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
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
	direct := func() (int, []byte) {
		code, stdout, _ := runWith(nil, nil, exe, "verify-stable-canonical", "--repository", r.repo, "--map", mapFile,
			"--expected-base", s0eBase, "--target", r.target, "--artifacts", r.artifacts)
		return code, stdout
	}
	env := []string{"CEM_REPOSITORY=" + r.repo, "CEM_BASE_SHA=" + s0eBase, "CEM_HEAD_SHA=" + r.target,
		"CEM_VERIFIER=" + exe, "CEM_VERIFIER_SHA256=" + shaHex(body), "CEM_ARTIFACTS=" + r.artifacts, "CEM_PROFILE=cem/1.0"}
	with := func(extra ...string) []string { return append(append([]string{}, env...), extra...) }

	wantCode, want := direct()
	code, stdout, stderr := runWith(nil, env, script)
	if code != wantCode || !bytes.Equal(stdout, want) {
		t.Fatalf("CEM-V1-PSR-004 stable branch: exit %d want %d\nstdout %s\nwant   %s\nstderr %s", code, wantCode, stdout, want, stderr)
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(stdout, &result) != nil || string(result["profile"]) != `"cem-stable-verification/1"` || string(result["spec"]) != `"cem/1.0"` {
		t.Fatalf("CEM-V1-PSR-004 stable branch did not emit the stable result: %s", stdout)
	}

	// A tampered artifact root changes both runs identically; the script never masks the exit.
	if err := os.WriteFile(filepath.Join(r.artifacts, "runner-plan.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantCode, want = direct()
	if code, stdout, stderr := runWith(nil, env, script); code != wantCode || code == 0 || !bytes.Equal(stdout, want) {
		t.Fatalf("CEM-V1-PSR-004 tampered: exit %d want %d\nstdout %s\nwant   %s\nstderr %s", code, wantCode, stdout, want, stderr)
	}

	// Input refusals exit before the verifier runs and write no report.
	for name, extra := range map[string][]string{
		"artifacts unset":    {"CEM_ARTIFACTS="},
		"artifacts relative": {"CEM_ARTIFACTS=artifacts"},
		"artifacts missing":  {"CEM_ARTIFACTS=" + filepath.Join(r.B, "absent")},
		"unknown profile":    {"CEM_PROFILE=cem/0.2"},
		"digest mismatch":    {"CEM_VERIFIER_SHA256=" + strings.Repeat("0", 64)},
	} {
		if code, stdout, _ := runWith(nil, with(extra...), script); code != 2 || len(stdout) != 0 {
			t.Fatalf("CEM-V1-PSR-004 %s: exit %d stdout %q", name, code, stdout)
		}
	}
	if code, stdout, _ := runWith(nil, with("CEM_MAP_PATH=.corvint/absent.cem.json"), script); code != 3 || len(stdout) != 0 {
		t.Fatalf("CEM-V1-PSR-004 absent map: exit %d stdout %q", code, stdout)
	}

	// Without the explicit profile the default ci path runs unchanged and routes the stable map.
	code, stdout, _ = runWith(nil, with("CEM_PROFILE="), script)
	wantCI := encodeCIReport(runCI([]string{"--repository", r.repo, "--base", s0eBase, "--head", r.target}))
	if code != 4 || !bytes.Equal(stdout, wantCI) || !bytes.Contains(stdout, []byte(`"code":"`+ciStableRouteCode+`"`)) {
		t.Fatalf("CEM-V1-PSR-004 default profile: exit %d\nstdout %s\nwant   %s", code, stdout, wantCI)
	}
}
