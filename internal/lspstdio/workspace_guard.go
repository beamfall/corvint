// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unicode/utf8"
)

const workspaceObservationBudget = 200 * time.Millisecond
const workspaceAdmissionBudget = 5 * time.Second

var errWorkspaceObservation = errors.New("workspace observation unavailable")

type workspaceEntry struct {
	Path     string
	Mode     uint32
	Size     int64
	Modified int64
	Digest   string
}
type workspaceStamp struct {
	repository workspaceGitIdentity
	digest     string
	identities map[string]os.FileInfo
}
type workspaceGuard struct {
	baseline workspaceStamp
	stale    bool
}

// The whole root is observed, not a dependency closure. External SDK/cache inputs remain unbound.
func observeWorkspace(ctx context.Context, root string) (workspaceStamp, error) {
	return observeWorkspaceUsing(ctx, root, observeWorkspaceGit)
}
func observeWorkspaceUsing(ctx context.Context, root string, identity func(context.Context, string) (workspaceGitIdentity, error)) (workspaceStamp, error) {
	before, err := identity(ctx, root)
	if err != nil {
		return workspaceStamp{}, err
	}
	admin := filepath.Join(root, ".git")
	adminInfo := before.gitEntry
	adminPath := before.adminPath
	var entries []workspaceEntry
	var total int64
	identities := map[string]os.FileInfo{}
	inventory := func(hash bool) ([]workspaceEntry, error) {
		var out []workspaceEntry
		err := boundedWorkspaceWalk(ctx, root, 100000, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			if rel == "." {
				return nil
			}
			if !utf8.ValidString(rel) || len(out) >= 100000 {
				return errWorkspaceObservation
			}
			depth := 0
			for _, c := range rel {
				if c == os.PathSeparator {
					depth++
				}
			}
			if depth >= 64 {
				return errWorkspaceObservation
			}
			if path == admin && adminInfo.IsDir() {
				return filepath.SkipDir
			}
			if path != admin && d.Name() == ".git" {
				return errWorkspaceObservation
			}
			info, e := os.Lstat(path)
			if e != nil {
				return e
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return errWorkspaceObservation
			}
			if hash {
				identities[rel] = info
			} else if original, ok := identities[rel]; !ok || !os.SameFile(original, info) {
				return errWorkspaceObservation
			}
			row := workspaceEntry{Path: rel, Mode: uint32(info.Mode()), Size: info.Size(), Modified: info.ModTime().UnixNano()}
			if info.Mode().IsRegular() && hash {
				if info.Size() > 16<<20 || info.Size() < 0 || total+info.Size() > 256<<20 {
					return errWorkspaceObservation
				}
				total += info.Size()
				f, e := openWorkspaceFile(path)
				if e != nil {
					return e
				}
				opened, e := f.Stat()
				if e != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
					f.Close()
					return errWorkspaceObservation
				}
				h := sha256.New()
				buffer := make([]byte, 64<<10)
				var read int64
				for {
					if ctx.Err() != nil {
						f.Close()
						return ctx.Err()
					}
					n, e := f.Read(buffer)
					read += int64(n)
					if read > info.Size() {
						f.Close()
						return errWorkspaceObservation
					}
					h.Write(buffer[:n])
					if e == io.EOF {
						break
					}
					if e != nil {
						f.Close()
						return e
					}
				}
				last, e := f.Stat()
				f.Close()
				current, statErr := os.Lstat(path)
				if e != nil || statErr != nil || read != info.Size() || !os.SameFile(info, last) || !os.SameFile(info, current) || last.Size() != info.Size() || last.ModTime() != info.ModTime() || current.ModTime() != info.ModTime() {
					return errWorkspaceObservation
				}
				row.Digest = hex.EncodeToString(h.Sum(nil))
			}
			out = append(out, row)
			return nil
		})
		sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
		return out, err
	}
	entries, err = inventory(true)
	if err != nil {
		return workspaceStamp{}, err
	}
	again, err := inventory(false)
	if err != nil || len(entries) != len(again) {
		return workspaceStamp{}, errWorkspaceObservation
	}
	for i, e := range entries {
		a := again[i]
		e.Digest = ""
		if e != a {
			return workspaceStamp{}, errWorkspaceObservation
		}
	}
	after, err := identity(ctx, root)
	if err != nil || !sameWorkspaceGit(before, after) {
		return workspaceStamp{}, errWorkspaceObservation
	}
	if ctx.Err() != nil {
		return workspaceStamp{}, ctx.Err()
	}
	raw, _ := json.Marshal(struct {
		Admin   string
		Entries []workspaceEntry
	}{adminPath, entries})
	sum := sha256.Sum256(raw)
	return workspaceStamp{repository: before, digest: hex.EncodeToString(sum[:]), identities: identities}, nil
}
func boundedGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	return boundedGitOutput(ctx, root, 16384, args...)
}
func boundedGitOutput(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
	executable, err := gitstatus.Pin()
	if err != nil {
		return nil, errWorkspaceObservation
	}
	argv := append([]string{executable, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", root}, args...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: gokernel.SanitizedGitEnvironment(), Timeout: workspaceAdmissionBudget, ShutdownTimeout: contextShutdownBudget, OutputLimit: limit, StderrLimit: 4096})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if o.Err != nil || o.ExitStatus != 0 || !o.WaitCompleted || !o.PipesDrained || !o.OwnedProcessGroupCleanup || o.DescendantCleanupQualification != "" || o.OutputOverflow {
		return nil, errWorkspaceObservation
	}
	return o.Stdout, nil
}
func (g *workspaceGuard) check(ctx context.Context, root string, remaining *time.Duration) error {
	if g.stale || *remaining <= 0 {
		g.stale = true
		return errWorkspaceObservation
	}
	started := time.Now()
	limited, stop := context.WithTimeout(ctx, *remaining)
	defer stop()
	stamp, err := observeWorkspace(limited, root)
	*remaining -= time.Since(started)
	if err != nil || *remaining <= 0 || stamp.digest != g.baseline.digest || !sameWorkspaceEntries(stamp.identities, g.baseline.identities) || !sameWorkspaceGit(stamp.repository, g.baseline.repository) {
		g.stale = true
		return errWorkspaceObservation
	}
	return nil
}

func sameWorkspaceEntries(a, b map[string]os.FileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for path, info := range a {
		other, ok := b[path]
		if !ok || !os.SameFile(info, other) {
			return false
		}
	}
	return true
}
