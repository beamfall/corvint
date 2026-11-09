// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/rootalias"
	"github.com/Beamfall/corvint/internal/workspace"
)

// corpusWorkspaceFlags are the workspace options every docs corpus operation admits (DCP-V1-604).
var corpusWorkspaceFlags = []string{"--workspace", "--no-workspace", "--repo"}

// corpusWorkspace is the bound workspace of one docs corpus invocation: each further alias's
// checkout and declared revision, keyed by alias. The alias whose checkout is --root is the
// primary and is not listed.
type corpusWorkspace struct {
	roots     map[string]string
	revisions map[string]string
	// primary is the alias declared at --root, if any.
	primary string
}

func (w *corpusWorkspace) aliases() []string {
	out := make([]string, 0, len(w.roots))
	for alias := range w.roots {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

// bindCorpusWorkspace binds the workspace the shared reader declares (WSD-V0-001..004): the file
// --workspace names, else the --repo flags, else, for manifest only and without --no-workspace,
// the declaration discovered under --root. A workspace never combines with --repo or
// --no-workspace. It returns nil when none is bound (DCP-V1-604).
func bindCorpusWorkspace(o corpusOptions) (*corpusWorkspace, error) {
	if o.given["--workspace"] && (len(o.repos) > 0 || o.noWorkspace) {
		return nil, argumentError("--workspace does not combine with --repo or --no-workspace")
	}
	if o.given["--workspace"] && o.workspaceFile == "" {
		return nil, argumentError("invalid --workspace")
	}
	primary, err := canonicalCheckout(o.root)
	if err != nil {
		return nil, err
	}
	bound := &corpusWorkspace{roots: map[string]string{}, revisions: map[string]string{}}
	add := func(alias, root, revision string) error {
		dir, err := canonicalCheckout(root)
		if err != nil {
			return err
		}
		if dir == primary {
			bound.primary = alias
			return nil
		}
		bound.roots[alias], bound.revisions[alias] = dir, revision
		return nil
	}
	if len(o.repos) > 0 {
		if o.noWorkspace {
			return nil, argumentError("--repo does not combine with --no-workspace")
		}
		for _, value := range o.repos {
			alias, root, ok := strings.Cut(value, "=")
			if !ok || !rootalias.Valid(alias) || root == "" {
				return nil, argumentError("--repo must be ALIAS=ROOT with ALIAS matching [a-z][a-z0-9-]{0,31}")
			}
			if _, dup := bound.roots[alias]; dup || alias == bound.primary {
				return nil, argumentError("--repo names alias " + alias + " twice")
			}
			if err := add(alias, root, ""); err != nil {
				return nil, err
			}
		}
		return bound, nil
	}
	var w *workspace.Workspace
	switch {
	case o.given["--workspace"]:
		w, err = workspace.Load(o.workspaceFile)
	case o.noWorkspace || o.op != "manifest":
		return nil, nil
	default:
		w, err = workspace.Discover(o.root)
	}
	if err != nil || w == nil {
		return nil, err
	}
	for _, alias := range w.Aliases() {
		if err := add(alias, w.Repos[alias].Root, w.Repos[alias].Revision); err != nil {
			return nil, err
		}
	}
	if bound.primary == "" {
		return nil, &doccorpus.Error{Code: "corpus-refused", Message: "workspace " + w.File + " declares no alias at --root (" + strings.Join(w.Aliases(), ", ") + "); the corpus is built from --root, so declare it, or pass --no-workspace to inventory --root alone"}
	}
	return bound, nil
}

// canonicalCheckout is root made absolute with its symbolic links resolved, so two spellings of
// one checkout compare equal.
func canonicalCheckout(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", &doccorpus.Error{Code: "corpus-input-unavailable", Message: "repository root is unusable: " + root}
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// workspaceRepositories splits the manifest scopes between --root and each bound alias: a scope
// ALIAS:PATH belongs to that alias (the primary alias's prefix names --root), every other scope
// to --root. An alias with no scope would be ignored, so it refuses naming it (WSD-V0-005).
func workspaceRepositories(o corpusOptions, w *corpusWorkspace) ([]string, []doccorpus.WorkspaceRepository, error) {
	primary := []string{}
	scoped := map[string][]string{}
	for _, scope := range o.scopes {
		alias, rest, ok := strings.Cut(scope, ":")
		switch {
		case ok && alias == w.primary && alias != "":
			primary = append(primary, rest)
		case ok && w.roots[alias] != "":
			scoped[alias] = append(scoped[alias], rest)
		default:
			primary = append(primary, scope)
		}
	}
	repos := []doccorpus.WorkspaceRepository{}
	missing := []string{}
	for _, alias := range w.aliases() {
		if len(scoped[alias]) == 0 {
			missing = append(missing, alias)
			continue
		}
		repos = append(repos, doccorpus.WorkspaceRepository{Alias: alias, Root: w.roots[alias], Revision: w.revisions[alias], Scopes: scoped[alias]})
	}
	if len(missing) > 0 {
		return nil, nil, &doccorpus.Error{Code: "corpus-refused", Message: "workspace aliases " + strings.Join(missing, ", ") + " have no --scope ALIAS:PATH, so the manifest would ignore them; scope each, or pass --no-workspace to inventory --root alone"}
	}
	return primary, repos, nil
}

// corpusWorkspaceManifest inventories --root and every bound alias (DCP-V1-600, DCP-V1-604).
func corpusWorkspaceManifest(ctx context.Context, o corpusOptions, w *corpusWorkspace, schema string) (doccorpus.Manifest, error) {
	scopes, repos, err := workspaceRepositories(o, w)
	if err != nil {
		return doccorpus.Manifest{}, err
	}
	return doccorpus.InventoryWorkspace(ctx, o.root, o.revision, scopes, schema, o.timestamp, repos)
}

// checkCorpusWorkspace refuses a bound workspace whose further aliases the corpus does not declare
// at the same checkout: such an operation would read the corpus and silently ignore the alias
// (WSD-V0-005, DCP-V1-604).
func checkCorpusWorkspace(root string, w *corpusWorkspace, m doccorpus.Manifest) error {
	if w == nil {
		return nil
	}
	declared := map[string]string{}
	for _, r := range m.AdditionalRepositories {
		if r.Alias != "" {
			dir, err := canonicalCheckout(m.RevisionRoot(root, r.Revision))
			if err != nil {
				return err
			}
			declared[r.Alias] = dir
		}
	}
	ignored := []string{}
	for _, alias := range w.aliases() {
		if declared[alias] != w.roots[alias] {
			ignored = append(ignored, alias)
		}
	}
	if len(ignored) > 0 {
		return &doccorpus.Error{Code: "corpus-refused", Message: "the corpus does not declare workspace aliases " + strings.Join(ignored, ", ") + " at their checkouts; rebuild its manifest with the workspace, or omit it"}
	}
	return nil
}

// corpusExtractRoot is the checkout extract reads: --root, or with --repository the bound
// workspace alias's checkout (DCP-V1-604).
func corpusExtractRoot(o corpusOptions, w *corpusWorkspace) (string, error) {
	if o.repository == "" {
		return o.root, nil
	}
	if w != nil && o.repository == w.primary {
		return o.root, nil
	}
	if w == nil || w.roots[o.repository] == "" {
		return "", argumentError("extract --repository must name a bound workspace alias")
	}
	return w.roots[o.repository], nil
}
