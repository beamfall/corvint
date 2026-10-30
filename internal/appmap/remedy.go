package appmap

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// Remedy names the declaration change that would resolve one unknown (AMAP-V0-210). Key is the
// declaration member to edit in File; Candidates are values inferred from the map, offered and
// never chosen when there is more than one; Uncertainty states what could not be inferred. An
// empty Key means no declaration change is inferred and Uncertainty says why: a remedy is never
// invented.
type Remedy struct {
	File        string   `json:"file,omitempty"`
	Key         string   `json:"key,omitempty"`
	Candidates  []string `json:"candidates,omitempty"`
	Basis       string   `json:"basis,omitempty"`
	Uncertainty string   `json:"uncertainty,omitempty"`
}

// Manifest members a remedy can name.
const (
	RemedyKeyPageObjectScreens = "page_object_screens"
	RemedyKeyRouters           = "routers"
	RemedyKeyDIClosureLimit    = "di_closure_limit"
)

func uncertain(why string) *Remedy { return &Remedy{Uncertainty: why} }

// pageObjectCandidate is one page_object_screens entry, encoded as the manifest spells it.
func pageObjectCandidate(p, state string) string {
	raw, _ := json.Marshal(struct {
		Path  string `json:"path"`
		State string `json:"state"`
	}{p, state})
	return string(raw)
}

// UnknownRemedy returns the remedy for one map unknown (AMAP-V0-210).
func UnknownRemedy(m *Map, u Unknown) *Remedy {
	manifest := m.Manifest.Path
	if limit, ok := closureLimitHit(u.Detail); ok {
		return &Remedy{File: manifest, Key: RemedyKeyDIClosureLimit,
			Basis: "the examined import closure of an injected table holds more than " + limit + " files",
			Uncertainty: "the closure size is not inferred; set di_closure_limit above " + limit + " (at most " +
				strconv.Itoa(maxReachFiles) + "), or narrow what the registering module imports"}
	}
	switch u.Reason {
	case "router-not-declared":
		return &Remedy{File: manifest, Key: RemedyKeyRouters, Candidates: []string{u.Path},
			Basis:       "a router_globs file the declared routers do not name",
			Uncertainty: "the router dialect of " + u.Path + " is not inferred; declare it or narrow router_globs"}
	case "unknown-state":
		return &Remedy{File: manifest, Key: RemedyKeyPageObjectScreens,
			Uncertainty: "the declared state for " + u.Path + " names no state in the map; correct the entry's state"}
	case "unresolved-screen":
		return uncertain("the declared state for " + u.Path + " is in the map, but its screen is unresolved (" + u.Detail +
			"); resolve that screen's own unknown, the entry's state needs no change")
	case "page-object-unbound", "page-object-ambiguous", "page-object-unresolved-target":
	default:
		return uncertain("no declaration change is inferred for " + u.Kind + " " + u.Reason)
	}
	var f *TestFile
	for i := range m.Files {
		if m.Files[i].Path == u.Path && m.Files[i].Anchor.Repo == u.Repo {
			f = &m.Files[i]
		}
	}
	r := &Remedy{File: manifest, Key: RemedyKeyPageObjectScreens}
	if f == nil {
		r.Uncertainty = "page_object_screens declares " + u.Path + " but the map holds no such page object; correct or remove the entry"
		return r
	}
	states, lines, unresolved := map[string][]string{}, []string{}, []string{}
	for _, g := range f.Gotos {
		line := strconv.Itoa(g.Line)
		if g.Screen == "" {
			unresolved = append(unresolved, line)
			continue
		}
		if s := m.screen(g.Screen); s != nil {
			states[s.State] = append(states[s.State], line)
			lines = append(lines, line)
		}
	}
	learned := false
	if len(f.Gotos) == 0 && len(f.HelperGotos) > 0 { // learned through one helper hop (AMAP-V0-342)
		learned = true
		for _, g := range f.HelperGotos {
			line := strconv.Itoa(g.Line)
			if s := m.screen(g.Screen); g.Screen != "" && s != nil {
				states[s.State] = append(states[s.State], line)
				lines = append(lines, line)
			} else {
				unresolved = append(unresolved, line)
			}
		}
	}
	keys := make([]string, 0, len(states))
	for s := range states {
		keys = append(keys, s)
	}
	sort.Strings(keys)
	for _, s := range keys {
		r.Candidates = append(r.Candidates, pageObjectCandidate(u.Path, s))
	}
	if len(lines) > 0 {
		r.Basis = "resolved navigation at lines " + strings.Join(lines, ",")
		if learned {
			r.Basis = "learned: a literal URL passed to a navigation helper at lines " + strings.Join(lines, ",")
		}
	}
	switch {
	case learned && len(keys) == 0:
		r.Uncertainty = "the page object navigates only through helper calls at lines " + strings.Join(unresolved, ",") + " whose URLs resolve to no screen; the screen it models is not inferred"
	case learned:
		r.Uncertainty = "learned from a helper's URL parameter, not the page object's own navigation; the map binds no screen from it, so declare the one it models"
		if len(unresolved) > 0 {
			r.Uncertainty += "; helper calls at lines " + strings.Join(unresolved, ",") + " resolve to no screen"
		}
	case len(unresolved) > 0:
		r.Uncertainty = "navigation at lines " + strings.Join(unresolved, ",") + " resolves to no screen; the screen it models is not inferred"
	case len(keys) == 0:
		r.Uncertainty = "the page object navigates nowhere the map resolves; the screen it models is not inferred"
	case len(keys) > 1:
		r.Uncertainty = "the page object reaches " + strconv.Itoa(len(keys)) + " screens; declare the one it models"
	}
	return r
}

