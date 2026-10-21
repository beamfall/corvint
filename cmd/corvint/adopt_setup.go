package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// adopt --setup (ADS-V0): one read-only, machine-readable report of a
// repository's Corvint setup gaps. It reads the snapshot store, the root agent
// instruction files, repository-scoped hook wiring and `.gitignore` rules, and
// writes nothing (invariant 4).

const (
	setupReportProfile = "corvint-setup-report/0"
	setupReadLimit     = 1 << 20
	setupGitTimeout    = 10 * time.Second
)

// setupInstructionFiles are the root agent-instruction files the report reads.
var setupInstructionFiles = []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"}

// setupHookFiles are the repository-scoped host files that can wire Corvint's
// hooks: a project-scope Claude Code plugin enablement or hook entry, or a
// Codex project hooks file. User-scope installs are outside the repository and
// are not read.
var setupHookFiles = []string{".claude/settings.json", ".codex/hooks.json"}

// setupIgnoredPaths are Corvint's private local state paths, each of which
// must be ignored by Git so derived or ledger state is never committed.
var setupIgnoredPaths = []string{
	".context-corvint/",
	".corvint/index/",
	".corvint/self-observations.jsonl",
	".corvint/unplanned-reads.jsonl",
	".corvint/unplanned-reads.enabled",
}

type setupCheck struct {
	Class    string   `json:"class"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
	Fix      string   `json:"fix,omitempty"`
}

func runSetupReport(ctx context.Context, root string, stdout, stderr io.Writer) int {
	checks := []setupCheck{
		setupIndexCheck(ctx, root),
		setupInstructionCheck(root),
		setupHookCheck(root),
		setupIgnoreCheck(ctx, root),
	}
	gaps := 0
	rows := make([]any, 0, len(checks))
	for _, check := range checks {
		if check.Status != "ok" {
			gaps++
		}
		row := map[string]any{"class": check.Class, "status": check.Status, "evidence": append([]string{}, check.Evidence...)}
		if check.Fix != "" {
			row["fix"] = check.Fix
		}
		rows = append(rows, row)
	}
	state := "READY"
	if gaps != 0 {
		state = "GAPS"
	}
	report := map[string]any{"profile": setupReportProfile, "state": state, "gap_count": gaps, "checks": rows}
	payload := map[string]any{"ok": true, "mutates": false, "tool": "adopt", "setup": report}
	encoded, err := contextindex.CanonicalJSON(payload)
	if err == nil {
		_, err = stdout.Write(append(encoded, '\n'))
	}
	if err != nil {
		emitError(stderr, argumentError("cannot write adopt --setup output"))
		return 2
	}
	return 0
}

// setupIndexCheck reports whether a current snapshot of the committed tree
// exists for this binary, using the read-only snapshot probe.
func setupIndexCheck(ctx context.Context, root string) setupCheck {
	check := setupCheck{Class: "index-snapshot", Status: "ok"}
	_, reason, err := contextindex.SnapshotFreshness(ctx, root)
	check.Evidence = []string{"snapshot: " + reason}
	if err != nil {
		check.Status, check.Evidence = "unobserved", append(check.Evidence, "error: "+err.Error())
		return check
	}
	if reason != "matching" {
		check.Status, check.Fix = "gap", "corvint --root "+root+" index --if-stale"
	}
	return check
}

// setupInstructionCheck reports whether a root agent-instruction file names
// Corvint, so an agent reading it is routed to Corvint's commands.
func setupInstructionCheck(root string) setupCheck {
	check := setupCheck{Class: "agent-instructions", Status: "gap"}
	for _, name := range setupInstructionFiles {
		content, ok := setupRead(root, name)
		if !ok {
			continue
		}
		if bytes.Contains(bytes.ToLower(content), []byte("corvint")) {
			check.Status = "ok"
			check.Evidence = append(check.Evidence, name+": names corvint")
		} else {
			check.Evidence = append(check.Evidence, name+": present, does not name corvint")
		}
	}
	if len(check.Evidence) == 0 {
		check.Evidence = []string{"no root " + strings.Join(setupInstructionFiles, ", ") + " file"}
	}
	if check.Status == "gap" {
		check.Fix = "add a Corvint section to AGENTS.md naming `corvint context`, `corvint impact` and `corvint affected`"
	}
	return check
}

// setupHookCheck reports whether a repository-scoped host file wires Corvint's
// hooks (a `corvint@` plugin enablement or a `corvint adapter` command).
func setupHookCheck(root string) setupCheck {
	check := setupCheck{Class: "hooks", Status: "gap"}
	for _, name := range setupHookFiles {
		content, ok := setupRead(root, name)
		if !ok {
			continue
		}
		if bytes.Contains(content, []byte("corvint adapter")) || bytes.Contains(content, []byte(`"corvint@`)) {
			check.Status = "ok"
			check.Evidence = append(check.Evidence, name+": wires corvint")
		} else {
			check.Evidence = append(check.Evidence, name+": present, no corvint hook or plugin")
		}
	}
	if len(check.Evidence) == 0 {
		check.Evidence = []string{"no repository-scoped hook file (" + strings.Join(setupHookFiles, ", ") + "); user-scope installs are not read"}
	}
	if check.Status == "gap" {
		check.Fix = "claude plugin install corvint@corvint --scope project (see integrations/claude-code/README.md), or the Codex plugin in integrations/codex/README.md"
	}
	return check
}

// setupIgnoreCheck reports each Corvint private state path that Git's ignore
// rules do not cover. `check-ignore --no-index` reads rules only; with
// GIT_OPTIONAL_LOCKS=0 it takes no index lock and writes nothing.
func setupIgnoreCheck(ctx context.Context, root string) setupCheck {
	check := setupCheck{Class: "ignored-paths", Status: "ok"}
	git, err := exec.LookPath("git")
	if err != nil {
		check.Status, check.Evidence = "unobserved", []string{"git: not found"}
		return check
	}
	ctx, cancel := context.WithTimeout(ctx, setupGitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, git, "-C", root, "check-ignore", "--no-index", "--stdin", "--non-matching", "--verbose")
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	command.Stdin = strings.NewReader(strings.Join(setupIgnoredPaths, "\n") + "\n")
	var output bytes.Buffer
	command.Stdout = &output
	runErr := command.Run()
	var exit *exec.ExitError
	if runErr != nil && !(errors.As(runErr, &exit) && exit.ExitCode() == 1) {
		check.Status, check.Evidence = "unobserved", []string{"git check-ignore: " + runErr.Error()}
		return check
	}
	for _, line := range strings.Split(strings.TrimRight(output.String(), "\n"), "\n") {
		source, path, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		// A non-matching path prints `::`; a negated pattern (`!...`) matches but un-ignores.
		fields := strings.SplitN(source, ":", 3)
		if source == "::" || len(fields) == 3 && strings.HasPrefix(fields[2], "!") {
			check.Status = "gap"
			check.Evidence = append(check.Evidence, path+": not ignored")
		}
	}
	if check.Status == "ok" {
		check.Evidence = []string{"all " + strconv.Itoa(len(setupIgnoredPaths)) + " Corvint local state paths are ignored"}
	} else {
		check.Fix = "add the listed paths to .gitignore"
	}
	return check
}

// setupRead reads one regular repository file of at most setupReadLimit bytes
// through the repository root, never following a link out of it.
func setupRead(root, name string) ([]byte, bool) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, false
	}
	defer repository.Close()
	file, err := repository.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, false
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	content, err := io.ReadAll(io.LimitReader(file, setupReadLimit+1))
	if err != nil || len(content) > setupReadLimit {
		return nil, false
	}
	return content, true
}
