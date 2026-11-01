package appmap

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Observed UI records (docs/specs/observed-ui-evidence-v0.md, OUE-V0-001..012). Playwright trace
// zips from ordinary test runs are read in memory, never extracted, and mined into screen,
// transition, endpoint, page-object binding and ARIA structure records labelled
// LOCALLY_OBSERVED, bound to the declared test key and the stated source revision. Header,
// body, query and typed-input values are never decoded into the output; only their counts are.
// The records are learned evidence: they never change a map node, edge or verdict.
const (
	UIRecordsSchema         = "observed-ui-records/0"
	UIRoutesSchema          = "observed-ui-routes/0"
	EvidenceLocallyObserved = "LOCALLY_OBSERVED"

	BasisMap     = "map"
	BasisRoute   = "route-template"
	BasisDefault = "default-normalized"
	BasisUnknown = StatusUnknown

	maxUIRecordsBytes = 8 << 20
	maxUIRoutes       = 1024
	maxUIStackFrames  = 64
)

// UIRoutes is the closed observed-ui-routes/0 normalization config (OUE-V0-004). A template
// segment `:name` or `{name}` matches one non-empty segment; a final `*` matches the rest.
// Paths are the repository-relative source paths a record normalized through the template links
// to, for staleness (OUE-V0-030).
type UIRoutes struct {
	Schema      string    `json:"schema"`
	HashRouting bool      `json:"hash_routing,omitempty"`
	Screens     []UIRoute `json:"screens"`
	Endpoints   []UIRoute `json:"endpoints"`
}

// UIRoute is one route template and the source paths it links to.
type UIRoute struct {
	Template string   `json:"template"`
	Paths    []string `json:"paths,omitempty"`
}

// UITraceInput names one trace zip and the test identity it was recorded for.
type UITraceInput struct {
	TestKey string
	File    string
}

// UIRecords is the observed-ui-records/0 document.
type UIRecords struct {
	Schema       string         `json:"schema"`
	Source       string         `json:"source"`
	Evidence     string         `json:"evidence"`
	Authority    string         `json:"authority"`
	Revision     string         `json:"revision"`
	App          string         `json:"app,omitempty"`
	MapDigest    string         `json:"map_digest,omitempty"`
	RoutesDigest string         `json:"routes_digest,omitempty"`
	Traces       []UITrace      `json:"traces"`
	Screens      []UIScreen     `json:"screens"`
	Transitions  []UITransition `json:"transitions"`
	Endpoints    []UIEndpoint   `json:"endpoints"`
	Bindings     []UIBinding    `json:"bindings"`
	Unbound      []UIUnbound    `json:"unbound"`
	Structures   []UIStructure  `json:"structures"`
	Dropped      UIDropped      `json:"dropped"`
	Unknowns     []Unknown      `json:"unknowns"`
	Digest       string         `json:"digest"`
}

// UITrace is one admitted trace and what it contributed.
type UITrace struct {
	TestKey       string `json:"test_key"`
	TraceSHA256   string `json:"trace_sha256"`
	Actions       int    `json:"actions"`
	Navigations   int    `json:"navigations"`
	Requests      int    `json:"requests"`
	StackFrames   int    `json:"stack_frames"`
	MatchedFrames int    `json:"matched_frames"`
	AriaSnapshots int    `json:"aria_snapshots"`
	// AriaOversized counts snapshots over the line bound, recorded as no structure.
	AriaOversized int `json:"aria_oversized"`
}

// UIScreen is one observed screen. ID is the map screen ID (basis map), `route:TEMPLATE` (basis
// route-template) or `url:PATH` (basis default-normalized); an UNKNOWN screen keeps its reason
// and any candidates and is never merged with a resolved one.
type UIScreen struct {
	ID         string   `json:"id"`
	Basis      string   `json:"basis"`
	Reason     string   `json:"reason,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Template   string   `json:"template,omitempty"`
	Paths      []string `json:"paths"`
	Tests      []string `json:"tests"`
	Visits     int      `json:"visits"`
}

// UITransition is one distinct observed (from, to, trigger, selector) crossing.
type UITransition struct {
	ID       string   `json:"id"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Trigger  string   `json:"trigger"`
	Selector string   `json:"selector,omitempty"`
	Paths    []string `json:"paths"`
	Tests    []string `json:"tests"`
	Count    int      `json:"count"`
}

// UIEndpoint is one observed request route: method, origin and normalized path. The query,
// headers and bodies are never kept.
type UIEndpoint struct {
	ID       string   `json:"id"`
	Method   string   `json:"method"`
	Origin   string   `json:"origin"`
	Template string   `json:"template"`
	Basis    string   `json:"basis"`
	Statuses []string `json:"statuses"`
	Screens  []string `json:"screens"`
	Unplaced int      `json:"unplaced"`
	Paths    []string `json:"paths"`
	Tests    []string `json:"tests"`
	Count    int      `json:"count"`
}

// UIBinding binds one map test-source element to the screens its observed actions ran on
// (OUE-V0-010). Kind is page-object-method, page-object-file, workflow-method, workflow-file or
// spec.
type UIBinding struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	File       string         `json:"file"`
	Repo       string         `json:"repo,omitempty"`
	Method     string         `json:"method,omitempty"`
	MethodID   string         `json:"method_id,omitempty"`
	Screens    []string       `json:"screens"`
	Paths      []string       `json:"paths"`
	Tests      []string       `json:"tests"`
	Provenance []UIProvenance `json:"provenance"`
}

// UIProvenance is one trace that observed a binding.
type UIProvenance struct {
	TestKey     string `json:"test_key"`
	TraceSHA256 string `json:"trace_sha256"`
	Frame       string `json:"frame"`
}

