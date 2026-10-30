package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitSmokeRepoPinsIntentBranch(t *testing.T) {
	root := t.TempDir()
	git := filepath.Join(root, "git")
	if err := os.WriteFile(git, []byte(`#!/bin/sh
if [ "$1" = init ]; then
	case " $* " in
		*" -b main "*) ;;
		*) exit 97 ;;
	esac
fi
exec /usr/bin/git "$@"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_COMPANION_GIT", git)
	repository := filepath.Join(root, "repo")
	if _, err := initSmokeRepo(context.Background(), repository, queuePolicyFiles{Queue: []byte("{}\n"), Policy: []byte("{}\n")}); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runCaptured(context.Background(), repository, closedGitEnv(root), subprocessTimeout, git, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(stdout)); got != "main" {
		t.Fatalf("smoke repository branch = %q, want main", got)
	}
}

func TestRunAtmJSONRetainsStructuredStdoutOnFailure(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "corvint-tasks")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' '{\"outcome\":\"REFUSED\",\"codes\":[\"INTENT_BRANCH_MISMATCH\"]}'\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	observed, _, err := runAtmJSON(context.Background(), binary, root, "tasks-init", "init")
	if err == nil || observed.OK {
		t.Fatalf("failed command accepted: step=%+v err=%v", observed, err)
	}
	for _, want := range []string{`"outcome":"REFUSED"`, `"INTENT_BRANCH_MISMATCH"`} {
		if !strings.Contains(observed.Detail, want) || !strings.Contains(err.Error(), want) {
			t.Fatalf("structured stdout omitted from failure: step=%+v err=%v", observed, err)
		}
	}
}
