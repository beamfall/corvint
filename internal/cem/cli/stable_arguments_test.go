package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// stableArgumentCase is one argument-stage refusal the portable consumer
// pairs in interop/cem01-go (TestCEMV1115ArgumentRefusalsPortable): the same
// stage, code and fix-line presence for the same argv.
type stableArgumentCase struct {
	name  string
	args  func(neutral, keys string) []string
	stage string
	code  string
	fix   string // the option the stderr fix line names; "" means no line
}

const (
	stableArgumentBase   = "467cbb73c1102cd7df0299cd74a6bef96ee3a9a4"
	stableArgumentTarget = "1f181cd0843318b72bb6679cd19878c46eb95ada"
)

func stableArgumentCases() []stableArgumentCase {
	full := func(repository, artifacts string, extra ...string) func(string, string) []string {
		return func(neutral, keys string) []string {
			a := []string{"--repository", repository, "--map", filepath.Join(neutral, "stable.json"), "--expected-base", stableArgumentBase, "--target", stableArgumentTarget}
			if artifacts == "NEUTRAL" {
				artifacts = neutral
			}
			if artifacts != "" {
				a = append(a, "--artifacts", artifacts)
			}
			for _, e := range extra {
				if e == "KEYS" {
					e = keys
				}
				a = append(a, e)
			}
			return a
		}
	}
	return []stableArgumentCase{
		{"missing map", func(neutral, _ string) []string {
			return []string{"--repository", "/nonexistent-repository", "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", neutral}
		}, "arguments", "invalid-arguments", ""},
		{"relative repository", full("relative/repository", "NEUTRAL"), "arguments", "invalid-arguments", "--repository"},
		{"repeated repository with a relative value", func(neutral, _ string) []string {
			return append([]string{"--repository", "/nonexistent-repository"}, full("relative/repository", "NEUTRAL")(neutral, "")...)
		}, "arguments", "invalid-arguments", "--repository"},
		{"trusted keys with no keys", full("/nonexistent-repository", "NEUTRAL", "--trusted-keys", "KEYS"), "arguments", "invalid-arguments", ""},
		{"relative artifacts", full("/nonexistent-repository", "relative/artifacts"), "authority-arguments", "invalid-arguments", "--artifacts"},
		{"missing artifacts", full("/nonexistent-repository", ""), "authority-arguments", "artifacts-required", ""},
	}
}

