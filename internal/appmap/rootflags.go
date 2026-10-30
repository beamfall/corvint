// SPDX-License-Identifier: Apache-2.0

package appmap

import (
	"errors"
	"flag"
	"sort"

	"github.com/Beamfall/corvint/internal/rootalias"
	"github.com/Beamfall/corvint/internal/workspace"
)

// BindRootFlags registers the aliased-root flags shared by every verb that compiles a map:
// --repo ALIAS=ABSOLUTE_ROOT (repeatable; the multi-root MCP spelling and alias rules,
// MMR-V0-001, MMR-V0-002), --manifest-repo ALIAS (AMAP-V0-017) and --repo-revision ALIAS=REV
// (repeatable, AMAP-V0-200), and --workspace FILE / --no-workspace (WSD-V0-004). Call Resolve
// after parsing.
func BindRootFlags(f *flag.FlagSet) *Roots {
	roots := &Roots{Repos: map[string]string{}}
	f.Func("repo", "ALIAS=ABSOLUTE_ROOT (repeatable)", func(v string) error {
		alias, path, aliased := rootalias.Split(v)
		if !aliased || path == "" || roots.Repos[alias] != "" || len(roots.Repos) == rootalias.MaxRoots {
			return errors.New("invalid --repo")
		}
		roots.Repos[alias] = path
		return nil
	})
	f.Func("manifest-repo", "alias of the root holding the manifest", func(v string) error {
		if !rootalias.Valid(v) || roots.ManifestRepo != "" {
			return errors.New("invalid --manifest-repo")
		}
		roots.ManifestRepo = v
		return nil
	})
	f.Func("repo-revision", "ALIAS=REV read in place of that root's HEAD (repeatable)", func(v string) error {
		alias, rev, aliased := rootalias.Split(v)
		if !aliased || rev == "" || roots.Revisions[alias] != "" {
			return errors.New("invalid --repo-revision")
		}
		if roots.Revisions == nil {
			roots.Revisions = map[string]string{}
		}
		roots.Revisions[alias] = rev
		return nil
	})
	f.Func("workspace", "FILE: a workspace declaration (WSD-V0) of the aliased roots", func(v string) error {
		if v == "" || roots.workspaceFlag != "" {
			return errors.New("invalid --workspace")
		}
		roots.workspaceFlag = v
		return nil
	})
	f.BoolVar(&roots.noWorkspace, "no-workspace", false, "do not discover "+workspace.DiscoveryPath+" under the root")
	return roots
}

// Resolve checks the parsed flags and binds the workspace (WSD-V0-003, WSD-V0-004): the file
// --workspace names, or else, when no aliased-root flag and no --no-workspace is given, the one
// discovered at root/.corvint/workspace.json. A workspace never merges with --repo,
// --manifest-repo or --repo-revision: naming both is an argument error.
func (r *Roots) Resolve(root string) error {
	if err := r.Check(); err != nil {
		return err
	}
	flagged := len(r.Repos) > 0 || r.ManifestRepo != "" || len(r.Revisions) > 0
	var w *workspace.Workspace
	var err error
	switch {
	case r.workspaceFlag != "" && (flagged || r.noWorkspace):
		return errors.New("--workspace does not combine with --repo, --manifest-repo, --repo-revision or --no-workspace")
	case r.workspaceFlag != "":
		w, err = workspace.Load(r.workspaceFlag)
	case flagged || r.noWorkspace:
		return nil
	default:
		w, err = workspace.Discover(root)
	}
	if err != nil || w == nil {
		return err
	}
	r.Repos, r.Revisions, r.ManifestRepo, r.Workspace = map[string]string{}, nil, w.ManifestRepo, w.File
	for alias, repo := range w.Repos {
		r.Repos[alias] = repo.Root
		if repo.Revision != "" {
			if r.Revisions == nil {
				r.Revisions = map[string]string{}
			}
			r.Revisions[alias] = repo.Revision
		}
	}
	return nil
}

// Aliases returns the bound aliases in order.
func (r *Roots) Aliases() []string {
	out := make([]string, 0, len(r.Repos))
	for alias := range r.Repos {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

// Check reports whether every --repo-revision names a declared --repo alias; an undeclared
// --manifest-repo or manifest tests.repo alias is refused by BuildRoots as appmap-root-unavailable.
func (r *Roots) Check() error {
	for alias := range r.Revisions {
		if r.Repos[alias] == "" {
			return errors.New("--repo-revision names an alias without --repo")
		}
	}
	return nil
}

// Aliased reports whether any aliased root is bound.
func (r *Roots) Aliased() bool { return len(r.Repos) > 0 }
