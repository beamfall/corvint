package appmap

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Navigation observation (ANL-V0-006..008): runtime URL events from a Playwright reporter become
// ordered screen visits and observed edges. They are overlay facts keyed to the map; they never
// add or change a static node or edge.
const (
	ObservationSchema      = "application-navigation-observation/0"
	NavigationEventsSchema = "playwright-navigation-events/0"
	UnmappedURL            = "unmapped-url"
	maxTraceFiles          = 64
	maxTraceEvents         = 4096
	redactedValue          = "REDACTED"
)

// NavigationEvents is the closed reporter document: page.on('framenavigated') URLs in order and
// the Playwright action (from trace metadata) that triggered each.
type NavigationEvents struct {
	Schema  string            `json:"schema"`
	TestKey string            `json:"test_key"`
	Events  []NavigationEvent `json:"events"`
}

// NavigationEvent kind is "navigate" or "redirect"; Action is the triggering action, if any.
type NavigationEvent struct {
	Kind   string            `json:"kind"`
	URL    string            `json:"url"`
	Action *NavigationAction `json:"action,omitempty"`
}

// NavigationAction is trace action metadata. Value (a typed or filled input) is read only to be
// dropped; it is never echoed.
type NavigationAction struct {
	Kind     string `json:"kind"`
	Selector string `json:"selector,omitempty"`
	Value    string `json:"value,omitempty"`
}

type ObservedVisit struct {
	Seq        int      `json:"seq"`
	URL        string   `json:"url"`
	Screen     string   `json:"screen,omitempty"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Redirect   bool     `json:"redirect,omitempty"`
}

type ObservedEdge struct {
	FromSeq  int    `json:"from_seq"`
	ToSeq    int    `json:"to_seq"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Trigger  string `json:"trigger"`
	Selector string `json:"selector,omitempty"`
	Status   string `json:"status"`
}

type ObservedTrace struct {
	TestKey     string          `json:"test_key"`
	TraceDigest string          `json:"trace_digest"`
	Visits      []ObservedVisit `json:"visits"`
	Edges       []ObservedEdge  `json:"edges"`
}

type Observation struct {
	Schema    string          `json:"schema"`
	App       string          `json:"app"`
	Revision  string          `json:"revision"`
	MapDigest string          `json:"map_digest"`
	Authority string          `json:"authority"`
	Traces    []ObservedTrace `json:"traces"`
	Omitted   map[string]int  `json:"omitted"`
	Digest    string          `json:"digest"`
}

// LoadNavigationEvents reads one reporter file or every *.json file of a directory (at most 64,
// sorted by name). Playwright trace zips are not read in this slice and are refused.
func LoadNavigationEvents(name string) ([]NavigationEvents, [][]byte, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s", msg) }
	info, err := os.Lstat(name)
	if err != nil {
		return nil, nil, bad("--trace must name a reporter JSON file or directory")
	}
	files := []string{name}
	if info.IsDir() {
		entries, err := os.ReadDir(name)
		if err != nil {
			return nil, nil, bad("--trace directory unreadable")
		}
		files = nil
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".zip") {
				return nil, nil, bad("trace zips are not supported; supply the reporter JSON")
			}
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
				files = append(files, filepath.Join(name, e.Name()))
			}
		}
		sort.Strings(files)
	}
	if len(files) == 0 || len(files) > maxTraceFiles {
		return nil, nil, bad("--trace needs 1 to 64 reporter JSON files")
	}
	var docs []NavigationEvents
	var raws [][]byte
	for _, f := range files {
		if strings.HasSuffix(f, ".zip") {
			return nil, nil, bad("trace zips are not supported; supply the reporter JSON")
		}
		raw, err := readReceipt(f)
		if err != nil {
			return nil, nil, bad("--trace must name regular reporter JSON files of at most 4 MiB, unchanged while read")
		}
		var d NavigationEvents
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF || d.Schema != NavigationEventsSchema || !validTestKey(d.TestKey) || len(d.Events) > maxTraceEvents {
			return nil, nil, bad("invalid navigation events document")
		}
		for _, ev := range d.Events {
			if (ev.Kind != "navigate" && ev.Kind != "redirect") || ev.URL == "" || len(ev.URL) > 4096 {
				return nil, nil, bad("navigation event kind must be navigate or redirect with a URL of at most 4096 bytes")
			}
		}
		docs = append(docs, d)
		raws = append(raws, raw)
	}
	return docs, raws, nil
}

var secretKey = regexp.MustCompile(`(?i)(token|secret|passw|pwd|auth|session|sig|key|code|credential|bearer|jwt|cookie)`)
var opaqueValue = regexp.MustCompile(`^[A-Za-z0-9_\-+/=.]{24,}$`)