// UIUnbound is one map page-object, workflow method or spec no observed action placed on a
// known screen. It reads UNKNOWN with a reason, never "no impact" (OUE-V0-011).
type UIUnbound struct {
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Method   string `json:"method,omitempty"`
	MethodID string `json:"method_id,omitempty"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

// UIStructure is the set of scrubbed ARIA structure digests observed on one screen (OUE-V0-040).
type UIStructure struct {
	Screen  string              `json:"screen"`
	Paths   []string            `json:"paths"`
	Digests []UIStructureDigest `json:"digests"`
}

// UIStructureDigest is one scrubbed ARIA snapshot digest, its node count and the tests that saw it.
type UIStructureDigest struct {
	Digest string   `json:"digest"`
	Nodes  int      `json:"nodes"`
	Tests  []string `json:"tests"`
}

// UIDropped counts values read from the trace only to be dropped (OUE-V0-006).
type UIDropped struct {
	Headers     int `json:"headers"`
	Bodies      int `json:"bodies"`
	TypedText   int `json:"typed_text"`
	QueryValues int `json:"query_values"`
	AriaText    int `json:"aria_text"`
	// SelectorValues counts quoted, regex and text= values replaced in action selectors.
	SelectorValues int `json:"selector_values"`
}

var (
	fullCommitID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	networkEntry = regexp.MustCompile(`^([0-9]+-)?trace\.network$`)
	stacksEntry  = regexp.MustCompile(`^([0-9]+-)?trace\.stacks$`)
	idSegment    = regexp.MustCompile(`^([0-9]+|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[0-9a-fA-F]{16,}|[A-Za-z0-9_\-+=.~]{24,})$`)
	staticAsset  = regexp.MustCompile(`(?i)\.(js|mjs|css|map|png|jpe?g|gif|svg|webp|ico|woff2?|ttf|otf|eot)$`)
)

// LoadUIRoutes reads one closed observed-ui-routes/0 file.
func LoadUIRoutes(name string) (*UIRoutes, string, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "--routes %s", msg) }
	raw, ok := readRegular(name, 1<<20)
	if !ok {
		return nil, "", bad("must be a regular file of at most 1 MiB, unchanged while read")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r UIRoutes
	if dec.Decode(&r) != nil || dec.Decode(new(json.RawMessage)) != io.EOF || r.Schema != UIRoutesSchema {
		return nil, "", bad("is not one closed " + UIRoutesSchema + " document")
	}
	if len(r.Screens)+len(r.Endpoints) > maxUIRoutes {
		return nil, "", bad("declares more than 1024 templates")
	}
	for _, list := range [][]UIRoute{r.Screens, r.Endpoints} {
		for _, t := range list {
			if !strings.HasPrefix(t.Template, "/") || len(t.Template) > 1024 || strings.ContainsAny(t.Template, "?#") {
				return nil, "", bad("template must start with / and carry no query or fragment")
			}
			for _, p := range t.Paths {
				if !safeRelative(p) {
					return nil, "", bad("paths must be clean repository-relative paths")
				}
			}
		}
	}
	return &r, digest(raw), nil
}

// uiTemplate is one parsed route template.
type uiTemplate struct {
	route    UIRoute
	segments []string
	splat    bool
	literals int
}

func parseTemplates(list []UIRoute) []uiTemplate {
	out := make([]uiTemplate, 0, len(list))
	for _, r := range list {
		t := uiTemplate{route: r, segments: splitSegments(r.Template)}
		if n := len(t.segments); n > 0 && t.segments[n-1] == "*" {
			t.splat, t.segments = true, t.segments[:n-1]
		}
		for _, s := range t.segments {
			if !templateParam(s) {
				t.literals++
			}
		}
		out = append(out, t)
	}
	return out
}

func templateParam(s string) bool {
	return strings.HasPrefix(s, ":") || (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"))
}

// matchTemplate returns the most literal template matching p; several equally literal matches
// are ambiguous (OUE-V0-004).
func matchTemplate(list []uiTemplate, p string) (*uiTemplate, []string) {
	segs := splitSegments(p)
	var best []*uiTemplate
	for i := range list {
		t := &list[i]
		if len(segs) < len(t.segments) || (!t.splat && len(segs) != len(t.segments)) {
			continue
		}
		ok := true
		for j, s := range t.segments {
			if segs[j] == "" || (!templateParam(s) && s != segs[j]) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		switch {
		case len(best) == 0 || t.literals > best[0].literals:
			best = []*uiTemplate{t}
		case t.literals == best[0].literals:
			best = append(best, t)
		}
	}
	if len(best) == 1 {
		return best[0], nil
	}
	var names []string
	for _, t := range best {
		names = append(names, t.route.Template)
	}
	sort.Strings(names)
	return nil, names
}

// defaultNormalize replaces identifier-shaped path segments (digits, UUIDs, long hex or opaque
// tokens) with `:param`, so a concrete ID or token never becomes a record key (OUE-V0-005).
func defaultNormalize(p string) string {
	segs := splitSegments(p)
	for i, s := range segs {
		if idSegment.MatchString(s) {
			segs[i] = ":param"
		}
	}
	return "/" + strings.Join(segs, "/")
}

type uiResolver struct {
	m       *Map
	x       *screenIndex
	routes  *UIRoutes
	screens []uiTemplate
	ends    []uiTemplate
	byID    map[string]*Screen
}

// screenFor resolves a page URL to a screen record prototype (OUE-V0-003).
func (r *uiResolver) screenFor(raw string) UIScreen {
	if r.x != nil {
		ids, reason := r.x.urlMatches(raw)
		switch {
		case reason == "" && len(ids) == 1:
			return UIScreen{ID: ids[0], Basis: BasisMap, Template: r.byID[ids[0]].Template, Paths: screenPaths(r.byID[ids[0]])}
		case reason == "" && len(ids) > 1:
			return UIScreen{ID: "unknown:ambiguous-template:" + strings.Join(ids, ","), Basis: BasisUnknown, Reason: "ambiguous-template", Candidates: ids}
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return UIScreen{ID: "unknown:non-http-url", Basis: BasisUnknown, Reason: "non-http-url"}
	}
	p := u.Path
	hash := r.routes != nil && r.routes.HashRouting
	if r.m != nil && r.m.HashPrefix != "" {
		hash = true
	}
	if hash {
		frag := strings.TrimPrefix(u.Fragment, "!")
		frag, _, _ = strings.Cut(frag, "?")
		if strings.HasPrefix(frag, "/") {
			p = frag
		}
	}
	if p == "" {
		p = "/"
	}
	if t, amb := matchTemplate(r.screens, p); t != nil {
		return UIScreen{ID: "route:" + t.route.Template, Basis: BasisRoute, Template: t.route.Template, Paths: append([]string{}, t.route.Paths...)}
	} else if len(amb) > 1 {
		return UIScreen{ID: "unknown:ambiguous-route:" + strings.Join(amb, ","), Basis: BasisUnknown, Reason: "ambiguous-route-template", Candidates: amb}
	}
	n := defaultNormalize(p)
	return UIScreen{ID: "url:" + n, Basis: BasisDefault, Template: n, Reason: "no-template", Paths: []string{}}
}

// screenPaths links a map screen to the router source that declares it and the constant tables
// its name or parent was read through. A path from an aliased root is written `@ALIAS/PATH`.
func screenPaths(s *Screen) []string {
	set := map[string]bool{}
	add := func(a Anchor) {
		if a.Path == "" {
			return
		}
		if a.Repo != "" {
			set["@"+a.Repo+"/"+a.Path] = true
			return
		}
		set[a.Path] = true
	}
	add(s.Anchor)
	for _, a := range s.NameFrom {
		add(a)
	}
	for _, a := range s.ParentFrom {
		add(a)
	}
	return sortedKeys(set)
}

// uiFrame is one stack frame of an action's call site.
type uiFrame struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// uiAction is one Playwright API call of a trace.
type uiAction struct {
	call, page, kind, selector, step string
	frames                           []uiFrame
	aria                             *string
}

type uiNav struct {
	ts  float64
	url string
}

// uiTraceData is what one trace zip contributes.
type uiTraceData struct {
	actions    []*uiAction
	beforeURL  map[string]string
	afterURL   map[string]string
	navs       map[string][]uiNav // page ID -> main-frame URL changes in stream order
	requests   []uiRequest
	stepFrames map[string][]uiFrame
	dropped    UIDropped
	// stacksUnavailable names the stacks entries that were not readable stacks documents.
	stacksUnavailable []string
}

type uiRequest struct {
	method, url, page string
	status            int
	ts                float64
	hasTS             bool
}

func openTraceZip(name string) (*zip.Reader, []byte, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s", msg) }
	if fi, err := os.Lstat(name); err == nil && fi.Mode().IsRegular() && fi.Size() > int64(traceZipLimit) {
		return nil, nil, bound("--trace zip exceeds 64 MiB")
	}
	raw, ok := readRegular(name, traceZipLimit)
	if !ok {
		return nil, nil, bad("--trace zip must be a regular file of at most 64 MiB, unchanged while read")
	}
	if bytes.Count(raw, []byte("PK\x01\x02")) > traceZipEntryLimit {
		return nil, nil, bound("--trace zip has more than 65536 entries")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, bad("--trace is not a readable zip")
	}
	if len(zr.File) > traceZipEntryLimit {
		return nil, nil, bound("--trace zip has more than 65536 entries")
	}
	return zr, raw, nil
}

// zipLines streams one bounded NDJSON entry.
func zipLines(f *zip.File, each func([]byte) error) error {
	bad := verifyError("appmap-invalid-query", "trace zip entry %s is unreadable", f.Name)
	over := bound("trace zip entry " + f.Name + " exceeds its byte or line bound")
	if f.UncompressedSize64 > uint64(traceEntryLimit) {
		return over
	}
	rc, err := f.Open()
	if err != nil {
		return bad
	}
	defer rc.Close()
	limited := &io.LimitedReader{R: rc, N: int64(traceEntryLimit) + 1}
	sc := bufio.NewScanner(limited)
	sc.Buffer(make([]byte, 0, min(64<<10, traceLineLimit)), traceLineLimit)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := each(line); err != nil {
			return err
		}
	}
	if errors.Is(sc.Err(), bufio.ErrTooLong) || limited.N <= 0 {
		return over
	}
	if sc.Err() != nil {
		return bad
	}
	return nil
}

// ctxPrefix keys one context's IDs: its entry prefix ("" or "N-") ending in NUL, which no prefix
// otherwise holds, so the keys of two entries never collide whatever their IDs spell.
func ctxPrefix(name, suffix string) string {
	return strings.TrimSuffix(name, suffix) + "\x00"
}

func zipWhole(f *zip.File, limit int) ([]byte, error) {
	bad := verifyError("appmap-invalid-query", "trace zip entry %s is unreadable", f.Name)
	if f.UncompressedSize64 > uint64(limit) {
		return nil, bound("trace zip entry " + f.Name + " exceeds its byte bound")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, bad
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	if err != nil {
		return nil, bad
	}
	if len(raw) > limit {
		return nil, bound("trace zip entry " + f.Name + " exceeds its byte bound")
	}
	return raw, nil
}

// parseTraceStacks decodes one `N-trace.stacks` document into frames by call ID. ok is false when
// the document or any of its pairs is malformed, so a partly read entry never binds.
func parseTraceStacks(raw []byte) (map[string][]uiFrame, bool) {
	var st struct {
		Files  []string            `json:"files"`
		Stacks [][]json.RawMessage `json:"stacks"`
	}
	if json.Unmarshal(raw, &st) != nil || st.Stacks == nil {
		return nil, false
	}
	out := map[string][]uiFrame{}
	for _, pair := range st.Stacks {
		var frames [][]json.RawMessage
		if len(pair) != 2 || json.Unmarshal(pair[1], &frames) != nil {
			return nil, false
		}
		var id string
		if bytes.Equal(bytes.TrimSpace(pair[0]), []byte("null")) {
			return nil, false
		}
		if json.Unmarshal(pair[0], &id) != nil || id == "" {
			var n int
			if json.Unmarshal(pair[0], &n) != nil {
				return nil, false
			}
			id = "call@" + strconv.Itoa(n)
		}
		var fs []uiFrame
		for _, fr := range frames {
			var fi, line int
			if len(fr) < 2 || json.Unmarshal(fr[0], &fi) != nil || json.Unmarshal(fr[1], &line) != nil || fi < 0 || fi >= len(st.Files) {
				continue
			}
			fs = append(fs, uiFrame{File: st.Files[fi], Line: line})
		}
		out[id] = fs
	}
	return out, true
}

func cleanFrames(in []uiFrame) []uiFrame {
	out := []uiFrame{}
	for _, f := range in {
		if f.File == "" || f.Line <= 0 || len(f.File) > 4096 || len(out) == maxUIStackFrames {
			continue
		}
		file := strings.ReplaceAll(strings.TrimPrefix(f.File, "file://"), "\\", "/")
		out = append(out, uiFrame{File: file, Line: f.Line})
	}
	return out
}

// readUITrace mines one trace zip. Values that may hold secrets (typed text, header and body
// content, query values) are counted through key presence only and never decoded (OUE-V0-006).
func readUITrace(zr *zip.Reader) (*uiTraceData, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s", msg) }
	d := &uiTraceData{beforeURL: map[string]string{}, afterURL: map[string]string{}, navs: map[string][]uiNav{}, stepFrames: map[string][]uiFrame{}}
	byCall := map[string]*uiAction{}
	var ctxEntries, netEntries, stackEntries []*zip.File
	var testEntry *zip.File
	for _, f := range zr.File {
		switch {
		case traceEntryName.MatchString(f.Name):
			ctxEntries = append(ctxEntries, f)
		case networkEntry.MatchString(f.Name):
			netEntries = append(netEntries, f)
		case stacksEntry.MatchString(f.Name):
			stackEntries = append(stackEntries, f)
		case f.Name == "test.trace":
			testEntry = f
		}
	}
	if len(ctxEntries) == 0 {
		return nil, bad("trace zip has no context trace (trace.trace or N-trace.trace)")
	}
	if len(ctxEntries) > maxTraceContexts {
		return nil, bad("trace zip holds more than 16 context traces")
	}
	if duplicateEntryName(ctxEntries) {
		return nil, bad("trace zip holds two context traces with one name")
	}
	for _, f := range ctxEntries {
		prefix := ctxPrefix(f.Name, "trace.trace")
		err := zipLines(f, func(line []byte) error {
			var r struct {
				Type     string                     `json:"type"`
				CallID   string                     `json:"callId"`
				Method   string                     `json:"method"`
				PageID   string                     `json:"pageId"`
				StepID   string                     `json:"stepId"`
				Params   map[string]json.RawMessage `json:"params"`
				Stack    []uiFrame                  `json:"stack"`
				Aria     *string                    `json:"ariaSnapshot"`
				Result   json.RawMessage            `json:"result"`
				Snapshot *struct {
					CallID       string   `json:"callId"`
					SnapshotName string   `json:"snapshotName"`
					Phase        string   `json:"phase"`
					FrameURL     string   `json:"frameUrl"`
					PageID       string   `json:"pageId"`
					IsMainFrame  bool     `json:"isMainFrame"`
					Timestamp    *float64 `json:"timestamp"`
				} `json:"snapshot"`
			}
			if json.Unmarshal(line, &r) != nil || r.Type == "" {
				return bad("trace zip context trace has an invalid record")
			}
			call := prefix + r.CallID
			switch {
			case r.Type == "before" && r.CallID != "":
				if !traceMethod.MatchString(r.Method) || byCall[call] != nil {
					return nil
				}
				a := &uiAction{call: call, page: prefix + r.PageID, kind: r.Method, step: r.StepID, frames: cleanFrames(r.Stack)}
				if raw, ok := r.Params["selector"]; ok {
					var s string
					if json.Unmarshal(raw, &s) == nil && len(s) <= 1024 {
						var n int
						a.selector, n = scrubSelector(s)
						d.dropped.SelectorValues += n
					}
				}
				for _, k := range []string{"text", "value", "values", "key", "files", "options", "label"} {
					if _, ok := r.Params[k]; ok {
						d.dropped.TypedText++
					}
				}
				byCall[call] = a
				d.actions = append(d.actions, a)
				// OUE-V0-685: Playwright takes no frame snapshot for a read-only
				// Frame.ariaSnapshot call and writes no pageId on before records,
				// so the call is placed on the last observed main-frame URL of its
				// page, or of the context stream's only page; with several pages
				// it stays unplaced. A snapshot of this call still wins.
				if r.Method == "ariaSnapshot" {
					navs := d.navs[a.page]
					if r.PageID == "" && len(d.navs) == 1 {
						for _, only := range d.navs {
							navs = only
						}
					}
					if len(navs) > 0 {
						d.afterURL[call] = navs[len(navs)-1].url
					}
				}
			case r.Type == "after" && r.CallID != "" && r.Aria != nil:
				if a := byCall[call]; a != nil {
					a.aria = r.Aria
				}
			case r.Type == "after" && r.CallID != "" && len(r.Result) > 0:
				// OUE-V0-685: Playwright writes the YAML only as the result of a
				// Frame.ariaSnapshot call (locator.ariaSnapshot()).
				if a := byCall[call]; a != nil && a.kind == "ariaSnapshot" && a.aria == nil {
					var res struct {
						Snapshot *string `json:"snapshot"`
					}
					if json.Unmarshal(r.Result, &res) == nil && res.Snapshot != nil {
						a.aria = res.Snapshot
					}
				}
			case r.Type == "frame-snapshot" && r.Snapshot != nil && r.Snapshot.IsMainFrame:
				s := r.Snapshot
				if s.FrameURL == "" || len(s.FrameURL) > 4096 {
					return bad("trace zip snapshot URL must be 1 to 4096 bytes")
				}
				phase, sc := snapshotPhase(s.SnapshotName, s.Phase, s.CallID)
				switch phase {
				case "before":
					d.beforeURL[prefix+sc] = s.FrameURL
				case "after", "input":
					d.afterURL[prefix+sc] = s.FrameURL
				}
				page := prefix + s.PageID
				navs := d.navs[page]
				if len(navs) == 0 || navs[len(navs)-1].url != s.FrameURL {
					n := uiNav{url: s.FrameURL, ts: -1}
					if s.Timestamp != nil {
						n.ts = *s.Timestamp
					}
					if len(navs) == maxTraceEvents {
						return bound("trace zip has more than 4096 navigations on one page")
					}
					d.navs[page] = append(navs, n)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, f := range netEntries {
		prefix := ctxPrefix(f.Name, "trace.network")
		err := zipLines(f, func(line []byte) error {
			var r struct {
				Type     string `json:"type"`
				Snapshot *struct {
					PageRef string   `json:"pageref"`
					TS      *float64 `json:"_monotonicTime"`
					Request struct {
						Method   string            `json:"method"`
						URL      string            `json:"url"`
						Headers  []json.RawMessage `json:"headers"`
						PostData json.RawMessage   `json:"postData"`
					} `json:"request"`
					Response struct {
						Status  int               `json:"status"`
						Headers []json.RawMessage `json:"headers"`
						Content json.RawMessage   `json:"content"`
					} `json:"response"`
				} `json:"snapshot"`
			}
			if json.Unmarshal(line, &r) != nil || r.Type == "" {
				return bad("trace zip network trace has an invalid record")
			}
			if r.Type != "resource-snapshot" || r.Snapshot == nil {
				return nil
			}
			s := r.Snapshot
			d.dropped.Headers += len(s.Request.Headers) + len(s.Response.Headers)
			if len(s.Request.PostData) > 0 && string(s.Request.PostData) != "null" {
				d.dropped.Bodies++
			}
			if len(s.Response.Content) > 0 && string(s.Response.Content) != "null" {
				d.dropped.Bodies++
			}
			if len(d.requests) == maxLoggedUIRequests {
				return bad("trace zip has more than 65536 network requests")
			}
			req := uiRequest{method: strings.ToUpper(s.Request.Method), url: s.Request.URL, status: s.Response.Status, page: prefix + s.PageRef}
			if s.TS != nil {
				req.ts, req.hasTS = *s.TS, true
			}
			d.requests = append(d.requests, req)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	// A stacks pair keys its frames by the action's call ID: the string Playwright 1.64 writes
	// (`wcuw@676`, matched as written) or the integer older versions wrote (`call@N`). An entry
	// that is not a stacks document, or holds a malformed pair, contributes no frames and is
	// recorded as stacks-unavailable; the rest of the trace is still read (OUE-V0-013, OUE-V0-014).
	for _, f := range stackEntries {
		prefix := ctxPrefix(f.Name, "trace.stacks")
		raw, err := zipWhole(f, traceLineLimit)
		var ke *gokernel.Error
		if err != nil && errors.As(err, &ke) && ke.Code == "appmap-bound-exceeded" {
			return nil, err
		}
		frames, ok := parseTraceStacks(raw)
		if err != nil || !ok {
			d.stacksUnavailable = append(d.stacksUnavailable, f.Name)
			continue
		}
		for id, fs := range frames {
			if a := byCall[prefix+id]; a != nil && len(a.frames) == 0 {
				a.frames = cleanFrames(fs)
			}
		}
	}
	if testEntry != nil {
		err := zipLines(testEntry, func(line []byte) error {
			var r struct {
				Type   string    `json:"type"`
				CallID string    `json:"callId"`
				Stack  []uiFrame `json:"stack"`
			}
			if json.Unmarshal(line, &r) != nil || r.Type == "" {
				return bad("trace zip test trace has an invalid record")
			}
			if r.Type == "before" && r.CallID != "" && len(r.Stack) > 0 {
				d.stepFrames[r.CallID] = cleanFrames(r.Stack)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		for _, a := range d.actions {
			if len(a.frames) == 0 && a.step != "" {
				a.frames = d.stepFrames[a.step]
			}
		}
	}
	return d, nil
}

const maxLoggedUIRequests = 65536

// pageURLAt is the page's main-frame URL at monotonic time ts: the last change at or before it.
func pageURLAt(navs []uiNav, ts float64) (string, bool) {
	found, out := false, ""
	for _, n := range navs {
		if n.ts < 0 {
			return "", false
		}
		if n.ts <= ts {
			found, out = true, n.url
		}
	}
	return out, found
}

// fileMatch places a frame file on the map test file whose repository path is the longest suffix
// of it; an equal-length tie is not placed (OUE-V0-010).
type fileMatch struct {
	files []*TestFile
}

func (fm fileMatch) place(frame string) *TestFile {
	var best *TestFile
	tie := false
	for _, f := range fm.files {
		if frame != f.Path && !strings.HasSuffix(frame, "/"+f.Path) {
			continue
		}
		switch {
		case best == nil || len(f.Path) > len(best.Path):
			best, tie = f, false
		case len(f.Path) == len(best.Path):
			tie = true
		}
	}
	if tie {
		return nil
	}
	return best
}

// methodAt is the narrowest method of f whose anchor span holds line.
func methodAt(f *TestFile, line int) *Method {
	var best *Method
	for i := range f.Methods {
		m := &f.Methods[i]
		if m.Anchor.Start <= line && line <= m.Anchor.End && (best == nil || m.Anchor.End-m.Anchor.Start < best.Anchor.End-best.Anchor.Start) {
			best = m
		}
	}
	return best
}

// ObserveUIRecords mines trace zips into one observed-ui-records/0 document (OUE-V0-001).
// m and routes are optional; revision must be a full commit ID (OUE-V0-002).
func ObserveUIRecords(m *Map, routes *UIRoutes, routesDigest string, inputs []UITraceInput, revision string) ([]byte, error) {
	bad := func(format string, args ...any) error { return verifyError("appmap-invalid-query", format, args...) }
	if !fullCommitID.MatchString(revision) {
		return nil, bad("--revision must be the full commit ID the traced run was built from")
	}
	if len(inputs) == 0 || len(inputs) > maxTraceFiles {
		return nil, bad("observe --records takes 1 to 64 --trace KEY=FILE.zip inputs")
	}
	r := &uiResolver{m: m, routes: routes, byID: map[string]*Screen{}}
	if m != nil {
		r.x = newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
		for i := range m.Screens {
			r.byID[m.Screens[i].ID] = &m.Screens[i]
		}
	}
	if routes != nil {
		r.screens, r.ends = parseTemplates(routes.Screens), parseTemplates(routes.Endpoints)
	}
	doc := UIRecords{Schema: UIRecordsSchema, Source: TraceZipSource, Evidence: EvidenceLocallyObserved, Authority: AuthorityLearned,
		Revision: revision, RoutesDigest: routesDigest, Traces: []UITrace{}, Screens: []UIScreen{}, Transitions: []UITransition{},
		Endpoints: []UIEndpoint{}, Bindings: []UIBinding{}, Unbound: []UIUnbound{}, Structures: []UIStructure{}, Unknowns: []Unknown{}}
	var fm fileMatch
	if m != nil {
		doc.App, doc.MapDigest = m.App, m.Digest
		for i := range m.Files {
			fm.files = append(fm.files, &m.Files[i])
		}
	} else {
		doc.Unknowns = append(doc.Unknowns, Unknown{Kind: "bindings", Ref: "page-objects", Reason: "no-map: page-object and spec bindings need --map"})
	}
	screens := map[string]*UIScreen{}
	screenTests := map[string]map[string]bool{}
	resolve := func(raw, test string, visit bool) string {
		s := r.screenFor(raw)
		cur := screens[s.ID]
		if cur == nil {
			c := s
			c.Tests = []string{}
			if c.Paths == nil {
				c.Paths = []string{}
			}
			screens[s.ID], screenTests[s.ID] = &c, map[string]bool{}
			cur = &c
		}
		if visit {
			cur.Visits++
			screenTests[s.ID][test] = true
		}
		return s.ID
	}
	transitions := map[string]*UITransition{}
	transitionTests := map[string]map[string]bool{}
	endpoints := map[string]*UIEndpoint{}
	endpointSets := map[string][3]map[string]bool{} // statuses, screens, tests
	type bindAcc struct {
		b       *UIBinding
		screens map[string]bool
		tests   map[string]bool
		prov    map[UIProvenance]bool
		unknown bool
	}
	binds := map[string]*bindAcc{}
	reachedUnknown := map[string]bool{}
	structures := map[string]map[string]*UIStructureDigest{}
	structureTests := map[string]map[string]bool{}
	totalFrames := 0
	seenKeys := map[string]bool{}
	for _, in := range inputs {
		if !validTestKey(in.TestKey) {
			return nil, bad("--trace KEY=FILE.zip needs a test key without spaces or control characters")
		}
		if seenKeys[in.TestKey] {
			return nil, bad("--trace test key %s repeats", in.TestKey)
		}
		seenKeys[in.TestKey] = true
		zr, raw, err := openTraceZip(in.File)
		if err != nil {
			return nil, err
		}
		td, err := readUITrace(zr)
		if err != nil {
			return nil, err
		}
		for _, name := range td.stacksUnavailable {
			doc.Unknowns = append(doc.Unknowns, Unknown{Kind: "stacks", Ref: in.TestKey, Path: name, Reason: "trace-without-stacks: the entry is not a readable stacks document, so its actions carry no stacks frames"})
		}
		tr := UITrace{TestKey: in.TestKey, TraceSHA256: digest(raw), Actions: len(td.actions), Requests: len(td.requests)}
		doc.Dropped.Headers += td.dropped.Headers
		doc.Dropped.Bodies += td.dropped.Bodies
		doc.Dropped.TypedText += td.dropped.TypedText
		// Screens and transitions, page by page in stream order. A change is attributed to the
		// action whose before snapshot held the previous URL and whose input or after snapshot
		// holds the new one (the ANL-V0-022 rule); any other change reads trigger UNKNOWN.
		pages := make([]string, 0, len(td.navs))
		for p := range td.navs {
			pages = append(pages, p)
		}
		sort.Strings(pages)
		usedCalls := map[string]bool{}
		for _, p := range pages {
			navs := td.navs[p]
			prev, prevURL := "", ""
			for _, n := range navs {
				if strings.HasPrefix(n.url, "about:") {
					prev, prevURL = "", ""
					continue
				}
				doc.Dropped.QueryValues += queryValueCount(n.url)
				tr.Navigations++
				id := resolve(n.url, in.TestKey, true)
				if prev != "" && prev != id {
					trigger, selector := StatusUnknown, ""
					for _, a := range td.actions {
						if usedCalls[a.call] || a.page != p || td.beforeURL[a.call] != prevURL || td.afterURL[a.call] != n.url {
							continue
						}
						usedCalls[a.call] = true
						if t, ok := observedTriggers[a.kind]; ok {
							trigger = t
						}
						selector = a.selector
						break
					}
					key := prev + "\x00" + id + "\x00" + trigger + "\x00" + selector
					t := transitions[key]
					if t == nil {
						t = &UITransition{ID: "transition:" + digest([]byte(key))[:16], From: prev, To: id, Trigger: trigger, Selector: selector}
						transitions[key], transitionTests[key] = t, map[string]bool{}
					}
					t.Count++
					transitionTests[key][in.TestKey] = true
				}
				prev, prevURL = id, n.url
			}
		}
		// Endpoints.
		for _, q := range td.requests {
			u, err := url.Parse(q.url)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || staticAsset.MatchString(u.Path) || q.method == "" {
				continue
			}
			doc.Dropped.QueryValues += queryValueCount(q.url)
			p := u.Path
			if p == "" {
				p = "/"
			}
			tmpl, basis, paths := defaultNormalize(p), BasisDefault, []string{}
			if t, amb := matchTemplate(r.ends, p); t != nil {
				tmpl, basis, paths = t.route.Template, BasisRoute, append(paths, t.route.Paths...)
			} else if len(amb) > 1 {
				basis = BasisUnknown
			}
			origin := u.Scheme + "://" + u.Host
			key := q.method + " " + origin + tmpl
			e := endpoints[key]
			if e == nil {
				e = &UIEndpoint{ID: "endpoint:" + digest([]byte(key))[:16], Method: q.method, Origin: origin, Template: tmpl, Basis: basis, Paths: paths}
				endpoints[key] = e
				endpointSets[key] = [3]map[string]bool{{}, {}, {}}
			}
			e.Count++
			sets := endpointSets[key]
			sets[0][statusClass(q.status)] = true
			sets[2][in.TestKey] = true
			if at, ok := pageURLAt(td.navs[q.page], q.ts); ok && q.hasTS && !strings.HasPrefix(at, "about:") {
				sets[1][resolve(at, in.TestKey, false)] = true
			} else {
				e.Unplaced++
			}
		}
		// Bindings and ARIA structure, per action.
		for _, a := range td.actions {
			// An action runs on the screen its before snapshot shows; a goto, or an action with
			// no usable before snapshot, is placed on the screen it lands on.
			at := td.beforeURL[a.call]
			if a.kind == "goto" || at == "" || strings.HasPrefix(at, "about:") {
				at = td.afterURL[a.call]
			}
			screenID, known := "", false
			if at != "" && !strings.HasPrefix(at, "about:") {
				screenID = resolve(at, in.TestKey, false)
				known = screens[screenID].Basis != BasisUnknown
			}
			tr.StackFrames += len(a.frames)
			totalFrames += len(a.frames)
			for _, fr := range a.frames {
				f := fm.place(fr.File)
				if f == nil {
					continue
				}
				tr.MatchedFrames++
				kind, method, methodID := "spec", "", ""
				switch f.Role {
				case rolePageObject, roleWorkflow:
					kind = f.Role + "-file"
					if mm := methodAt(f, fr.Line); mm != nil {
						kind, method, methodID = f.Role+"-method", mm.Name, mm.ID
					}
				case roleSpec:
				default:
					continue
				}
				key := f.Path + "\x00" + methodID
				if !known {
					reachedUnknown[key] = true
					continue
				}
				acc := binds[key]
				if acc == nil {
					acc = &bindAcc{b: &UIBinding{ID: "binding:" + digest([]byte(key))[:16], Kind: kind, File: f.Path, Repo: f.Anchor.Repo, Method: method, MethodID: methodID},
						screens: map[string]bool{}, tests: map[string]bool{}, prov: map[UIProvenance]bool{}}
					binds[key] = acc
				}
				acc.screens[screenID] = true
				acc.tests[in.TestKey] = true
				acc.prov[UIProvenance{TestKey: in.TestKey, TraceSHA256: tr.TraceSHA256, Frame: f.Path + ":" + strconv.Itoa(fr.Line)}] = true
			}
			if a.aria != nil {
				dg, nodes, dropped, ok := ScrubAriaDigest(*a.aria)
				if !ok {
					tr.AriaOversized++
					continue
				}
				tr.AriaSnapshots++
				after := td.afterURL[a.call]
				if after == "" {
					after = at
				}
				doc.Dropped.AriaText += dropped
				sid := "unknown:unplaced-snapshot"
				if after != "" && !strings.HasPrefix(after, "about:") {
					sid = resolve(after, in.TestKey, false)
				}
				if structures[sid] == nil {
					structures[sid], structureTests[sid] = map[string]*UIStructureDigest{}, map[string]bool{}
				}
				sd := structures[sid][dg]
				if sd == nil {
					sd = &UIStructureDigest{Digest: dg, Nodes: nodes, Tests: []string{}}
					structures[sid][dg] = sd
				}
				structureTests[sid+"\x00"+dg] = mergeTest(structureTests[sid+"\x00"+dg], in.TestKey)
			}
		}
		doc.Traces = append(doc.Traces, tr)
	}
	// Assemble in sorted order.
	for id, s := range screens {
		s.Tests = sortedKeys(screenTests[id])
		doc.Screens = append(doc.Screens, *s)
	}
	sort.Slice(doc.Screens, func(i, j int) bool { return doc.Screens[i].ID < doc.Screens[j].ID })
	pathsOf := func(ids ...string) []string {
		set := map[string]bool{}
		for _, id := range ids {
			if s := screens[id]; s != nil {
				for _, p := range s.Paths {
					set[p] = true
				}
			}
		}
		return sortedKeys(set)
	}
	for k, t := range transitions {
		t.Tests, t.Paths = sortedKeys(transitionTests[k]), pathsOf(t.From, t.To)
		doc.Transitions = append(doc.Transitions, *t)
	}
	sort.Slice(doc.Transitions, func(i, j int) bool { return doc.Transitions[i].ID < doc.Transitions[j].ID })
	for k, e := range endpoints {
		sets := endpointSets[k]
		e.Statuses, e.Screens, e.Tests = sortedKeys(sets[0]), sortedKeys(sets[1]), sortedKeys(sets[2])
		doc.Endpoints = append(doc.Endpoints, *e)
	}
	sort.Slice(doc.Endpoints, func(i, j int) bool { return doc.Endpoints[i].ID < doc.Endpoints[j].ID })
	for _, acc := range binds {
		b := acc.b
		b.Screens, b.Tests = sortedKeys(acc.screens), sortedKeys(acc.tests)
		set := map[string]bool{}
		if b.Repo != "" {
			set["@"+b.Repo+"/"+b.File] = true
		} else {
			set[b.File] = true
		}
		for _, p := range pathsOf(b.Screens...) {
			set[p] = true
		}
		b.Paths = sortedKeys(set)
		for p := range acc.prov {
			b.Provenance = append(b.Provenance, p)
		}
		sort.Slice(b.Provenance, func(i, j int) bool {
			x, y := b.Provenance[i], b.Provenance[j]
			if x.TestKey != y.TestKey {
				return x.TestKey < y.TestKey
			}
			if x.TraceSHA256 != y.TraceSHA256 {
				return x.TraceSHA256 < y.TraceSHA256
			}
			return x.Frame < y.Frame
		})
		doc.Bindings = append(doc.Bindings, *b)
	}
	sort.Slice(doc.Bindings, func(i, j int) bool { return doc.Bindings[i].ID < doc.Bindings[j].ID })
	if m != nil {
		reason := func(key string) string {
			switch {
			case totalFrames == 0:
				return "trace-without-stacks"
			case reachedUnknown[key]:
				return "observed-screen-unresolved"
			}
			return "not-observed"
		}
		for i := range m.Files {
			f := &m.Files[i]
			switch f.Role {
			case rolePageObject, roleWorkflow:
				for _, mm := range f.Methods {
					if binds[f.Path+"\x00"+mm.ID] == nil {
						doc.Unbound = append(doc.Unbound, UIUnbound{Kind: f.Role + "-method", File: f.Path, Method: mm.Name, MethodID: mm.ID, Status: StatusUnknown, Reason: reason(f.Path + "\x00" + mm.ID)})
					}
				}
			case roleSpec:
				if binds[f.Path+"\x00"] == nil {
					doc.Unbound = append(doc.Unbound, UIUnbound{Kind: "spec", File: f.Path, Status: StatusUnknown, Reason: reason(f.Path + "\x00")})
				}
			}
		}
		sort.Slice(doc.Unbound, func(i, j int) bool {
			x, y := doc.Unbound[i], doc.Unbound[j]
			if x.File != y.File {
				return x.File < y.File
			}
			return x.MethodID < y.MethodID
		})
	}
	for sid, set := range structures {
		st := UIStructure{Screen: sid, Paths: pathsOf(sid), Digests: []UIStructureDigest{}}
		for dg, sd := range set {
			sd.Tests = sortedKeys(structureTests[sid+"\x00"+dg])
			st.Digests = append(st.Digests, *sd)
		}
		sort.Slice(st.Digests, func(i, j int) bool { return st.Digests[i].Digest < st.Digests[j].Digest })
		doc.Structures = append(doc.Structures, st)
	}
	sort.Slice(doc.Structures, func(i, j int) bool { return doc.Structures[i].Screen < doc.Structures[j].Screen })
	complete, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	doc.Digest = digest(complete)
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	out = append(out, '\n')
	if len(out) > maxUIRecordsBytes {
		return nil, bound("observed UI records exceed 8 MiB; pass fewer traces")
	}
	return out, nil
}

func mergeTest(set map[string]bool, k string) map[string]bool {
	if set == nil {
		set = map[string]bool{}
	}
	set[k] = true
	return set
}

func statusClass(s int) string {
	if s < 100 || s > 599 {
		return "no-response"
	}
	return strconv.Itoa(s/100) + "xx"
}

// queryValueCount counts query values (in the query and in a hash-routed fragment query) that
// the records drop.
func queryValueCount(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	n := 0
	if u.RawQuery != "" {
		n += strings.Count(u.RawQuery, "&") + 1
	}
	if _, q, ok := strings.Cut(u.Fragment, "?"); ok && q != "" {
		n += strings.Count(q, "&") + 1
	}
	return n
}

// LoadUIRecords reads one observed-ui-records/0 document and checks its digest.
func LoadUIRecords(name string) (*UIRecords, error) {
	bad := func(msg string) error { return verifyError("appmap-invalid-query", "%s %s", name, msg) }
	raw, ok := readRegular(name, maxUIRecordsBytes)
	if !ok {
		return nil, bad("must be a regular file of at most 8 MiB, unchanged while read")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r UIRecords
	if dec.Decode(&r) != nil || dec.Decode(new(json.RawMessage)) != io.EOF || r.Schema != UIRecordsSchema || !fullCommitID.MatchString(r.Revision) {
		return nil, bad("is not one closed " + UIRecordsSchema + " document")
	}
	want := r.Digest
	r.Digest = ""
	complete, err := json.Marshal(r)
	if err != nil || digest(complete) != want {
		return nil, bad("digest does not match its content")
	}
	r.Digest = want
	return &r, nil
}

// scrubSelector replaces every value an action selector carries — quoted strings, regex
// literals and unquoted text= values, which can hold accessible names, visible text or query
// values — with a placeholder, keeping the selector's structure (OUE-V0-006).
func scrubSelector(sel string) (string, int) {
	parts := strings.Split(sel, " >> ")
	n := 0
	for i, part := range parts {
		var b strings.Builder
		for j := 0; j < len(part); j++ {
			c := part[j]
			switch {
			case c == '"' || c == '\'':
				k := j + 1
				for k < len(part) && part[k] != c {
					if part[k] == '\\' {
						k++
					}
					k++
				}
				b.WriteString(`"*"`)
				n++
				j = k
			case c == '/' && j > 0 && part[j-1] == '=':
				k := j + 1
				for k < len(part) && part[k] != '/' {
					if part[k] == '\\' {
						k++
					}
					k++
				}
				b.WriteString("/*/")
				n++
				j = k
			default:
				b.WriteByte(c)
			}
		}
		out := b.String()
		for _, prefix := range []string{"text=", "internal:text=", "internal:label=", "internal:has-text="} {
			if rest, ok := strings.CutPrefix(out, prefix); ok && rest != "" && !strings.HasPrefix(rest, `"*"`) && !strings.HasPrefix(rest, "/*/") {
				out = prefix + `"*"`
				n++
			}
		}
		parts[i] = out
	}
	return strings.Join(parts, " >> "), n
}
