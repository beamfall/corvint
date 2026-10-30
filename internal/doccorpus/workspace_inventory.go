// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// WorkspaceRepository is one further checkout a manifest inventories beside the primary root: a
// workspace (WSD-V0) alias, its absolute checkout root, the revision expression to pin there
// (empty pins HEAD) and the scopes inventoried in it (DCP-V1-600).
type WorkspaceRepository struct {
	Alias    string
	Root     string
	Revision string
	Scopes   []string
}

// InventoryWorkspace inventories the primary root as InventoryScopes does, then each further
// repository at its own pinned commit as a DCP-V1-175 additional repository carrying its alias,
// with its scopes and native inputs pinned at that commit (DCP-V1-600). Without further
// repositories the manifest is InventoryScopes' own, byte for byte (DCP-V1-603).
func InventoryWorkspace(ctx context.Context, root, revision string, scopes []string, schema, timestamp string, repos []WorkspaceRepository) (Manifest, error) {
	m, refused, err := inventoryScopes(ctx, root, revision, scopes, schema, timestamp, "primary")
	if err != nil {
		return m, err
	}
	if len(repos) == 0 {
		if len(refused) > 0 {
			return m, generatedRefusal(refused)
		}
		return m, nil
	}
	if len(repos) > maxAdditionalRepositories {
		return Manifest{}, capacityExceeded("additional repository count", len(repos), maxAdditionalRepositories)
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fail("workspace root is unusable")
	}
	for _, r := range repos {
		if !rootalias.Valid(r.Alias) || len(r.Scopes) == 0 {
			return Manifest{}, fail("workspace repository " + r.Alias + " needs a valid alias and at least one scope")
		}
		pinned, err := resolveWorkspaceRevision(ctx, r)
		if err != nil {
			return Manifest{}, err
		}
		sub, subRefused, err := inventoryScopes(ctx, r.Root, pinned, r.Scopes, schema, timestamp, r.Alias)
		if err != nil {
			return Manifest{}, err
		}
		refused = append(refused, subRefused...)
		rel, err := filepath.Rel(base, r.Root)
		if err != nil || !validRepositoryRoot(filepath.ToSlash(rel)) {
			return Manifest{}, fail("workspace repository " + r.Alias + " root is not a separate checkout")
		}
		m.AdditionalRepositories = append(m.AdditionalRepositories, AdditionalRepository{ID: sub.Repository.ID, Revision: pinned, Root: filepath.ToSlash(rel), Alias: r.Alias})
		m.Scopes = append(m.Scopes, sub.Scopes...)
		m.Inputs = append(m.Inputs, sub.Inputs...)
		m.OverBoundInputs = append(m.OverBoundInputs, sub.OverBoundInputs...)
	}
	sort.Slice(m.AdditionalRepositories, func(i, j int) bool { return m.AdditionalRepositories[i].ID < m.AdditionalRepositories[j].ID })
	// The declaration is checked as a build checks it, so a shared repository identity or
	// revision refuses here rather than at build (DCP-V1-175).
	if err := validateAdditionalRepositories(m); err != nil {
		return Manifest{}, err
	}
	// Every generated input of every repository is named in one refusal, which still returns the
	// pinned manifest (DCP-V1-608).
	if len(refused) > 0 {
		return m, generatedRefusal(refused)
	}
	return m, nil
}

// resolveWorkspaceRevision pins the repository's declared revision, or HEAD, to its full commit.
func resolveWorkspaceRevision(ctx context.Context, r WorkspaceRepository) (string, error) {
	auth, err := gitauth.Open(r.Root, newGitBudget(ctx))
	if err != nil {
		return "", checkoutUnavailable("workspace repository "+r.Alias+" checkout unavailable", err)
	}
	revision := r.Revision
	if revision == "" {
		revision = "HEAD"
	}
	pinned, err := auth.Resolve(ctx, revision)
	if err != nil {
		return "", &Error{Code: "corpus-input-unavailable", Message: "workspace repository " + r.Alias + " revision " + revision + " does not resolve to a commit"}
	}
	return pinned, nil
}
