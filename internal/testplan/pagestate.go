package testplan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// PageStateSchema names the page-state grouping view (ANL-V0-012).
const PageStateSchema = "test-page-state-groups/0"

// PageStateGroup is variations that can assert on one visit of the same page state.
type PageStateGroup struct {
	Key          string   `json:"key"`
	App          string   `json:"app"`
	Screen       string   `json:"screen"`
	Setup        string   `json:"setup"`
	User         string   `json:"user"`
	Org          string   `json:"org"`
	Requires     []string `json:"requires"`
	VariationIDs []string `json:"variation_ids"`
	Specs        []string `json:"specs"`
	Assertions   []string `json:"assertions"`
	VisitsSaved  int      `json:"visits_saved"`
}

// PageStateBlocked is a variation that is never merged into a group, with its reason.
type PageStateBlocked struct {
	VariationID string `json:"variation_id"`
	Reason      string `json:"reason"`
}

// PageStateReport is the deterministic, read-only grouping of one input.
type PageStateReport struct {
	Schema      string             `json:"schema"`
	InputDigest string             `json:"input_digest"`
	Groups      []PageStateGroup   `json:"groups"`
	Blocked     []PageStateBlocked `json:"blocked"`
	Digest      string             `json:"digest"`
}

// GroupByPageState groups variations that share app, screen, setup, user, org and the required
// fact set when none is destructive or changes state (ANL-V0-012, ANL-V0-013).
func GroupByPageState(in *Input) ([]byte, error) {
	groups := map[string]*PageStateGroup{}
	r := PageStateReport{Schema: PageStateSchema, InputDigest: in.Digest(), Groups: []PageStateGroup{}, Blocked: []PageStateBlocked{}}
	for _, v := range in.Variations {
		if reason := pageStateBlock(v); reason != "" {
			r.Blocked = append(r.Blocked, PageStateBlocked{VariationID: v.ID, Reason: reason})
			continue
		}
		requiresJSON, _ := json.Marshal(v.Requires)
		requires := string(requiresJSON)
		key := strings.Join([]string{*v.App, *v.Screen, *v.Setup, *v.User, *v.Org, requires}, "\x1f")
		sum := sha256.Sum256([]byte(key))
		id := "pagestate:" + hex.EncodeToString(sum[:8])
		g := groups[id]
		if g == nil {
			g = &PageStateGroup{Key: id, App: *v.App, Screen: *v.Screen, Setup: *v.Setup, User: *v.User, Org: *v.Org, Requires: append([]string{}, v.Requires...)}
			groups[id] = g
		}
		g.VariationIDs = append(g.VariationIDs, v.ID)
		g.Specs = appendUnique(g.Specs, *v.Spec)
		for _, a := range v.Assertion {
			g.Assertions = appendUnique(g.Assertions, a)
		}
	}
	for _, g := range groups {
		sort.Strings(g.VariationIDs)
		sort.Strings(g.Specs)
		sort.Strings(g.Assertions)
		g.VisitsSaved = len(g.VariationIDs) - 1
		if g.VisitsSaved > 0 {
			r.Groups = append(r.Groups, *g)
		}
	}
	sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].Key < r.Groups[j].Key })
	sort.Slice(r.Blocked, func(i, j int) bool { return r.Blocked[i].VariationID < r.Blocked[j].VariationID })
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	r.Digest = "sha256:" + hex.EncodeToString(sum[:])
	out, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if len(out) > MaxOutputBytes {
		return nil, boundExceeded("the page-state report exceeds 8 MiB")
	}
	return append(out, '\n'), nil
}

// pageStateBlock names why a variation cannot join a shared visit; empty means eligible.
func pageStateBlock(v Variation) string {
	switch {
	case len(v.Missing) > 0:
		return "anchor-missing:" + strings.Join(v.Missing, ",")
	case v.Destructive == nil || *v.Destructive:
		return "destructive"
	case len(v.Changes) > 0:
		return "changes-state"
	}
	return ""
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// DecodePageStateReport strictly reads an earlier page-state report as a drift baseline: the
// schema, no unknown members or trailing data, and a digest that matches its own content
// (ANL-V0-033).
func DecodePageStateReport(raw []byte) (*PageStateReport, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var r PageStateReport
	if err := d.Decode(&r); err != nil {
		return nil, invalidInput("page-state baseline is not a %s report", PageStateSchema)
	}
	if d.Decode(new(json.RawMessage)) != io.EOF || r.Schema != PageStateSchema || r.Groups == nil || r.Blocked == nil {
		return nil, invalidInput("page-state baseline is not a %s report", PageStateSchema)
	}
	want := r.Digest
	r.Digest = ""
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	if want != "sha256:"+hex.EncodeToString(sum[:]) {
		return nil, invalidInput("page-state baseline digest does not match its content")
	}
	r.Digest = want
	return &r, nil
}

// PageStateDrift compares a fresh report with a baseline by group key and blocked variation. It
// returns nil when they agree and a test-plan-mismatch naming the added, removed and changed
// groups and the changed blocked variations otherwise. Input digests are not compared: an
// input edit that leaves every group and block unchanged is not drift (ANL-V0-033).
func PageStateDrift(baseline, current *PageStateReport) error {
	old, now := map[string]PageStateGroup{}, map[string]PageStateGroup{}
	for _, g := range baseline.Groups {
		old[g.Key] = g
	}
	for _, g := range current.Groups {
		now[g.Key] = g
	}
	var added, removed, changed []string
	for k, g := range now {
		if o, ok := old[k]; !ok {
			added = append(added, k)
		} else if !reflect.DeepEqual(o, g) {
			changed = append(changed, k)
		}
	}
	for k := range old {
		if _, ok := now[k]; !ok {
			removed = append(removed, k)
		}
	}
	blocks := map[string]string{}
	for _, b := range baseline.Blocked {
		blocks[b.VariationID] = b.Reason
	}
	var blocked []string
	for _, b := range current.Blocked {
		if r, ok := blocks[b.VariationID]; !ok || r != b.Reason {
			blocked = append(blocked, b.VariationID)
		}
		delete(blocks, b.VariationID)
	}
	for id := range blocks {
		blocked = append(blocked, id)
	}
	if len(added)+len(removed)+len(changed)+len(blocked) == 0 {
		return nil
	}
	list := func(name string, ids []string) string {
		n := len(ids)
		sort.Strings(ids)
		if n > 8 {
			ids = append(ids[:8:8], fmt.Sprintf("+%d more", n-8))
		}
		return fmt.Sprintf("%s %d [%s]", name, n, strings.Join(ids, ","))
	}
	return mismatch("page-state groups drifted from the baseline: %s; %s; %s; %s", list("added", added), list("removed", removed), list("changed", changed), list("blocked-changed", blocked))
}
