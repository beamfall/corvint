package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// refusalRun is one forced refusal of a converted family: the process outcome plus the complete
// envelope the CLI must emit for it.
type refusalRun struct {
	exit           int
	stdout, stderr string
	want           map[string]any
}

func refusalEnvelope(code, message string, subject map[string]any, evidence []any, fixes []any, terminal string) map[string]any {
	envelope := map[string]any{
		"code": code, "error": message, "ok": false,
		"subject": subject, "evidence": evidence, "supported_fixes": fixes,
	}
	if terminal != "" {
		envelope["terminal"] = terminal
	}
	return envelope
}

func subjectOf(kind, value string) map[string]any {
	return map[string]any{"kind": kind, "value": value}
}

func evidenceOf(pairs ...string) []any {
	evidence := []any{}
	for index := 0; index < len(pairs); index += 2 {
		evidence = append(evidence, map[string]any{"name": pairs[index], "value": pairs[index+1]})
	}
	return evidence
}

func fixesOf(identifiers ...string) []any {
	fixes := []any{}
	for _, identifier := range identifiers {
		fixes = append(fixes, identifier)
	}
	return fixes
}

func cliRun(want map[string]any, arguments ...string) func(*testing.T) refusalRun {
	return func(t *testing.T) refusalRun {
		exit, stdout, stderr := runCLI(t, arguments...)
		return refusalRun{exit: exit, stdout: stdout, stderr: stderr, want: want}
	}
}

func platformEnvelope(code, message string) map[string]any {
	return refusalEnvelope(code, message, subjectOf("host-capability", "native-platform"), evidenceOf("platform", "windows"), fixesOf(), "unsupported-platform")
}

func impactPathSuffixEnvelope(path string) map[string]any {
	return refusalEnvelope("unsupported-impact-path-suffix", "native Go impact requires a suffix admitted by the repository index", subjectOf("value", path), evidenceOf(), fixesOf("impact.omit-unadmitted-path", "affected.use-path"), "")
}

func resolvedTestRoot(t *testing.T, directory string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// TestConvertedRefusalDiagnostics forces one refusal per site converted after the working-tree
// impact family and asserts, through the CLI emission path, the complete DRC-V0-001..005 members,
// the DRC-V0-006 code and error bytes ahead of them, and the DRC-V0-010 exit, envelope and stdout.
func TestConvertedRefusalDiagnostics(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*testing.T) refusalRun{
		"explicit root without .git": func(t *testing.T) refusalRun {
			directory := t.TempDir()
			root := resolvedTestRoot(t, directory)
			want := refusalEnvelope("invalid-arguments", "not a Git repository: "+root, subjectOf("value", directory), evidenceOf("root", root), fixesOf("cli.use-git-repository-root"), "")
			return cliRun(want, "--root", directory, "affected")(t)
		},
		"genesis platform": func(t *testing.T) refusalRun {
			var stdout, stderr bytes.Buffer
			exit := runActivationForPlatform(context.Background(), filepath.Join(t.TempDir(), "missing"), "init", nil, "windows", &stdout, &stderr)
			return refusalRun{exit: exit, stdout: stdout.String(), stderr: stderr.String(), want: platformEnvelope("unsupported-genesis-platform", "native Go Genesis inventory is qualified only on Darwin and Linux")}
		},
		"impact budget option": func(t *testing.T) refusalRun {
			want := refusalEnvelope("unsupported-impact-option", "--budget-bytes applies only to path impact, not --base", subjectOf("argument", "--budget-bytes"), evidenceOf(), fixesOf("impact.omit-budget-bytes"), "")
			return cliRun(want, "--root", impactCLIRepository(t), "impact", "--base", "HEAD", "--budget-bytes", "2048")(t)
		},
		"affected without git": func(t *testing.T) refusalRun {
			stdout, stderr, exit := runCandidateWithEnvironment(t, "", []string{"PATH=" + t.TempDir()}, "--root", affectedFixtureRepository(t), "affected")
			want := refusalEnvelope("unsupported-affected-status", "git executable is unavailable", subjectOf("host-capability", "git-executable"), evidenceOf(), fixesOf("host.put-git-on-path"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"affected unknown base": func(t *testing.T) refusalRun {
			base := strings.Repeat("0", 40)
			want := refusalEnvelope("unsupported-affected-revision", "base "+base+" is not a commit in this repository", subjectOf("value", base), evidenceOf(), fixesOf("affected.use-commit-base", "affected.omit-base"), "")
			return cliRun(want, "--root", affectedFixtureRepository(t), "affected", "--base", base)(t)
		},
		"affected without HEAD": func(t *testing.T) refusalRun {
			root := t.TempDir()
			affectedGit(t, root, "init", "-q")
			want := refusalEnvelope("unsupported-affected-revision", "repository has no resolvable HEAD commit", subjectOf("repository-state", "head-commit"), evidenceOf(), fixesOf("git.create-head-commit"), "")
			return cliRun(want, "--root", root, "affected")(t)
		},
		"impact platform": func(t *testing.T) refusalRun {
			_, err := parseImpactArgumentsForPlatform(options{}, []string{"pkg/main.go"}, "windows")
			var stderr bytes.Buffer
			emitError(&stderr, err)
			return refusalRun{exit: 2, stderr: stderr.String(), want: platformEnvelope("unsupported-impact-platform", "native Go impact is qualified only on Darwin and Linux")}
		},
		"impact path suffix": func(t *testing.T) refusalRun {
			return cliRun(impactPathSuffixEnvelope("README.csv"), "--root", impactCLIRepository(t), "impact", "README.csv")(t)
		},
	}
	for name, force := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			outcome := force(t)
			if outcome.exit != 2 || outcome.stdout != "" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", outcome.exit, outcome.stdout, outcome.stderr)
			}
			prefix := "{\"code\": " + pythonJSONString(outcome.want["code"].(string)) + ", \"error\": " + pythonJSONString(outcome.want["error"].(string)) + ", \"evidence\": ["
			if !strings.HasPrefix(outcome.stderr, prefix) {
				t.Fatalf("code and error bytes changed:\n got %q\nwant prefix %q", outcome.stderr, prefix)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(outcome.stderr), &got); err != nil {
				t.Fatalf("stderr is not one JSON envelope: %v: %q", err, outcome.stderr)
			}
			if !reflect.DeepEqual(got, outcome.want) {
				t.Fatalf("envelope:\n got %#v\nwant %#v", got, outcome.want)
			}
		})
	}
}
