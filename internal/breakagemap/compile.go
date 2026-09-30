package breakagemap

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/extevidence"
)

type Anchor struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Tree       string `json:"tree"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	SpanSHA256 string `json:"span_sha256"`
}
type Unknown struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}
type Edge struct {
	Kind           string                 `json:"kind"`
	Confidence     string                 `json:"confidence"`
	Reason         string                 `json:"reason"`
	From           *Anchor                `json:"from,omitempty"`
	To             *Anchor                `json:"to,omitempty"`
	ProviderSHA256 string                 `json:"provider_sha256,omitempty"`
	Relation       *extevidence.Relation1 `json:"relation,omitempty"`
	ByteState      string                 `json:"byte_state"`
}
type RepoState struct {
	Repository
	Head  string `json:"head,omitempty"`
	State string `json:"state"`
}
type Report struct {
	Schema         string      `json:"schema"`
	Mutates        bool        `json:"mutates"`
	Experimental   bool        `json:"experimental"`
	ManifestSHA256 string      `json:"manifest_sha256"`
	API            string      `json:"api"`
	Base           string      `json:"base,omitempty"`
	Change         string      `json:"change"`
	Repositories   []RepoState `json:"repositories"`
	Edges          []Edge      `json:"edges"`
	Unknowns       []Unknown   `json:"unknowns"`
	Scope          string      `json:"scope"`
	Limits         string      `json:"limits"`
	Truncated      bool        `json:"truncated"`
	unknownSeen    map[string]bool
}

func (r *Report) unknown(subject, reason string) {
	if r.unknownSeen == nil {
		r.unknownSeen = map[string]bool{}
	}
	k := subject + "\x00" + reason
	if r.unknownSeen[k] {
		return
	}
	if len(r.Unknowns) == MaxEdges {
		r.Truncated = true
		return
	}
	r.unknownSeen[k] = true
	r.Unknowns = append(r.Unknowns, Unknown{subject, reason})
}
func (r *Report) add(e Edge) {
	if len(r.Edges) == MaxEdges {
		r.Truncated = true
		return
	}
	r.Edges = append(r.Edges, e)
}

// Compile reads only enumerated immutable blobs. Bindings are operator input and
// are deliberately absent from the portable manifest and resulting report.
func Compile(parent context.Context, raw []byte, bindings map[string]string, api, base string) (Report, error) {
	r := Report{Schema: Schema, Experimental: true, ManifestSHA256: digest(raw), API: api, Base: base, Change: "explicit-selection", Edges: []Edge{}, Unknowns: []Unknown{}, Scope: "INCOMPLETE", Limits: "8 repositories; 256 files; 1 MiB/blob; 16 MiB source total; 2000 edges and unknowns each; 30 seconds; explicit paths only"}
	m, err := Decode(raw)
	if err != nil {
		return r, err
	}
	parts := strings.Split(api, ":")
	if len(parts) != 3 || !identifier.MatchString(parts[0]) || !safePath(parts[1]) || parts[2] == "" {
		return r, errors.New("API must be REPOSITORY:PATH:SYMBOL")
	}
	if base != "" && !oid.MatchString(base) {
		return r, errors.New("base must be a full commit id in the API repository")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	repos := map[string]Repository{}
	ready := map[string]bool{}
	for _, repo := range m.Repositories {
		repos[repo.ID] = repo
	}
	for id := range bindings {
		if _, ok := repos[id]; !ok {
			return r, errors.New("undeclared repository binding")
		}
	}
	if _, ok := repos[parts[0]]; !ok {
		return r, errors.New("API repository undeclared")
	}
	for _, repo := range m.Repositories {
		state := RepoState{Repository: repo, State: "unavailable"}
		root := bindings[repo.ID]
		if root == "" {
			r.unknown(repo.ID, "checkout-unbound")
		} else if err := bind(ctx, repo, root); err != nil {
			r.unknown(repo.ID, err.Error())
		} else {
			ready[repo.ID] = true
			head, e := git(ctx, root, "rev-parse", "HEAD")
			if e != nil {
				return r, e
			}
			state.Head = strings.TrimSpace(string(head))
			state.State = "pinned"
			if state.Head != repo.Commit {
				state.State = "pinned-historical"
				r.unknown(repo.ID, "checkout HEAD differs from pinned commit; map describes historical bytes")
			}
		}
		r.Repositories = append(r.Repositories, state)
	}
	sources := map[string]captured{}
	total := 0
	for _, s := range m.Sources {
		key := sourceKey(s.Repository, s.Path)
		if !ready[s.Repository] {
			r.unknown(key, "repository-unavailable")
			continue
		}
		b, e := readBlob(ctx, bindings[s.Repository], repos[s.Repository].Commit, s.Path, s.Blob)
		if e != nil {
			r.unknown(key, e.Error())
			continue
		}
		total += len(b)
		if total > 16*MaxBytes {
			return r, errors.New("aggregate source bytes exceed bound")
		}
		if s.End > len(strings.Split(string(b), "\n")) {
			return r, errors.New("source span outside blob")
		}
		sources[key] = captured{s, repos[s.Repository], b}
	}
	apiKey := sourceKey(parts[0], parts[1])
	selected, ok := sources[apiKey]
	if !ok {
		r.unknown(apiKey, "selected API source unavailable")
	} else if strings.HasSuffix(selected.source.Path, ".go") && !moduleBoundary(ctx, bindings[selected.repo.ID], selected, sources) {
		r.unknown(apiKey, "module boundary has an undeclared or unavailable go.mod; syntax mapping withheld")
	} else {
		analyzeGo(&r, sources, selected, parts[2])
	}
	if base != "" && ready[parts[0]] {
		repo := repos[parts[0]]
		root := bindings[parts[0]]
		ancestry, e := git(ctx, root, "merge-base", "--all", base, repo.Commit)
		if e != nil || strings.TrimSpace(string(ancestry)) != base {
			return r, errors.New("base must be an available ancestor in the API repository")
		}
		before, e := readBlob(ctx, root, base, parts[1], "")
		switch {
		case e != nil:
			r.Change = "base-path-unavailable"
			r.unknown(apiKey, "base path unavailable; added/renamed paths are not inferred")
		case !ok:
			r.Change = "target-path-unavailable"
			r.unknown(apiKey, "deleted/renamed/missing target remains unresolved")
		default:
			old, oldOK := declarationBytes(before, parts[2])
			now, newOK := declarationBytes(selected.text, parts[2])
			switch {
			case !oldOK || !newOK:
				r.Change = "declaration-unresolved"
				r.unknown(apiKey, "deleted/added/renamed/unsupported declaration is not inferred")
			case old == now:
				r.Change = "selected-declaration-unchanged"
			default:
				r.Change = "selected-declaration-changed"
			}
		}
	}
	composeProviders(&r, m, sources, apiKey)
	r.unknown("scope", "only explicitly enumerated paths examined; no complete caller, behavior, test-coverage or breakage claim")
	if r.Truncated {
		r.unknown("output", "edge bound reached; relationship frontier remains open")
	}
	if ctx.Err() != nil {
		return r, errors.New("breakage map deadline exceeded")
	}
	sort.Slice(r.Unknowns, func(i, j int) bool {
		a, b := r.Unknowns[i], r.Unknowns[j]
		return a.Subject+"\x00"+a.Reason < b.Subject+"\x00"+b.Reason
	})
	sort.Slice(r.Repositories, func(i, j int) bool { return r.Repositories[i].ID < r.Repositories[j].ID })
	return r, nil
}

func endpointKey(e extevidence.Endpoint1) string {
	if e.Repository != "" {
		return sourceKey(e.Repository, e.Path)
	}
	return "entity:" + e.Provider + ":" + e.Entity
}
func composeProviders(r *Report, m Manifest, sources map[string]captured, apiKey string) {
	if len(m.Providers) > 0 {
		r.unknown(apiKey, "provider associations bind files/entities, not the selected symbol; shared file membership does not demonstrate API dependency")
	}
	for _, raw := range m.Providers {
		record, _ := extevidence.Decode1(raw)
		hash := digest(raw)
		valid := true
		for _, pr := range record.Repositories {
			found := false
			for _, mr := range m.Repositories {
				if pr.ID == mr.ID && pr.Origin == mr.Origin && pr.Revision == mr.Commit && (pr.Tree == "" || pr.Tree == mr.Tree) {
					found = true
				}
			}
			if !found {
				valid = false
				r.unknown(record.Provider.ID, "provider repository pin mismatch or undeclared repository")
			}
		}
		if !valid {
			continue
		}
		declared := map[string]bool{}
		for _, pr := range record.Repositories {
			declared[pr.ID] = true
		}
		entities := map[string]bool{}
		for _, e := range record.Entities {
			entities["entity:"+record.Provider.ID+":"+e.ID] = true
		}
		reached := map[string]bool{apiKey: true}
		used := make([]bool, len(record.Relations))
		for progress := true; progress; {
			progress = false
			for i, rel := range record.Relations {
				a, b := endpointKey(rel.From), endpointKey(rel.To)
				if used[i] || (!reached[a] && !reached[b]) {
					continue
				}
				used[i] = true
				progress = true
				edge := Edge{Kind: "provider-relationship", Confidence: "external-provider", Reason: "explicit file/entity EEP association, not symbol-specific coupling; byte verification does not establish assertion truth", ProviderSHA256: hash, Relation: &record.Relations[i], ByteState: "verified"}
				good := true
				if rel.Evidence != "declared" && rel.Evidence != "observed" && rel.Evidence != "inferred" && rel.Evidence != "generated" {
					good = false
					r.unknown(record.Provider.ID, "unsupported EEP evidence kind")
				}
				if record.Schema == extevidence.Schema1 && rel.From.Repository != "" && rel.To.Repository != "" {
					good = false
					r.unknown(record.Provider.ID, "path-to-path relations require EEP V2")
				}
				for side, e := range []extevidence.Endpoint1{rel.From, rel.To} {
					key := endpointKey(e)
					if e.Repository == "" {
						if !entities[key] {
							r.unknown(key, "provider entity undeclared")
							good = false
						}
						edge.ByteState = "entity-declaration"
						continue
					}
					c, ok := sources[key]
					if !declared[e.Repository] || !safePath(e.Path) || !ok || (e.Blob != "" && c.source.Blob != e.Blob) {
						r.unknown(key, "EEP endpoint unbound, missing, out of scope or blob mismatch")
						good = false
						continue
					}
					anchor, valid := c.anchor(c.source.Start, c.source.End)
					if !valid {
						r.unknown(key, "EEP endpoint outside supplied physical span")
						good = false
						continue
					}
					if side == 0 {
						edge.From = &anchor
					} else {
						edge.To = &anchor
					}
				}
				if !good {
					edge.ByteState = "unresolved"
				} else {
					reached[a] = true
					reached[b] = true
				}
				r.add(edge)
			}
		}
		for i, rel := range record.Relations {
			if !used[i] {
				r.unknown(fmt.Sprintf("%s:relation:%d", record.Provider.ID, i), "not reached from selected API: "+rel.Type)
			}
		}
	}
}