// redactURL drops userinfo and the fragment payload of non-route data, and replaces a query value
// whose name is secret-shaped or whose value looks opaque (ANL-V0-007).
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return redactedValue
	}
	u.User = nil
	if u.RawQuery != "" {
		q := u.Query()
		for k, vs := range q {
			for i, v := range vs {
				if secretKey.MatchString(k) || opaqueValue.MatchString(v) {
					vs[i] = redactedValue
				}
			}
		}
		u.RawQuery = q.Encode()
	}
	if strings.Contains(u.Fragment, "?") {
		// Hash-routed apps carry the query inside the fragment.
		frag, query, _ := strings.Cut(u.Fragment, "?")
		q, err := url.ParseQuery(query)
		if err != nil {
			q = url.Values{"_": {redactedValue}}
		}
		for k, vs := range q {
			for j, v := range vs {
				if secretKey.MatchString(k) || opaqueValue.MatchString(v) {
					vs[j] = redactedValue
				}
			}
		}
		u.Fragment = frag + "?" + q.Encode()
	}
	return u.String()
}

// Observe resolves each event URL through the map's template lookup. A URL matching several
// screens reads UNKNOWN with candidates; one matching none reads unmapped-url (ANL-V0-006).
func Observe(m *Map, docs []NavigationEvents, raws [][]byte, revision string, o Options) ([]byte, error) {
	cap, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	if len(docs) != len(raws) {
		return nil, verifyError("appmap-invalid-query", "events and digests differ")
	}
	x := newScreenIndex(m.HashPrefix, m.Screens)
	obs := Observation{Schema: ObservationSchema, App: m.App, Revision: revision, MapDigest: m.Digest, Authority: AuthorityLearned, Traces: []ObservedTrace{}, Omitted: map[string]int{"visits": 0, "edges": 0}}
	for i, d := range docs {
		tr := ObservedTrace{TestKey: d.TestKey, TraceDigest: digest(raws[i]), Visits: []ObservedVisit{}, Edges: []ObservedEdge{}}
		for n, ev := range d.Events {
			v := ObservedVisit{Seq: n, URL: redactURL(ev.URL), Redirect: ev.Kind == "redirect"}
			ids, reason := x.urlMatches(ev.URL)
			switch {
			case reason == "" && len(ids) == 1:
				v.Screen, v.Status = ids[0], StatusResolved
			case reason == "" && len(ids) > 1:
				v.Status, v.Reason, v.Candidates = StatusUnknown, "ambiguous-template", append([]string{}, ids...)
				sort.Strings(v.Candidates)
			case reason == "" || reason == "no-matching-screen":
				v.Status, v.Reason = UnmappedURL, UnmappedURL
			default:
				v.Status, v.Reason = StatusUnknown, reason
			}
			if n > 0 {
				prev := tr.Visits[n-1]
				e := ObservedEdge{FromSeq: n - 1, ToSeq: n, From: prev.Screen, To: v.Screen, Trigger: "goto", Status: StatusResolved}
				if ev.Kind == "redirect" {
					e.Trigger = "redirect"
				} else if a := ev.Action; a != nil {
					switch a.Kind {
					case "click", "goto":
						e.Trigger = a.Kind
					case "submit", "press":
						e.Trigger = "form submit"
					default:
						e.Trigger = StatusUnknown
					}
					e.Selector = a.Selector
				}
				if prev.Status != StatusResolved || v.Status != StatusResolved {
					e.Status = StatusUnknown
				}
				tr.Edges = append(tr.Edges, e)
			}
			tr.Visits = append(tr.Visits, v)
		}
		obs.Traces = append(obs.Traces, tr)
	}
	complete, _ := json.Marshal(obs)
	obs.Digest = digest(complete)
	for {
		raw, err := json.Marshal(obs)
		if err != nil {
			return nil, err
		}
		raw = append(raw, '\n')
		if len(raw) <= cap {
			return raw, nil
		}
		trimmed := false
		for j := len(obs.Traces) - 1; j >= 0 && !trimmed; j-- {
			t := &obs.Traces[j]
			switch {
			case len(t.Edges) > 0:
				t.Edges = t.Edges[:len(t.Edges)-1]
				obs.Omitted["edges"]++
				trimmed = true
			case len(t.Visits) > 0:
				t.Visits = t.Visits[:len(t.Visits)-1]
				obs.Omitted["visits"]++
				trimmed = true
			}
		}
		if !trimmed {
			return nil, budgetError("observation header exceeds --budget")
		}
	}
}
