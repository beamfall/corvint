package appmap

import (
	"context"
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// ImpactSchema names the change-to-navigation projection (ANL-V0-015).
const ImpactSchema = "application-map-impact/0"

const maxImpactPaths = 4096

// Verdicts: an unmapped UI path anywhere forces the whole app's suite (fail closed).
const (
	VerdictTargeted     = "targeted"
	VerdictFullSuite    = "full-suite-required"
	VerdictNoNavigation = "no-navigation-impact"
)

type ImpactScreen struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
	Basis string   `json:"basis"`
}
type ImpactEdge struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Status  string `json:"status"`
	Reprove string `json:"reprove_with,omitempty"`
	Flipped bool   `json:"flipped"`
}
type ImpactUnknown struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type ImpactReport struct {
	Schema       string          `json:"schema"`
	App          string          `json:"app"`
	MapRevision  string          `json:"map_revision"`
	Base         string          `json:"base"`
	Head         string          `json:"head"`
	MapDigest    string          `json:"map_digest"`
	LedgerDigest string          `json:"ledger_digest,omitempty"`
	Authority    string          `json:"authority"`
	Verdict      string          `json:"verdict"`
	Changed      int             `json:"changed_paths"`
	Ignored      int             `json:"ignored_paths"`
	Screens      []ImpactScreen  `json:"screens"`
	Edges        []ImpactEdge    `json:"edges"`
	SpecFiles    []string        `json:"spec_files"`
	TestKeys     []string        `json:"test_keys"`
	Unknown      []ImpactUnknown `json:"unknown"`
	Omitted      map[string]int  `json:"omitted"`
	Digest       string          `json:"digest"`
}

var impactUIExtensions = map[string]bool{".html": true, ".htm": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".vue": true, ".svelte": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	".pug": true, ".hbs": true, ".mdx": true}

