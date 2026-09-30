package gitauth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestPinnedRepositoryGitAllReadPaths(t *testing.T) {
	root, base, _ := makeRepo(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err = filepath.EvalSymlinks(realGit)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	log := filepath.Join(outside, "calls")
	binary := filepath.Join(outside, "host-git")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := fmt.Sprintf("#!/bin/sh\n[ \"${GIT_ALLOW_PROTOCOL+x}\" = x ] && [ -z \"$GIT_ALLOW_PROTOCOL\" ] || exit 91\nprintf '%%s\\n' \"$*\" >> %s\nexec %s \"$@\"\n", quote(log), quote(realGit))
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	repository, err := OpenPinned(root, gitrun.NewDefaultBudget(), binary)
	if err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(outside, "git")
	marker := filepath.Join(outside, "planted-ran")
	if err := os.WriteFile(decoy, []byte("#!/bin/sh\ntouch "+quote(marker)+"\nexit 92\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", outside+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx := context.Background()
	if got, err := repository.Resolve(ctx, base); err != nil || got != base {
		t.Fatalf("one-shot %q %v", got, err)
	}
	release := repository.BeginObjectSession()
	entry, exists, err := repository.LookupTreeEntry(ctx, base, "docs/rule.txt")
	if err != nil || !exists {
		t.Fatalf("session entry %v %v", exists, err)
	}
	if _, err := repository.BlobBytes(ctx, entry.OID); err != nil {
		t.Fatal(err)
	}
	// A refused co-process answer must replay through the same explicit pin.
	if _, err := repository.sessionRead(ctx, []string{entry.OID}, batchBody, func([]string, int) bool { return false }, 4096, []byte(entry.OID+"\n"), "cat-file", "--batch"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CommitTree(ctx, base); err != nil {
		t.Fatal(err)
	}
	release()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !strings.Contains(call, "core.hooksPath="+os.DevNull) {
			t.Fatalf("unisolated call %q", call)
		}
	}
	if len(strings.Split(strings.TrimSpace(string(data)), "\n")) < 4 {
		t.Fatalf("missing execution paths: %s", data)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient Git ran: %v", err)
	}
	if _, err := OpenPinned(root, gitrun.NewDefaultBudget(), "git"); err == nil {
		t.Fatal("relative pin admitted")
	}
	ordinary, err := Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.gitOptions(100, nil).Binary != "" || strings.Contains(strings.Join(ordinary.gitOptions(100, nil).Env, "\n"), "GIT_ALLOW_PROTOCOL=") || strings.Contains(strings.Join(ordinary.pinnedArgs(), " "), "core.hooksPath=") {
		t.Fatal("ordinary Open invocation changed")
	}
}
