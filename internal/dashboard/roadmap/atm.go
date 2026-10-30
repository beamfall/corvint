package roadmap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	maxEnvelopeBytes = 32 << 20
	envelopeProfile  = "taskman-command-result/0"
	defaultTimeout   = 30 * time.Second
)

// envelope is the subset of `atm`'s taskman-command-result/0 reply this join
// reads. Items stay untyped JSON so each caller decodes only the shape it
// needs (roadmap summary vs. ticket detail), matching internal/console's
// Envelope convention for the same profile.
type envelope struct {
	Profile string            `json:"profile"`
	Outcome string            `json:"outcome"`
	Codes   []string          `json:"codes"`
	Items   []json.RawMessage `json:"items"`
}

// runBinary executes one local binary with bounded output and a timeout via
// internal/procgroup.Run, and never treats a non-zero exit or a boundary
// failure as empty success: the caller gets stdout, stderr and an error, and
// must decide from those. It is used for both `atm` (read verbs only) and
// `git` (rev-parse only) — neither is invoked with a shell, and both are
// resolved to an absolute path before exec. env is the child's whole
// environment.
func runBinary(ctx context.Context, binary, dir string, env []string, timeout time.Duration, outputLimit int, args ...string) (stdout, stderr []byte, err error) {
	resolved, lookErr := exec.LookPath(binary)
	if lookErr != nil {
		return nil, nil, fmt.Errorf("%s binary unavailable: %w", binary, lookErr)
	}
	resolved, absErr := filepath.Abs(resolved)
	if absErr != nil {
		return nil, nil, absErr
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if outputLimit <= 0 {
		outputLimit = maxEnvelopeBytes
	}
	argv := append([]string{resolved}, args...)
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: argv, Dir: dir, Env: env, Timeout: timeout,
		OutputLimit: outputLimit, StderrLimit: 64 << 10,
	})
	switch {
	case observation.OutputOverflow || observation.StdoutOverflow || observation.StderrOverflow:
		return observation.Stdout, observation.Stderr, fmt.Errorf("%s output exceeded its collection bound", binary)
	case observation.TimedOut:
		return observation.Stdout, observation.Stderr, fmt.Errorf("%s timed out", binary)
	case observation.Cancelled || ctx.Err() != nil:
		return observation.Stdout, observation.Stderr, fmt.Errorf("%s run was cancelled", binary)
	case !observation.Started || !observation.ExitObserved:
		return observation.Stdout, observation.Stderr, fmt.Errorf("%s did not run to completion: %v", binary, observation.Err)
	case observation.ExitStatus != 0:
		return observation.Stdout, observation.Stderr, fmt.Errorf("%s exited with status %d", binary, observation.ExitStatus)
	}
	return observation.Stdout, observation.Stderr, nil
}

// runAtm executes one read-only `atm` verb rooted at storeRoot and decodes
// its taskman-command-result/0 envelope. Only "roadmap" and "ticket show"
// are ever passed by this package (both listed read verbs; see `atm help`).
// A boundary failure, an unparseable reply, or a reply on the wrong profile
// is an error — never an empty successful envelope.
func runAtm(ctx context.Context, binary, storeRoot string, timeout time.Duration, args ...string) (*envelope, error) {
	stdout, stderr, runErr := runBinary(ctx, binary, storeRoot, os.Environ(), timeout, maxEnvelopeBytes, args...)
	// A refusal may arrive on stderr only with a non-zero exit; a zero exit
	// with empty stdout produced no reply (LOD-V0-030, as LAC-V0-022).
	raw := stdout
	if len(bytes.TrimSpace(raw)) == 0 && runErr != nil {
		raw = stderr
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		if runErr != nil {
			return nil, runErr
		}
		return nil, errors.New("atm produced no output")
	}
	var decoded envelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, fmt.Errorf("atm did not reply with a %s envelope: %w", envelopeProfile, err)
	}
	if decoded.Profile != envelopeProfile {
		return nil, fmt.Errorf("atm replied with profile %q, want %q", decoded.Profile, envelopeProfile)
	}
	// Only a refusal is admitted alongside a failed run; an OK envelope with
	// a non-zero exit or a signal is not an observation.
	if runErr != nil && decoded.Outcome == "OK" {
		return nil, runErr
	}
	return &decoded, nil
}

// CurrentTreeDigest reports the current HEAD tree object id for root via
// `git rev-parse HEAD^{tree}`, used to classify receipts as CURRENT or
// STALE. It reports the last committed tree, not uncommitted worktree
// edits — a receipt measured against a dirty tree is out of this join's
// scope and is a documented gap (see the IPR-10 dashboard evidence note).
func CurrentTreeDigest(ctx context.Context, root string, timeout time.Duration) (string, error) {
	stdout, stderr, err := runBinary(ctx, "git", root, sanitizedGitEnvironment(), timeout, 4096, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", err
	}
	digest := bytes.TrimSpace(stdout)
	if len(digest) == 0 {
		digest = bytes.TrimSpace(stderr)
	}
	if len(digest) == 0 {
		return "", errors.New("git rev-parse returned no tree digest")
	}
	return string(digest), nil
}

// WorktreeDirty reports whether root's worktree has any change not reflected
// in the tree CurrentTreeDigest measured, via
// `git status --porcelain=v1 --untracked-files=all --ignored=no` (no
// filters, no hooks). Non-empty output means dirty. A boundary failure is
// itself missing evidence (AGENTS.md invariant 2): it is reported dirty=true
// alongside the error so a caller that only checks the bool still refuses to
// render an unverifiable tree as clean.
func WorktreeDirty(ctx context.Context, root string, timeout time.Duration) (bool, error) {
	stdout, _, err := runBinary(ctx, "git", root, sanitizedGitEnvironment(), timeout, maxEnvelopeBytes, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=no")
	if err != nil {
		return true, err
	}
	return len(bytes.TrimSpace(stdout)) > 0, nil
}

// sanitizedGitEnvironment mirrors internal/touchsurprise's Git environment:
// no inherited GIT_* variable, user or system configuration, prompt, or
// replace object reaches the tree checks (LOD-V0-032).
func sanitizedGitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}