// Impact maps the paths changed between base and the evaluated revision onto screens (router
// anchors and their lineage descendants, attributed test files and flow anchors), the edges
// touching them and the tests that cross those edges. A changed UI path the map cannot place is
// listed as unknown and forces full-suite-required (ANL-V0-015, ANL-V0-016).
func Impact(ctx context.Context, m *Map, l *EdgeLedger, base string, o Options) ([]byte, error) {
	cap, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	r := repo{ctx: ctx, root: o.Root}
	head := o.Revision
	if head == "" {
		head = "HEAD"
	}
	headOID, err := r.resolve(head)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "evaluated revision is not a commit")
	}
	if err = checkLedgerBinding(m, l, headOID); err != nil {
		return nil, err
	}
	baseOID, err := r.resolve(base)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "--base is not a commit")
	}
	out, err := r.git(8<<20, nil, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", baseOID, headOID, "--")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			changed = append(changed, p)
		}
	}
	if len(changed) > maxImpactPaths {
		return nil, budgetError("more than %d changed paths", maxImpactPaths)
	}
	sort.Strings(changed)
	rep := ImpactReport{Schema: ImpactSchema, App: m.App, MapRevision: m.Revision, Base: baseOID, Head: headOID, MapDigest: m.Digest,
		Authority: AuthorityLearned, Changed: len(changed), Screens: []ImpactScreen{}, Edges: []ImpactEdge{}, SpecFiles: []string{},
		TestKeys: []string{}, Unknown: []ImpactUnknown{}, Omitted: map[string]int{"screens": 0, "edges": 0, "spec_files": 0, "test_keys": 0, "unknown": 0}}
	if l != nil {
		rep.LedgerDigest = l.Digest
	}
	direct, hit := map[string][]string{}, map[string]*ImpactScreen{}
	touch := func(id, p, basis string) {
		s := hit[id]
		if s == nil {
			s = &ImpactScreen{ID: id, Basis: basis}
			hit[id] = s
		}
		if len(s.Paths) == 0 || s.Paths[len(s.Paths)-1] != p {
			s.Paths = append(s.Paths, p)
		}
	}
	specs := map[string]bool{}
	for _, p := range changed {
		placed := false
		if p == m.Manifest.Path && m.Manifest.Repo == "" {
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "manifest-changed"})
			continue
		}
		for i := range m.Screens {
			s := &m.Screens[i]
			for _, a := range append(append([]Anchor{s.Anchor}, s.NameFrom...), s.ParentFrom...) {
				if a.Repo == "" && a.Path == p {
					direct[s.ID] = append(direct[s.ID], p)
					touch(s.ID, p, "router-anchor")
					placed = true
				}
			}
			for _, a := range s.Specs {
				if a.File == p || contains(a.Via, p) {
					touch(s.ID, p, "test-attribution")
					specs[a.File] = true
					placed = true
				}
			}
		}
		for _, f := range m.Files {
			if f.Path == p && f.Anchor.Repo == "" {
				placed = true
				if f.Screen != "" {
					touch(f.Screen, p, "test-file")
				}
				if f.Role == roleSpec {
					specs[p] = true
				}
			}
		}
		for _, fl := range m.Flows {
			if fl.Anchor.Repo == "" && fl.Anchor.Path == p {
				placed = true
				for _, st := range fl.Steps {
					if st.Screen != "" {
						touch(st.Screen, p, "flow-anchor")
					}
				}
			}
		}
		switch {
		case placed:
		case impactUIExtensions[strings.ToLower(path.Ext(p))]:
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "unmapped-path"})
		default:
			rep.Ignored++
		}
	}
	for i := range m.Screens {
		s := &m.Screens[i]
		seen := map[string]bool{s.ID: true}
		for cur := m.screen(s.Parent); cur != nil && !seen[cur.ID]; cur = m.screen(cur.Parent) {
			seen[cur.ID] = true
			if ps, ok := direct[cur.ID]; ok {
				for _, p := range ps {
					touch(s.ID, p, "lineage:"+cur.ID)
				}
				break
			}
		}
	}
	for _, s := range hit {
		sort.Strings(s.Paths)
		rep.Screens = append(rep.Screens, *s)
	}
	sort.Slice(rep.Screens, func(i, j int) bool { return rep.Screens[i].ID < rep.Screens[j].ID })
	for _, s := range m.Screens {
		if hit[s.ID] != nil {
			for _, a := range s.Specs {
				specs[a.File] = true
			}
		}
	}
	keys := map[string]bool{}
	if l != nil {
		for _, e := range l.Edges {
			if hit[e.From] == nil && hit[e.To] == nil {
				continue
			}
			ie := ImpactEdge{ID: e.ID, From: e.From, To: e.To, Status: e.Status, Flipped: e.Status == "UNVERIFIED_AT_HEAD"}
			if e.Evidence != nil && e.Evidence.Verification.Receipt != nil {
				ie.Reprove = e.Evidence.Verification.Receipt.TestKey
				keys[ie.Reprove] = true
			}
			for _, k := range e.RedundantCrossers {
				keys[k] = true
			}
			rep.Edges = append(rep.Edges, ie)
		}
	} else {
		for _, e := range m.Edges {
			if hit[e.From] != nil || hit[e.To] != nil {
				rep.Edges = append(rep.Edges, ImpactEdge{ID: prefixEdgeID(e.From, e.To), From: e.From, To: e.To, Status: "NOT_IN_LEDGER"})
			}
		}
	}
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" && (hit[e.From] != nil || hit[e.To] != nil) {
			for _, f := range m.Files {
				if f.ID == e.Source && f.Role == roleSpec {
					specs[f.Path] = true
				}
			}
		}
	}
	sort.Slice(rep.Edges, func(i, j int) bool { return rep.Edges[i].ID < rep.Edges[j].ID })
	for f := range specs {
		if f != "" {
			rep.SpecFiles = append(rep.SpecFiles, f)
		}
	}
	for k := range keys {
		rep.TestKeys = append(rep.TestKeys, k)
	}
	sort.Strings(rep.SpecFiles)
	sort.Strings(rep.TestKeys)
	switch {
	case len(rep.Unknown) > 0:
		rep.Verdict = VerdictFullSuite
	case len(rep.Screens) > 0:
		rep.Verdict = VerdictTargeted
	default:
		rep.Verdict = VerdictNoNavigation
	}
	complete, _ := json.Marshal(rep)
	rep.Digest = digest(complete)
	for {
		raw, err := json.Marshal(rep)
		if err != nil {
			return nil, err
		}
		raw = append(raw, '\n')
		if len(raw) <= cap {
			return raw, nil
		}
		switch {
		case len(rep.TestKeys) > 0:
			rep.TestKeys = rep.TestKeys[:len(rep.TestKeys)-1]
			rep.Omitted["test_keys"]++
		case len(rep.SpecFiles) > 0:
			rep.SpecFiles = rep.SpecFiles[:len(rep.SpecFiles)-1]
			rep.Omitted["spec_files"]++
		case len(rep.Edges) > 0:
			rep.Edges = rep.Edges[:len(rep.Edges)-1]
			rep.Omitted["edges"]++
		case len(rep.Screens) > 0:
			rep.Screens = rep.Screens[:len(rep.Screens)-1]
			rep.Omitted["screens"]++
		case len(rep.Unknown) > 1:
			rep.Unknown = rep.Unknown[:len(rep.Unknown)-1]
			rep.Omitted["unknown"]++
		default:
			return nil, budgetError("impact report header exceeds --budget")
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// checkLedgerBinding refuses a ledger from another app, map digest or evaluated revision, or one
// trimmed to a byte cap (a nil ledger passes).
func checkLedgerBinding(m *Map, l *EdgeLedger, evaluated string) error {
	if l == nil {
		return nil
	}
	if l.App != m.App || l.MapDigest != m.Digest || l.Revision != evaluated {
		return verifyError("appmap-invalid-query", "the edge ledger was not built from this map at the evaluated revision")
	}
	for _, n := range l.Omitted {
		if n > 0 {
			return verifyError("appmap-invalid-query", "the edge ledger is trimmed; rebuild it with --full")
		}
	}
	return nil
}