// impactRemedy returns the remedy for one impact unknown (AMAP-V0-210).
func impactRemedy(m *Map, u ImpactUnknown) *Remedy {
	switch u.Reason {
	case "manifest-changed":
		return uncertain("the manifest changed; rebuild the map at head, no declaration bounds it")
	case "non-ui-path":
		return uncertain("the application map declares no exemption (ANL-V0-048); only e2e obligations reads reach_exempt")
	case "test-support-unreached":
		return uncertain("no spec imports " + u.Path + "; a spec must import it, no declaration bounds it")
	case "endpoint-callers-incomplete":
		return uncertain("the endpoint join has unknowns (" + u.Detail + "); resolve them as `flows appmap endpoints` names them")
	case "endpoint-not-called":
		return uncertain("no read client call reaches " + u.Detail + "; a caller outside the --source directories is not bounded")
	case "endpoint-declaration-changed":
		return uncertain("the endpoints at base and head are not fully compared (" + u.Detail + "); resolve the route unknowns as `flows appmap endpoints` names them")
	case "test-import-graph-incomplete":
		return uncertain("the test import graph has unresolved imports; resolve the map's import unknowns")
	}
	for _, mu := range m.Unknowns {
		if mu.Repo == "" && mu.Path == u.Path {
			if r := UnknownRemedy(m, mu); r.Key != "" {
				return r
			}
		}
	}
	why := "no declaration change is inferred for " + u.Reason
	if u.Detail != "" {
		why += " (" + u.Detail + ")"
	}
	return uncertain(why)
}

// closureLimitHit returns N when detail ends in the closure hop `closure-limit:N` that
// di_closure_limit raises (AMAP-V0-369); a closure over maxReachFiles names that bound, which no
// manifest value raises.
func closureLimitHit(detail string) (string, bool) {
	i := strings.LastIndex(detail, "closure-limit:")
	if i < 0 {
		return "", false
	}
	n := detail[i+len("closure-limit:"):]
	v, err := strconv.Atoi(n)
	return n, err == nil && v < maxReachFiles
}
