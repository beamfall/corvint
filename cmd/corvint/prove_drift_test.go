package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProveRefusesDriftInEveryMode(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, mode, drift string
	}{
		{"task dirty-set drift", "task", "dirty"},
		{"task HEAD-tree drift", "task", "tree"},
		{"task control", "task", "control"},
		{"paths dirty-set drift", "paths", "dirty"},
		{"paths HEAD-tree drift", "paths", "tree"},
		{"paths control", "paths", "control"},
		{"base dirty-set drift", "base", "dirty"},
		{"base HEAD-tree drift", "base", "tree"},
		{"base control", "base", "control"},
		{"cem dirty-set drift", "cem", "dirty"},
		{"cem HEAD-tree drift", "cem", "tree"},
		{"cem map-bytes drift", "cem", "map"},
		{"cem control", "cem", "control"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, arguments := proveDriftFixture(t, test.mode)
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			if test.drift == "map" {
				mapPath := filepath.Join(root, ".corvint", "change.cem.json")
				contents, err := os.ReadFile(mapPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(mapPath, append(contents, ' '), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			environment := checkpointGitWrapper(t, proveDriftScript(test.drift, root, realGit))
			_, stdout, stderr, code := runProveCLIEnvironment(t, environment, root, arguments...)
			if test.drift == "control" {
				if code != 0 {
					t.Fatalf("control exited %d: %s", code, stderr)
				}
				return
			}
			if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "unsupported-prove-drift") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func proveDriftFixture(t *testing.T, mode string) (string, []string) {
	t.Helper()
	switch mode {
	case "task":
		return proveFixtureRepository(t), []string{"--task", authorityStartPrompt, "--limit", "1"}
	case "paths":
		return proveGoRepository(t), []string{"pkg/core/core.go"}
	case "base":
		root, base := proveChangeRepository(t)
		return root, []string{"--base", base}
	case "cem":
		root, base := proveCEMRepository(t)
		return root, []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD"}
	default:
		t.Fatalf("unknown prove mode %q", mode)
		return "", nil
	}
}

func proveDriftScript(drift, root, realGit string) string {
	switch drift {
	case "dirty":
		return `case " $* " in
 *" --ignored=no "*)
  if [ -f "$0.status" ]; then printf '?? drift.go\000'; exit 0; fi
  : > "$0.status";;
esac`
	case "tree":
		return `if [ -f "$0.closed" ]; then
 for arg in "$@"; do
  if [ "$arg" = 'HEAD^{tree}' ]; then printf '0000000000000000000000000000000000000000\n'; exit 0; fi
 done
fi
case " $* " in
 *" --ignored=no "*)
  if [ -f "$0.status" ]; then : > "$0.closed"; else : > "$0.status"; fi;;
esac`
	case "map":
		return `case " $* " in
 *" --ignored=no "*)
  if [ -f "$0.status" ]; then
   ` + shellQuote(realGit) + ` "$@"
   code=$?
   printf ' ' >> ` + shellQuote(filepath.Join(root, ".corvint", "change.cem.json")) + `
   exit "$code"
  fi
  : > "$0.status";;
esac`
	default:
		return ":"
	}
}