// writeEmptyTrustedKeys writes a trusted-keys document whose key list is empty,
// which both consumers refuse at the arguments stage.
func writeEmptyTrustedKeys(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trusted-keys.json")
	if err := os.WriteFile(path, []byte(`{"profile":"corvint-trusted-keys/1","keys":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func stableArgumentNeutral(t *testing.T) string {
	t.Helper()
	neutral, err := filepath.Abs("../../../protocol/cem-1.0/stable/neutral-criterion")
	if err != nil {
		t.Fatal(err)
	}
	return neutral
}

// TestCEMV1115ArgumentRefusalsNative checks that an arguments or
// authority-arguments refusal is UNSUPPORTED with accept null and exit 2
// (CEM-V1-115, CEM-V1-116), and that a relative path option is refused with
// the absolute-path fix on stderr, never inside the result (CEM-V1-117).
func TestCEMV1115ArgumentRefusalsNative(t *testing.T) {
	neutral, keys := stableArgumentNeutral(t), writeEmptyTrustedKeys(t)
	cases := append(stableArgumentCases(), stableArgumentCase{"relative root", func(neutral, _ string) []string {
		return append([]string{"--root", "relative/root"}, []string{"--repository", "/nonexistent-repository", "--map", filepath.Join(neutral, "stable.json"), "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", neutral}...)
	}, "arguments", "invalid-arguments", "--root"})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args(neutral, keys)
			if args[0] == "--root" {
				args = append(append(append([]string{}, args[:2]...), "cem", "verify-stable"), args[2:]...)
			} else {
				args = append([]string{"cem", "verify-stable"}, args...)
			}
			var out, errout bytes.Buffer
			exit, handled := RunStableInvocation(context.Background(), args, &out, &errout)
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !handled {
				t.Fatalf("handled %v, stdout %q: %v", handled, out.String(), err)
			}
			if exit != 2 || result["outcome"] != "UNSUPPORTED" || result["accept"] != nil || result["assurance"] != nil || result["stage"] != c.stage || result["code"] != c.code {
				t.Fatalf("exit %d result outcome %v accept %v stage %v code %v, want UNSUPPORTED %s/%s exit 2", exit, result["outcome"], result["accept"], result["stage"], result["code"], c.stage, c.code)
			}
			requireFixLine(t, errout.String(), "corvint cem verify-stable: ", c.fix)
			if strings.Contains(out.String(), "fix:") {
				t.Fatalf("the fix leaked into the closed result: %s", out.String())
			}
		})
	}
	// The absolute form of each relative option passes the argument stages and
	// prints no fix line (CEM-V1-117: refused, never resolved).
	var out, errout bytes.Buffer
	exit, _ := RunStableInvocation(context.Background(), []string{"--root", "/nonexistent-repository", "cem", "verify-stable", "--repository", "/nonexistent-repository", "--map", filepath.Join(neutral, "stable.json"), "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", neutral}, &out, &errout)
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || exit != 2 || result["stage"] == "arguments" || result["stage"] == "authority-arguments" || errout.Len() != 0 {
		t.Fatalf("absolute form: exit %d stage %v stderr %q: %v", exit, result["stage"], errout.String(), err)
	}
}

func requireFixLine(t *testing.T, stderr, prefix, option string) {
	t.Helper()
	if option == "" {
		if stderr != "" {
			t.Fatalf("unexpected stderr %q", stderr)
		}
		return
	}
	want := prefix + "invalid-arguments: relative path option " + option + "; fix: pass each as an absolute path such as \"$PWD/<path>\"; a relative path is refused, never resolved\n"
	if stderr != want {
		t.Fatalf("stderr %q, want %q", stderr, want)
	}
}

// TestCEMV1117CoreReadNamesRelativePathFix checks that a Core stable read with
// a relative artifact root or repository keeps the UNSUPPORTED refusal and
// names the fix in the envelope beside, never inside, the result (CEM-V1-117).
func TestCEMV1117CoreReadNamesRelativePathFix(t *testing.T) {
	f := materializeStableRead(t)
	for _, c := range []struct {
		name, repository, artifacts, stage, option string
	}{
		{"relative artifacts", "", "relative/artifacts", "authority-arguments", "--artifacts"},
		{"relative root", "relative/root", f.artifacts, "arguments", "--root"},
		{"absolute forms", "", f.artifacts, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			session, err := workflow.Open(f.repo)
			if err != nil {
				t.Fatal(err)
			}
			if c.repository != "" {
				session.SetStableRepository(c.repository)
			}
			envelope, err := session.Read(context.Background(), "verify", workflow.ReadOptions{MapPath: "change.cem.json", ExpectedBase: f.expectedBase, Target: f.target, ArtifactRoot: c.artifacts})
			if err != nil {
				t.Fatal(err)
			}
			verification, _ := envelope["stableVerification"].(workflow.StableVerification)
			fix, _ := envelope["fix"].(string)
			if c.option == "" {
				if fix != "" || verification.Result.Stage == "arguments" || verification.Result.Stage == "authority-arguments" {
					t.Fatalf("absolute forms: fix %q stage %s", fix, verification.Result.Stage)
				}
				return
			}
			if verification.Exit != 2 || verification.Result.Outcome != "UNSUPPORTED" || verification.Result.Accept != nil || verification.Result.Stage != c.stage || verification.Result.Code == nil || *verification.Result.Code != "invalid-arguments" {
				t.Fatalf("result %+v exit %d, want UNSUPPORTED %s/invalid-arguments", verification.Result, verification.Exit, c.stage)
			}
			if !strings.Contains(fix, "relative path option "+c.option+";") || !strings.Contains(fix, "absolute path") {
				t.Fatalf("envelope fix %q does not name %s", fix, c.option)
			}
		})
	}
}
