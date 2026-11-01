package cli

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// remoteHintTimeout bounds each read-only git probe of the hint.
const remoteHintTimeout = 5 * time.Second

// remoteProjectionHint is the CTS-V0-821 addition to a journal-absent
// ticket show miss: it reads this repository's own origin remote-tracking
// ref, without fetching, and names the exact sync step for the ticket's
// intent file. It never decodes or serves the remote record; any git
// failure leaves the CTS-V0-010 warning as it is ("").
func remoteProjectionHint(repo *intent.Repository, id string) string {
	local := id[strings.LastIndexByte(id, ':')+1:]
	if !plainFileName(local) || repo.CommonDir == "" {
		return ""
	}
	ref, sha := "", ""
	if name, ok := remoteGit(repo.CommonDir, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); ok {
		if commit, ok := remoteGit(repo.CommonDir, "rev-parse", "--verify", "-q", "refs/remotes/origin/HEAD^{commit}"); ok {
			ref, sha = name, commit
		}
	}
	if sha == "" {
		commit, ok := remoteGit(repo.CommonDir, "rev-parse", "--verify", "-q", "refs/remotes/origin/main^{commit}")
		if !ok {
			return ""
		}
		ref, sha = "origin/main", commit
	}
	rel := intent.Dir + "/" + intent.TicketsDir + "/" + local + ".json"
	if _, ok := remoteGit(repo.CommonDir, "cat-file", "-e", sha+":"+rel); ok {
		return " The remote-tracking ref " + ref + " (commit " + sha + ") holds " + rel + ": read it read-only with `git show " + sha + ":" + rel +
			"`, or sync this checkout's projection with `git merge --ff-only " + ref + "` on a checkout of that branch, then retry (CTS-V0-821)."
	}
	return " The remote-tracking ref " + ref + " (commit " + sha + ") does not hold " + rel + " either; `git fetch origin` refreshes that ref (this read never fetches), then retry (CTS-V0-821)."
}

// plainFileName admits a ticket local id that names one file in tickets/.
func plainFileName(s string) bool {
	if s == "" || s[0] == '.' || s[0] == '-' {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// remoteGit runs one bounded, read-only git command against the common
// directory with the sanitized plan-batches environment.
func remoteGit(gitDir string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteHintTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "credential.helper=", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull}, args...)...)
	c.Dir = gitDir
	c.Env = append(batchGitEnvironment(), "GIT_DIR="+gitDir)
	c.WaitDelay = time.Second
	out := &boundedBuffer{max: 4096}
	c.Stdout = out
	if err := c.Run(); err != nil || out.over {
		return "", false
	}
	return strings.TrimSpace(out.buf.String()), true
}
