package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

func TestEscapeASCIIJSONMatchesPythonSpelling(t *testing.T) {
	got := escapeASCIIJSON([]byte("{\"text\":\"caf\u00e9 \u2014 \U0001f642\u007f\"}"))
	want := []byte(`{"text":"caf\u00e9 \u2014 \ud83d\ude42\u007f"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("escaped JSON = %q, want %q", got, want)
	}
}

// CCF-V1-004 (accepted 2026-09-26, decision 0422; from decision 0398): the two
// CEM read failures keep the oracle's fixed text and add their code.
func TestReadFailuresKeepTheFixedTextAndAddTheirCode(t *testing.T) {
	for code, want := range map[string]string{
		cemcode.PatchUnavailable: `{"code": "patch-unavailable", "error": "cannot read patch", "ok": false}` + "\n",
		cemcode.MapUnavailable:   `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}` + "\n",
	} {
		var stderr bytes.Buffer
		emitCEMError(&stderr, cemcode.New(code, "cannot open .corvint/detail"))
		if stderr.String() != want {
			t.Fatalf("stderr=%q, want %q", stderr.String(), want)
		}
	}
}

// CEM-PILOT-028,029: the native argument parser admits the new formats and
// optional OCM input without changing the existing report default.
func TestReviewProjectionArguments(t *testing.T) {
	t.Run("CEM-PILOT-028 admitted formats retain report default", func(t *testing.T) {
		spec := cemActions["report"]
		for _, args := range [][]string{{"--map", "map.json", "--format", "json", "--ocm", "map.ocm.json"}, {"--map", "map.json", "--format", "markdown"}, {"--map", "map.json"}} {
			flags, unrecognized, err := parseCEMFlags(spec, args)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateParsed(spec, flags, unrecognized); err != nil {
				t.Fatal(err)
			}
		}
		flags, unrecognized, err := parseCEMFlags(spec, []string{"--map", "map.json", "--format", "html"})
		if err == nil {
			err = validateParsed(spec, flags, unrecognized)
		}
		if err == nil {
			t.Fatal("unsupported report format admitted")
		}
	})
}

// CEM-PILOT-029: optional verifier state is invocation-local. An unavailable
// verifier retains the hunk denominator and creates no obligation join.
func TestReviewProjectionPerCallResolver(t *testing.T) {
	t.Run("CEM-PILOT-029 invocation-local resolver and unavailable gap", func(t *testing.T) {
		root := t.TempDir()
		git := func(args ...string) string {
			t.Helper()
			command := exec.Command("git", args...)
			command.Dir = root
			command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
			raw, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, raw)
			}
			return string(bytes.TrimSpace(raw))
		}
		git("init", "-q")
		if err := os.WriteFile(filepath.Join(root, "change.txt"), []byte("before\n"), 0600); err != nil {
			t.Fatal(err)
		}
		git("add", ".")
		git("commit", "-qm", "base")
		base := git("rev-parse", "HEAD")
		if err := os.WriteFile(filepath.Join(root, "change.txt"), []byte("after\n"), 0600); err != nil {
			t.Fatal(err)
		}
		git("add", ".")
		git("commit", "-qm", "target")
		target := git("rev-parse", "HEAD")
		session, err := workflow.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		argv := []string{"report", "--map", wire.ExcludedCEMPath, "--expected-base", base, "--target", target, "--format", "json", "--ocm", "intent.ocm.json"}
		run := func(reader workflow.ReviewOCMReader) (int, map[string]any) {
			var stdout, stderr bytes.Buffer
			exit := RunWithOCM(context.Background(), root, argv, &stdout, &stderr, reader)
			var envelope map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode: %v %s", err, stderr.String())
			}
			return exit, envelope["review"].(map[string]any)
		}
		calls := 0
		reader := func(reason string) workflow.ReviewOCMReader {
			return func(_ context.Context, _, _ string, raw []byte, gotBase, gotTarget string) (map[string]any, error) {
				calls++
				if len(raw) == 0 || gotBase != base || gotTarget != target {
					t.Fatal("unbound callback")
				}
				return map[string]any{"valid": true, "state": reason, "obligations": []any{}, "claims": []any{}}, nil
			}
		}
		firstExit, first := run(reader("first"))
		secondExit, second := run(reader("second"))
		unavailableExit, unavailable := run(nil)
		if firstExit != 0 || secondExit != 0 || unavailableExit != 1 || calls != 2 {
			t.Fatalf("exits/calls: %d %d %d %d", firstExit, secondExit, unavailableExit, calls)
		}
		if first["ocm"].(map[string]any)["state"] != "first" || second["ocm"].(map[string]any)["state"] != "second" {
			t.Fatal("resolver leaked between calls")
		}
		if unavailable["ocm"].(map[string]any)["reason"] != "ocm-verifier-unavailable" || len(unavailable["hunks"].([]any)) != 1 {
			t.Fatal("unavailable OCM omitted hunk denominator")
		}
	})
}
