// SPDX-License-Identifier: Apache-2.0

// Package lineevidence reads the private, additive line-evidence store (LEV-V1).
// Qualification is deliberately closed: no FIH record has been admitted.
package lineevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

type Tier string

const (
	Line        Tier = "line"
	File        Tier = "file"
	Flow        Tier = affected.TierFlow
	Full        Tier = "full"
	Absent           = "line-evidence-absent"
	Stale            = "line-evidence-stale"
	Damaged          = "index-section-invalid"
	Unqualified      = "line-evidence-unqualified"
	MaxBytes         = 64 << 20
)

// Verdict cannot authorize removal. Rank is advisory and contains canonical IDs.
type Verdict struct {
	Instance string   `json:"instance"`
	Tier     Tier     `json:"tier"`
	Exact    bool     `json:"exact"`
	Reason   string   `json:"reason"`
	Rank     []string `json:"rank,omitempty"`
}

func (v Verdict) String() string {
	return fmt.Sprintf("%s tier=%s exact=%t %s", v.Instance, v.Tier, v.Exact, v.Reason)
}

// Filename includes the root as well as the runner: two instances never share a file.
func Filename(d testinventory.Declared) string {
	return fmt.Sprintf("%s@%x.linecov", d.Runner, sha256.Sum256([]byte(d.Root)))
}

// Binding binds instance, command, config and lockfile without changing index/4.
// Native runner identity is supplied separately by collection and freshness callers.
func Binding(d testinventory.Declared, files map[string]string, command []string, native string) string {
	values := []string{d.Key, d.Root, d.Config, files[d.Config], d.Lockfile, files[d.Lockfile], native}
	values = append(values, command...)
	return fmt.Sprintf("line-evidence/1:%x", sha256.Sum256([]byte(strings.Join(values, "\x00"))))
}

func Store(ctx context.Context, git testinventory.Git) (string, error) {
	b, err := git.Output(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(string(b)), "corvint", "line-evidence"), nil
}

// Input names the planned immutable base and working-tree files. Changed includes
// staged, unstaged and untracked paths. Ancestor errors are never evidence of ancestry.
type Input struct {
	// Path overrides the store path for watch --index; it grants no authority.
	Path                    string
	Store                   string
	Instance                testinventory.Declared
	Runner                  string
	Command                 []string
	BaseFiles, CurrentFiles map[string]string
	Changed                 []string
	Ancestor                func(commit string) bool
}

func Read(in Input) Verdict {
	v := Verdict{Instance: in.Instance.Key, Tier: File, Reason: Absent}
	name := in.Path
	if name == "" {
		name = filepath.Join(in.Store, Filename(in.Instance))
	}
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return v
	}
	v.Reason = Damaged
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return v
	}
	f, err := os.Open(name)
	if err != nil {
		return v
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil || len(data) > MaxBytes || !bytes.HasPrefix(data, []byte(linecov.Format+"\n")) {
		return v
	}
	ix, err := linecov.Decode(bytes.NewReader(data))
	if err != nil {
		return v
	}
	defer ix.Close()
	d := in.Instance
	if ix.Language != language(d.Runner) || ix.RunnerFamily != d.Runner || ix.Config != d.Config || ix.Runner != in.Runner ||
		!slices.Equal(ix.Command, in.Command) || in.Ancestor == nil || !in.Ancestor(ix.Commit) {
		v.Tier, v.Reason = Full, Stale
		return v
	}
	v.Reason = Unqualified
	for _, changed := range in.Changed {
		if d.Root != "." && !strings.HasPrefix(changed, d.Root+"/") {
			continue
		}
		rel := changed
		if d.Root != "." {
			rel = strings.TrimPrefix(changed, d.Root+"/")
		}
		file, ok := ix.FileByPath(rel)
		if !ok || in.BaseFiles[changed] == "" || file.OID != in.BaseFiles[changed] || in.CurrentFiles[changed] == "" {
			v.Reason = Stale
			continue
		}
		for _, test := range ix.TestsOfFile(rel) {
			id, err := testid.Parse(test.Name)
			if err != nil || id.Runner != d.Runner {
				continue
			}
			// File-level evidence is safe only within this runner instance.
			if d.Root != "." && id.File != d.Root && !strings.HasPrefix(id.File, d.Root+"/") {
				continue
			}
			v.Rank = append(v.Rank, id.String())
		}
	}
	slices.Sort(v.Rank)
	v.Rank = slices.Compact(v.Rank)
	return v
}

// FileIDs projects additions to whole files (Go's file position is a package).
func FileIDs(ids []string) []string {
	var out []string
	for _, s := range ids {
		id, err := testid.Parse(s)
		if err != nil {
			continue
		}
		id.Segments = nil
		out = append(out, id.String())
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Relative maps a repository path to an instance path without admitting traversal.
func Relative(root, name string) string {
	if root == "." {
		return name
	}
	return strings.TrimPrefix(name, root+"/")
}

func joined(root, name string) string { return path.Join(root, name) }

func language(runner string) string {
	switch runner {
	case "go":
		return "go"
	case "vitest", "jest", "playwright":
		return "js"
	case "rspec", "minitest":
		return "ruby"
	}
	return ""
}
