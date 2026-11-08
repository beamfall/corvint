// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/groupreap"
)

type boundedBuffer struct {
	data  []byte
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-len(b.data) {
		return 0, fmt.Errorf("capture-cap-exceeded")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

// repositorySnapshot includes content, so a modification to an already dirty
// file is detected even if its porcelain status has not changed. A cap or an
// unreadable/special file makes the session unavailable, never apparently clean.
func repositorySnapshot(ctx context.Context, root string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	git, err = filepath.Abs(git)
	if err != nil {
		return "", err
	}
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, git, args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + filepath.Dir(git), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}
		groupreap.Contain(cmd)
		out := &boundedBuffer{limit: limit}
		cmd.Stdout = out
		cmd.Stderr = &boundedBuffer{limit: 64 << 10}
		cmd.WaitDelay = 2 * time.Second
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		err := groupreap.Wait(cmd)
		return out.data, err
	}
	status, err := gitstatus.Status(ctx, root, 1<<20, run, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write(status)
	var bytes int64
	entries := 0
	visit := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		entries++
		if entries > 100000 {
			return fmt.Errorf("repository-entry-cap")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%d:%s:%v\x00", len(rel), rel, info.Mode())
		if d.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(path)
			if e != nil {
				return e
			}
			fmt.Fprintf(hash, "%d:%s\x00", len(target), target)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("repository-special-file")
		}
		bytes += info.Size()
		if bytes > 1<<30 {
			return fmt.Errorf("repository-byte-cap")
		}
		f, e := openRegular(path)
		if e != nil {
			return e
		}
		defer f.Close()
		fileHash := sha256.New()
		n, e := io.Copy(fileHash, io.LimitReader(f, info.Size()+1))
		if e != nil {
			return e
		}
		if n != info.Size() {
			return fmt.Errorf("repository-file-raced")
		}
		hash.Write(fileHash.Sum(nil))
		return nil
	}
	if err = filepath.WalkDir(root, visit); err != nil {
		return "", err
	}
	// Resolve linked-worktree metadata without executing Git. Hash only config
	// and hooks, sharing the worktree's bounds; objects and packs are excluded.
	gitdir, common, err := gitstatus.Directories(root)
	if err != nil {
		return "", err
	}
	for _, path := range []string{filepath.Join(common, "config"), filepath.Join(gitdir, "config.worktree"), filepath.Join(common, "hooks")} {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		if err := filepath.WalkDir(path, visit); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ReadProjectPins reads a tracked immutable blob through the caller's existing
// trusted Git reader. The host never chooses a CI base from candidate input.
// The reader must return an error wrapping fs.ErrNotExist only for a proven
// absent path at that revision; all other read failures remain FULL.
func ReadProjectPins(readBlob func(string, string) ([]byte, error), trustedBase, head string, ci bool) DiscoveryPlan {
	invalid := func(reason string) DiscoveryPlan { return DiscoveryPlan{FullAll: reason} }
	if readBlob == nil || strings.Trim(trustedBase, "0123456789abcdef") != "" || len(trustedBase) != 40 && len(trustedBase) != 64 {
		return invalid("trusted-base-unavailable")
	}
	readPins := func(revision string) ([]byte, error) {
		data, err := readBlob(revision, ".corvint/runner-plugins.json")
		if errors.Is(err, fs.ErrNotExist) {
			return []byte(`{"schema":"corvint-runner-plugins/1","plugins":[]}`), nil
		}
		return data, err
	}
	base, err := readPins(trustedBase)
	if err != nil {
		return invalid("trusted-pins-unavailable")
	}
	candidate := base
	if ci {
		if strings.Trim(head, "0123456789abcdef") != "" || len(head) != 40 && len(head) != 64 {
			return invalid("candidate-revision-unavailable")
		}
		candidate, err = readPins(head)
		if err != nil {
			return invalid("candidate-pins-unavailable")
		}
	}
	return PlanDiscovery(base, candidate, ci)
}
