// SPDX-License-Identifier: Apache-2.0
package main

// The process, Git and receipt plumbing below is copied from
// tools/affected-edit-oracle (AEO-V0-002) so both tools contain, neutralise and
// bound commands the same way; the parity test of FIH-V1-011 runs both on one
// input. Only the identities (author name, profile) differ.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const outputLimit = 32 << 20

// capture is a bounded synchronized sink shared by stdout and stderr.
type capture struct {
	mu       sync.Mutex
	body     []byte
	overflow bool
}

func (b *capture) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	space := outputLimit - len(b.body)
	if n > space {
		b.overflow = true
		p = p[:space]
	}
	b.body = append(b.body, p...)
	return n, nil
}

// errWallTimeout marks a command stopped by the harness's own wall bound. It
// is a failed measurement, never a fault-induced crash (G3): a faulted run
// that hits it is an ERROR, not a dropped case.
var errWallTimeout = errors.New("harness wall timeout")

// command runs argv in root under a process group with a wall-clock bound.
// A timeout, an unproved cleanup or an output overflow is an error, never a
// result (FIH-V1-005: a timed-out run is a failed measurement).
func command(ctx context.Context, root string, timeout time.Duration, argv ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = root
	// Ambient Git pointers must not redirect a clone operation to the source index.
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if strings.HasPrefix(key, "GIT_") || slices.Contains([]string{"GOFLAGS", "GOPROXY", "GOWORK", "GOTOOLCHAIN", "LC_ALL", "HEAVY_REMOTE", "HEAVY_LOCAL_REASON", "HEAVY_REMOTE_HOST", "HEAVY_REMOTE_HOSTS", "HEAVY_NICE", "HEAVY_SELECTION", "HEAVY_SELECTION_REASON"}, key) {
			continue
		}
		c.Env = append(c.Env, v)
	}
	c.Env = append(c.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_DATE=2026-11-08T00:00:00Z", "GIT_COMMITTER_DATE=2026-11-08T00:00:00Z", "LC_ALL=C", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "HEAVY_REMOTE=0", "HEAVY_LOCAL_REASON=owner forbids network writes; local fault-injection measurement", "HEAVY_REMOTE_HOST=", "HEAVY_REMOTE_HOSTS=", "HEAVY_NICE=0", "HEAVY_SELECTION=manual", "HEAVY_SELECTION_REASON=fault-injection harness runs a pre-registered test set")
	var out capture
	c.Stdout = &out
	c.Stderr = &out
	c.WaitDelay = 5 * time.Second
	owner, err := groupreap.Start(c)
	if err != nil {
		return nil, err
	}
	select {
	case <-owner.Exited():
	case <-ctx.Done():
		owner.Stop()
	}
	retirement, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	result := owner.Finish(retirement.Done())
	if err := stopError(parent, ctx, result, argv[0], timeout); err != nil {
		return out.body, err
	}
	if out.overflow {
		return nil, errors.New("command output exceeded 32 MiB")
	}
	return out.body, result.WaitErr
}

// errCleanupUnproved marks a command whose process group was not proved
// released. Like a wall timeout it says nothing about a fault: it is a failed
// measurement, never a dropped case (G3).
var errCleanupUnproved = errors.New("process cleanup unproved")

// stopError classifies how a command ended before its output is read: an
// unproved cleanup, a cancelled parent or the harness's wall bound. An
// unproved cleanup of a command that also hit the wall bound keeps both marks,
// so no caller can read it as a fault-induced crash.
func stopError(parent, bounded context.Context, result groupreap.Result, name string, timeout time.Duration) error {
	var stop error
	switch {
	case bounded.Err() == nil:
	case parent.Err() != nil:
		stop = parent.Err()
	default:
		stop = fmt.Errorf("%w: %s after %s", errWallTimeout, name, timeout)
	}
	if result.State != groupreap.Released {
		return errors.Join(fmt.Errorf("%w: state %s: %v", errCleanupUnproved, result.State, result.Err), stop)
	}
	return stop
}

// stopped reports whether err, or the context, says the harness stopped a
// command rather than the command failing on its own: cancellation, the wall
// bound or an unproved cleanup. Such an error is a failed measurement.
func stopped(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, errWallTimeout) || errors.Is(err, errCleanupUnproved)
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	out, err := gitRaw(ctx, root, args...)
	return strings.TrimSpace(string(out)), err
}

// gitRaw returns Git's output byte for byte, for file bodies and patches.
func gitRaw(ctx context.Context, root string, args ...string) ([]byte, error) {
	argv := append([]string{"git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Fault Harness", "-c", "user.email=fault-harness@invalid"}, args...)
	out, err := command(ctx, root, 2*time.Minute, argv...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

// commitAll commits the faulted tree and returns the committed paths.
func commitAll(ctx context.Context, root, message string) ([]string, error) {
	if _, err := git(ctx, root, "add", "--all"); err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "commit", "--quiet", "--allow-empty", "-m", message); err != nil {
		return nil, err
	}
	paths, err := git(ctx, root, "diff", "--name-only", "HEAD~1", "HEAD")
	if err != nil {
		return nil, err
	}
	if paths == "" {
		return nil, nil
	}
	return strings.Split(paths, "\n"), nil
}

type witness struct {
	Kind      string `json:"kind"`
	DirtyPath string `json:"dirtyPath"`
	ViaLength int    `json:"viaLength"`
}
type selection struct {
	Kind    string   `json:"kind,omitempty"`
	UnitID  string   `json:"unitId"`
	Run     []string `json:"run,omitempty"`
	Witness witness  `json:"witness"`
}
type receipt struct {
	OK   bool `json:"ok"`
	Plan *struct {
		Scope    string            `json:"scope"`
		Tier     string            `json:"tier"`
		Selected []selection       `json:"selected"`
		Unknown  []json.RawMessage `json:"unknown"`
	} `json:"plan"`
	Summary struct {
		Line string `json:"line"`
	} `json:"summary"`
}

// plan runs the planner against base on the committed faulted tree.
func plan(ctx context.Context, root, bin, base string) (receipt, error) {
	// The harness commits its fault, so a binary built before it is stale by
	// AFP-V0-846; the plan is what the harness measures, so it is accepted.
	out, err := command(ctx, root, 2*time.Minute, bin, "affected", "--base", base, "--format", "json", "--accept-stale-binary")
	if err != nil {
		return receipt{}, fmt.Errorf("affected: %w: %s", err, out)
	}
	var p receipt
	if err = json.Unmarshal(out, &p); err != nil {
		return p, fmt.Errorf("decode affected: %w", err)
	}
	if !p.OK || p.Plan == nil || p.Plan.Selected == nil || p.Plan.Unknown == nil || p.Summary.Line == "" {
		return p, errors.New("incomplete affected receipt")
	}
	for _, s := range p.Plan.Selected {
		if s.UnitID == "" || s.Witness.Kind == "" {
			return p, errors.New("selection missing identity/witness")
		}
	}
	return p, nil
}

func sha(body []byte) string { h := sha256.Sum256(body); return hex.EncodeToString(h[:]) }

func boundedRead(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: input too large", path)
	}
	return b, e
}

// decodeStrict decodes exactly one JSON value with no unknown fields.
func decodeStrict(body []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}
