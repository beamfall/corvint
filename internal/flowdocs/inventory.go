package flowdocs

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

type OpaqueGitlink struct {
	Path   string `json:"path"`
	Commit string `json:"commit"`
}

func inScope(name, scope string) bool { return name == scope || strings.HasPrefix(name, scope+"/") }
func opaqueGitlinks(ctx context.Context, auth *gitauth.Repository, index *contextindex.Index, revision, scope string) ([]OpaqueGitlink, error) {
	result := []OpaqueGitlink{}
	for name := range index.Skipped {
		if !inScope(name, scope) {
			continue
		}
		entry, ok, err := auth.LookupTreeEntry(ctx, revision, name)
		if err != nil {
			return nil, err
		}
		if !ok || entry.Type != "commit" || entry.Mode != "160000" {
			return nil, fail("scope includes unsupported non-gitlink tree entry")
		}
		result = append(result, OpaqueGitlink{Path: name, Commit: entry.OID})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

// sourceInventory enumerates every regular superproject input as an explicit
// file scope. Gitlinks remain recorded provider uncertainty, not missing blobs
// or recursively admitted contents. The generic corpus inventory stays strict.
func sourceInventory(ctx context.Context, root string, m Manifest) (doccorpus.Manifest, error) {
	inventory := doccorpus.Manifest{Schema: doccorpus.ManifestSchemaV2, Repository: m.Source, BuiltAt: "2000-01-01T00:00:00Z", Profile: doccorpus.Profile{ID: "native", Revision: "1", Title: "Flow source evidence; gitlinks outside coverage", Format: "markdown", Groups: []string{}}, Scopes: []doccorpus.Scope{}, Inputs: []doccorpus.Input{}, Providers: []doccorpus.Provider{{ID: "native", Kind: "native", Version: "1", Revision: m.Source.Revision}}, MergeRule: "none"}
	index, err := contextindex.BuildRevisionContext(ctx, root, m.Source.Revision)
	if err != nil {
		return inventory, err
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return inventory, err
	}
	release := auth.BeginObjectSession()
	defer release()
	links, err := opaqueGitlinks(ctx, auth, index, m.Source.Revision, m.Scope)
	if err != nil {
		return inventory, err
	}
	a, _ := Encode(links)
	b, _ := Encode(m.OpaqueGitlinks)
	if string(a) != string(b) {
		return inventory, fail("opaque gitlink inventory differs from generation")
	}
	names := []string{}
	for name := range index.Tracked {
		if inScope(name, m.Scope) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 || len(names) > doccorpus.MaxRecords {
		return inventory, fail("empty or oversized source inventory")
	}
	total := 0
	for _, name := range names {
		entry, ok, err := auth.LookupTreeEntry(ctx, m.Source.Revision, name)
		if err != nil {
			return inventory, err
		}
		if !ok || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
			return inventory, fail("source input is not a regular Git blob")
		}
		raw, err := auth.BlobBytes(ctx, entry.OID)
		if err != nil {
			return inventory, err
		}
		total += len(raw)
		if len(raw) > doccorpus.MaxBytes || total > doccorpus.MaxCorpusBytes {
			return inventory, fail("source inventory exceeds byte bound")
		}
		inventory.Scopes = append(inventory.Scopes, doccorpus.Scope{Path: name, Revision: m.Source.Revision})
		inventory.Inputs = append(inventory.Inputs, doccorpus.Input{Path: name, Revision: m.Source.Revision, Blob: entry.OID, SHA256: doccorpus.Digest(raw), Provider: "native", Purpose: "source"})
	}
	return inventory, nil
}
