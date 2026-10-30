package appmap

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const uiTestRevision = "0123456789abcdef0123456789abcdef01234567"

// writeUITraceZip writes a synthetic Playwright trace zip from entry name to NDJSON lines.
func writeUITraceZip(t *testing.T, entries map[string][]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "trace.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for entry, lines := range entries {
		w, err := zw.Create(entry)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

func uiTestMap() *Map {
	return &Map{
		App: "web", Digest: "mapdigest",
		Screens: []Screen{
			{ID: "screen:login", Status: StatusResolved, Key: "/login", Template: "/login", Anchor: Anchor{Path: "src/router.ts", Start: 3, End: 3}},
			{ID: "screen:user", Status: StatusResolved, Key: "/users/{}", Template: "/users/:id", Anchor: Anchor{Path: "src/router.ts", Start: 4, End: 4}},
		},
		Files: []TestFile{
			{ID: "file:login", Path: "e2e/pages/login.ts", Role: rolePageObject, Methods: []Method{
				{ID: "m:open", Name: "open", Anchor: Anchor{Path: "e2e/pages/login.ts", Start: 3, End: 6}},
				{ID: "m:signIn", Name: "signIn", Anchor: Anchor{Path: "e2e/pages/login.ts", Start: 8, End: 13}},
				{ID: "m:unused", Name: "unused", Anchor: Anchor{Path: "e2e/pages/login.ts", Start: 15, End: 20}},
			}},
			{ID: "file:spec", Path: "e2e/specs/login.spec.ts", Role: roleSpec},
			{ID: "file:other", Path: "e2e/pages/other.ts", Role: rolePageObject, Methods: []Method{
				{ID: "m:other", Name: "other", Anchor: Anchor{Path: "e2e/pages/other.ts", Start: 1, End: 5}},
			}},
		},
	}
}

func uiLoginTrace() map[string][]string {
	return map[string][]string{
		"trace.trace": {
			`{"type":"context-options","version":7}`,
			`{"type":"before","callId":"call@1","pageId":"page@1","method":"goto","params":{"url":"http://app.test/login?token=SECRETQUERY"},"stack":[{"file":"/home/ci/repo/e2e/pages/login.ts","line":5,"column":3}]}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"before@call@1","frameUrl":"about:blank","pageId":"page@1","isMainFrame":true,"timestamp":1}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"after@call@1","frameUrl":"http://app.test/login?token=SECRETQUERY","pageId":"page@1","isMainFrame":true,"timestamp":2}}`,
			`{"type":"before","callId":"call@2","pageId":"page@1","method":"fill","params":{"selector":"#password","value":"hunter2-typed"},"stack":[{"file":"/home/ci/repo/e2e/pages/login.ts","line":9,"column":5}]}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@2","snapshotName":"before@call@2","frameUrl":"http://app.test/login?token=SECRETQUERY","pageId":"page@1","isMainFrame":true,"timestamp":2.5}}`,
			`{"type":"after","callId":"call@2","ariaSnapshot":"- heading \"Welcome Jane\" [level=1]\n- textbox \"Password\": hunter2-typed\n- link \"Home\":\n  - /url: /home?sig=SECRETLINK"}`,
			`{"type":"before","callId":"call@3","pageId":"page@1","method":"click","params":{"selector":"button[type=submit]"},"stack":[{"file":"/home/ci/repo/e2e/pages/login.ts","line":12,"column":5},{"file":"/home/ci/repo/e2e/specs/login.spec.ts","line":4,"column":1}]}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@3","snapshotName":"before@call@3","frameUrl":"http://app.test/login?token=SECRETQUERY","pageId":"page@1","isMainFrame":true,"timestamp":3}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@3","snapshotName":"after@call@3","frameUrl":"http://app.test/users/12345","pageId":"page@1","isMainFrame":true,"timestamp":4}}`,
		},
		"trace.network": {
			`{"type":"resource-snapshot","snapshot":{"pageref":"page@1","_monotonicTime":3.5,"request":{"method":"POST","url":"http://app.test/api/session?code=SECRETCODE","headers":[{"name":"Authorization","value":"Bearer SECRETBEARER"}],"postData":{"text":"{\"password\":\"hunter2-body\"}"}},"response":{"status":201,"headers":[{"name":"Set-Cookie","value":"sid=SECRETCOOKIE"}],"content":{"text":"SECRETBODY"}}}}`,
			`{"type":"resource-snapshot","snapshot":{"pageref":"page@1","_monotonicTime":3.6,"request":{"method":"GET","url":"http://app.test/assets/app.js","headers":[]},"response":{"status":200,"headers":[]}}}`,
			`{"type":"resource-snapshot","snapshot":{"pageref":"page@1","_monotonicTime":4.5,"request":{"method":"GET","url":"http://app.test/api/users/12345","headers":[]},"response":{"status":500,"headers":[]}}}`,
		},
	}
}

func decodeUIRecords(t *testing.T, raw []byte) UIRecords {
	t.Helper()
	var doc UIRecords
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("records do not decode: %v\n%s", err, raw)
	}
	return doc
}

// OUE-V0-001, 003, 006, 007, 010, 011: one trace yields screens, a transition, endpoints and
// page-object bindings; every unbound method and spec reads UNKNOWN; no secret value survives.
func TestObserveUIRecordsBindsAndScrubs(t *testing.T) {
	zipName := writeUITraceZip(t, uiLoginTrace())
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "login.spec.ts:signs-in", File: zipName}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET", "hunter2", "Jane", "Bearer", "/home", "/12345", "Password"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("records leak %q:\n%s", secret, raw)
		}
	}
	doc := decodeUIRecords(t, raw)
	if doc.Evidence != EvidenceLocallyObserved || doc.Authority != AuthorityLearned || doc.Revision != uiTestRevision {
		t.Fatalf("evidence label: %+v", doc)
	}
	if len(doc.Screens) != 2 || doc.Screens[0].ID != "screen:login" || doc.Screens[0].Basis != BasisMap || doc.Screens[1].ID != "screen:user" {
		t.Fatalf("screens: %+v", doc.Screens)
	}
	if len(doc.Transitions) != 1 || doc.Transitions[0].From != "screen:login" || doc.Transitions[0].To != "screen:user" || doc.Transitions[0].Trigger != "click" || doc.Transitions[0].Selector != "button[type=submit]" {
		t.Fatalf("transitions: %+v", doc.Transitions)
	}
	if strings.Join(doc.Transitions[0].Paths, ",") != "src/router.ts" {
		t.Fatalf("transition paths: %+v", doc.Transitions[0].Paths)
	}
	ends := map[string]UIEndpoint{}
	for _, e := range doc.Endpoints {
		ends[e.Method+" "+e.Template] = e
	}
	if len(ends) != 2 {
		t.Fatalf("endpoints (static asset must be skipped): %+v", doc.Endpoints)
	}
	if e := ends["POST /api/session"]; e.Basis != BasisDefault || strings.Join(e.Screens, ",") != "screen:login" || strings.Join(e.Statuses, ",") != "2xx" {
		t.Fatalf("session endpoint: %+v", e)
	}
	if e := ends["GET /api/users/:param"]; strings.Join(e.Screens, ",") != "screen:user" || strings.Join(e.Statuses, ",") != "5xx" {
		t.Fatalf("user endpoint: %+v", e)
	}
	bound := map[string]UIBinding{}
	for _, b := range doc.Bindings {
		bound[b.File+"#"+b.Method] = b
	}
	if b := bound["e2e/pages/login.ts#open"]; b.Kind != "page-object-method" || strings.Join(b.Screens, ",") != "screen:login" || len(b.Provenance) != 1 || b.Provenance[0].Frame != "e2e/pages/login.ts:5" {
		t.Fatalf("open binding: %+v", b)
	}
	if b := bound["e2e/pages/login.ts#signIn"]; strings.Join(b.Screens, ",") != "screen:login" || strings.Join(b.Paths, ",") != "e2e/pages/login.ts,src/router.ts" {
		t.Fatalf("signIn binding: %+v", b)
	}
	if b := bound["e2e/specs/login.spec.ts#"]; b.Kind != "spec" || strings.Join(b.Tests, ",") != "login.spec.ts:signs-in" {
		t.Fatalf("spec binding: %+v", b)
	}
	if len(doc.Unbound) != 2 {
		t.Fatalf("unbound: %+v", doc.Unbound)
	}
	for _, u := range doc.Unbound {
		if u.Status != StatusUnknown || u.Reason != "not-observed" {
			t.Fatalf("unbound must read UNKNOWN not-observed: %+v", u)
		}
	}
	if d := doc.Dropped; d.Headers != 2 || d.Bodies != 2 || d.TypedText != 1 || d.QueryValues < 2 || d.AriaText < 3 {
		t.Fatalf("dropped counts: %+v", d)
	}
	if len(doc.Structures) != 1 || doc.Structures[0].Screen != "screen:login" || doc.Structures[0].Digests[0].Nodes != 3 {
		t.Fatalf("structures: %+v", doc.Structures)
	}
	if doc.Traces[0].MatchedFrames != 4 || doc.Traces[0].AriaSnapshots != 1 {
		t.Fatalf("trace counts: %+v", doc.Traces[0])
	}
	// Byte-identical rerun, and the digest round-trips through LoadUIRecords.
	again, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "login.spec.ts:signs-in", File: zipName}}, uiTestRevision)
	if err != nil || string(again) != string(raw) {
		t.Fatalf("records are not deterministic: %v", err)
	}
	file := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUIRecords(file); err != nil {
		t.Fatalf("LoadUIRecords: %v", err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(raw), `"visits":1`, `"visits":9`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUIRecords(file); err == nil {
		t.Fatal("a tampered record document must be refused")
	}
}

// OUE-V0-010, 011: frames come from trace.stacks and from test.trace by stepId; a trace with no
// stack frames leaves every method UNKNOWN with its reason; without a map, bindings are UNKNOWN.
func TestObserveUIRecordsStackSources(t *testing.T) {
	stacks := map[string][]string{
		"trace.trace": {
			`{"type":"before","callId":"call@7","pageId":"page@1","method":"click","params":{"selector":"#go"}}`,
			`{"type":"before","callId":"call@8","pageId":"page@1","method":"click","stepId":"step@2","params":{"selector":"#other"}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@7","snapshotName":"before@call@7","frameUrl":"http://app.test/login","pageId":"page@1","isMainFrame":true,"timestamp":1}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@8","snapshotName":"before@call@8","frameUrl":"http://app.test/login","pageId":"page@1","isMainFrame":true,"timestamp":2}}`,
		},
		"trace.stacks": {`{"files":["/ci/e2e/pages/login.ts"],"stacks":[[7,[[0,10,3,"signIn"]]]]}`},
		"test.trace":   {`{"type":"before","callId":"step@2","stack":[{"file":"C:\\ci\\e2e\\pages\\other.ts","line":2}]}`},
	}
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "t1", File: writeUITraceZip(t, stacks)}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeUIRecords(t, raw)
	got := map[string]bool{}
	for _, b := range doc.Bindings {
		got[b.Method] = true
	}
	if !got["signIn"] || !got["other"] || len(doc.Bindings) != 2 {
		t.Fatalf("bindings from trace.stacks and test.trace: %+v", doc.Bindings)
	}

	bare := map[string][]string{"trace.trace": {
		`{"type":"before","callId":"call@1","pageId":"page@1","method":"click","params":{"selector":"#go"}}`,
		`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"before@call@1","frameUrl":"http://app.test/login","pageId":"page@1","isMainFrame":true,"timestamp":1}}`,
	}}
	raw, err = ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "t1", File: writeUITraceZip(t, bare)}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc = decodeUIRecords(t, raw)
	if len(doc.Bindings) != 0 || len(doc.Unbound) != 5 {
		t.Fatalf("stackless trace: %+v %+v", doc.Bindings, doc.Unbound)
	}
	for _, u := range doc.Unbound {
		if u.Status != StatusUnknown || u.Reason != "trace-without-stacks" {
			t.Fatalf("stackless unbound: %+v", u)
		}
	}

	raw, err = ObserveUIRecords(nil, nil, "", []UITraceInput{{TestKey: "t1", File: writeUITraceZip(t, uiLoginTrace())}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc = decodeUIRecords(t, raw)
	if len(doc.Bindings) != 0 || len(doc.Unknowns) != 1 || !strings.HasPrefix(doc.Unknowns[0].Reason, "no-map") {
		t.Fatalf("no map: %+v %+v", doc.Bindings, doc.Unknowns)
	}
	if doc.Screens[0].Basis != BasisDefault || doc.Screens[0].ID != "url:/login" || doc.Screens[1].ID != "url:/users/:param" {
		t.Fatalf("default normalization: %+v", doc.Screens)
	}
}

// OUE-V0-003, 004: an observed screen that resolves to an unresolved screen leaves its method
// UNKNOWN with observed-screen-unresolved; route templates normalize and tie as ambiguous.
func TestObserveUIRecordsRoutesAndUnresolved(t *testing.T) {
	dir := t.TempDir()
	routesFile := filepath.Join(dir, "routes.json")
	routesDoc := `{"schema":"observed-ui-routes/0","screens":[{"template":"/orders/:id","paths":["src/orders.tsx"]},{"template":"/orders/new","paths":["src/new.tsx"]},{"template":"/a/:x"},{"template":"/a/{y}"}],"endpoints":[{"template":"/api/*","paths":["api/routes.rb"]}]}`
	if err := os.WriteFile(routesFile, []byte(routesDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	routes, rd, err := LoadUIRoutes(routesFile)
	if err != nil {
		t.Fatal(err)
	}
	trace := map[string][]string{
		"trace.trace": {
			`{"type":"before","callId":"call@1","pageId":"page@1","method":"click","params":{"selector":"#a"},"stack":[{"file":"e2e/pages/other.ts","line":3}]}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"before@call@1","frameUrl":"http://app.test/a/1","pageId":"page@1","isMainFrame":true,"timestamp":1}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"after@call@1","frameUrl":"http://app.test/orders/new","pageId":"page@1","isMainFrame":true,"timestamp":2}}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"after@call@2","frameUrl":"http://app.test/orders/77","pageId":"page@1","isMainFrame":true,"timestamp":3}}`,
		},
		"trace.network": {
			`{"type":"resource-snapshot","snapshot":{"pageref":"page@1","request":{"method":"GET","url":"http://app.test/api/v1/x"},"response":{"status":0}}}`,
		},
	}
	raw, err := ObserveUIRecords(uiTestMap(), routes, rd, []UITraceInput{{TestKey: "t1", File: writeUITraceZip(t, trace)}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeUIRecords(t, raw)
	byID := map[string]UIScreen{}
	for _, s := range doc.Screens {
		byID[s.ID] = s
	}
	if s := byID["route:/orders/new"]; s.Basis != BasisRoute || strings.Join(s.Paths, ",") != "src/new.tsx" {
		t.Fatalf("most literal template must win: %+v", doc.Screens)
	}
	if _, ok := byID["route:/orders/:id"]; !ok {
		t.Fatalf("param template: %+v", doc.Screens)
	}
	if s := byID["unknown:ambiguous-route:/a/:x,/a/{y}"]; s.Basis != BasisUnknown || len(s.Candidates) != 2 {
		t.Fatalf("tied templates must read UNKNOWN: %+v", doc.Screens)
	}
	for _, u := range doc.Unbound {
		if u.Method == "other" && u.Reason != "observed-screen-unresolved" {
			t.Fatalf("method on an unresolved screen: %+v", u)
		}
	}
	if len(doc.Endpoints) != 1 || doc.Endpoints[0].Template != "/api/*" || doc.Endpoints[0].Unplaced != 1 || doc.Endpoints[0].Statuses[0] != "no-response" {
		t.Fatalf("endpoint route: %+v", doc.Endpoints)
	}
}

func TestObserveUIRecordsRefusals(t *testing.T) {
	good := writeUITraceZip(t, uiLoginTrace())
	for name, c := range map[string]struct {
		rev    string
		inputs []UITraceInput
	}{
		"short revision": {"abc123", []UITraceInput{{TestKey: "t", File: good}}},
		"no traces":      {uiTestRevision, nil},
		"bad key":        {uiTestRevision, []UITraceInput{{TestKey: "a b", File: good}}},
		"repeated key":   {uiTestRevision, []UITraceInput{{TestKey: "t", File: good}, {TestKey: "t", File: good}}},
		"no context":     {uiTestRevision, []UITraceInput{{TestKey: "t", File: writeUITraceZip(t, map[string][]string{"x.txt": {"{}"}})}}},
		"not a zip":      {uiTestRevision, []UITraceInput{{TestKey: "t", File: "uirecords.go"}}},
	} {
		if _, err := ObserveUIRecords(nil, nil, "", c.inputs, c.rev); err == nil {
			t.Fatalf("%s: want refusal", name)
		}
	} // OUE-V0-009: an input over a byte or line bound refuses as a bound, not as a bad query.
	defer func(a, b int) { traceEntryLimit, traceLineLimit = a, b }(traceEntryLimit, traceLineLimit)
	for _, lim := range [][2]int{{256, 16 << 20}, {64 << 20, 128}} {
		traceEntryLimit, traceLineLimit = lim[0], lim[1]
		if _, err := ObserveUIRecords(nil, nil, "", []UITraceInput{{TestKey: "t", File: good}}, uiTestRevision); codeOf(err) != "appmap-bound-exceeded" {
			t.Fatalf("limits %v: got %v", lim, err)
		}
	}
}

// OUE-V0-040, 041: scrubbing keeps role, depth and state attributes and drops every name,
// text and property value, so a renamed label keeps its digest and a new node changes it.
func TestScrubAriaDigest(t *testing.T) {
	a, n, dropped, _ := ScrubAriaDigest("- heading \"Hello Jane\" [level=1]\n- list:\n  - listitem: Secret text\n  - link \"x [y]\" [disabled]:\n    - /url: /secret")
	if n != 4 || dropped != 4 {
		t.Fatalf("nodes %d dropped %d", n, dropped)
	}
	b, _, _, _ := ScrubAriaDigest("- heading \"Bonjour Marie\" [level=1]\n- list:\n  - listitem: autre\n  - link \"z\" [disabled]:\n    - /url: /other")
	if a != b {
		t.Fatal("text and name changes must not change the structure digest")
	}
	c, _, _, _ := ScrubAriaDigest("- heading \"Hello Jane\" [level=2]\n- list:\n  - listitem: Secret text\n  - link \"x\" [disabled]")
	if a == c {
		t.Fatal("a level change must change the structure digest")
	}
	// A snapshot over the line bound yields no digest rather than a digest of its prefix.
	if _, _, _, ok := ScrubAriaDigest(strings.Repeat("- listitem\n", maxAriaLines)); ok {
		t.Fatal("an oversized snapshot must not digest")
	}
}

// OUE-V0-006: selector values (names, text, attribute and regex values) never reach output.
func TestScrubSelector(t *testing.T) {
	for in, want := range map[string]string{
		`internal:role=button[name="Jane Smith"s]`: `internal:role=button[name="*"s]`,
		`a[href='/x?token=1'] >> text=Jane Smith`:  `a[href="*"] >> text="*"`,
		`internal:text=/Welcome \/ Jane/i`:         `internal:text=/*/i`,
		`internal:label="Email \"work\"" >> nth=0`: `internal:label="*" >> nth=0`,
		`button[type=submit]`:                      `button[type=submit]`,
	} {
		if got, _ := scrubSelector(in); got != want {
			t.Fatalf("%s: got %s want %s", in, got, want)
		}
	}
}

// OUE-V0-007: only distinct screens make a transition, and each crossing keeps its own action.
func TestObserveUIRecordsTransitionAttribution(t *testing.T) {
	snap := func(call, name, url string, ts int) string {
		return `{"type":"frame-snapshot","snapshot":{"callId":"` + call + `","snapshotName":"` + name + `@` + call + `","frameUrl":"` + url + `","pageId":"page@1","isMainFrame":true,"timestamp":` + strconv.Itoa(ts) + `}}`
	}
	act := func(call, method, sel string) string {
		return `{"type":"before","callId":"` + call + `","pageId":"page@1","method":"` + method + `","params":{"selector":"` + sel + `"}}`
	}
	login, user1, user2 := "http://app.test/login", "http://app.test/users/1", "http://app.test/users/2"
	trace := map[string][]string{"trace.trace": {
		act("call@1", "click", "#a"), snap("call@1", "before", login, 1), snap("call@1", "after", user1, 2),
		act("call@2", "click", "#next"), snap("call@2", "before", user1, 3), snap("call@2", "after", user2, 4),
		act("call@3", "click", "#back"), snap("call@3", "before", user2, 5), snap("call@3", "after", login, 6),
		act("call@4", "press", "#a"), snap("call@4", "before", login, 7), snap("call@4", "after", user1, 8),
	}}
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "t", File: writeUITraceZip(t, trace)}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeUIRecords(t, raw)
	got := []string{}
	for _, tr := range doc.Transitions {
		if tr.From == tr.To {
			t.Fatalf("self transition: %+v", tr)
		}
		got = append(got, tr.From+">"+tr.To+":"+tr.Trigger+":"+strconv.Itoa(tr.Count))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "screen:login>screen:user:click:1,screen:login>screen:user:key press:1,screen:user>screen:login:click:1" {
		t.Fatalf("transitions: %v", got)
	}
}

func TestDiffUIStructures(t *testing.T) {
	base := &UIRecords{Revision: uiTestRevision, Traces: []UITrace{{AriaSnapshots: 1}}, Structures: []UIStructure{
		{Screen: "screen:a", Digests: []UIStructureDigest{{Digest: "d1"}}},
		{Screen: "screen:b", Digests: []UIStructureDigest{{Digest: "d2"}}},
		{Screen: "screen:c", Digests: []UIStructureDigest{{Digest: "d3"}}},
	}}
	head := &UIRecords{Revision: uiTestRevision, Traces: []UITrace{{}}, Structures: []UIStructure{
		{Screen: "screen:a", Digests: []UIStructureDigest{{Digest: "d1"}}},
		{Screen: "screen:b", Digests: []UIStructureDigest{{Digest: "d9", Tests: []string{"t"}}}},
		{Screen: "unknown:unplaced-snapshot", Digests: []UIStructureDigest{{Digest: "d4"}}},
	}}
	raw, err := DiffUIStructures(base, head)
	if err != nil {
		t.Fatal(err)
	}
	var d UIStructureDiff
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Signal != SignalCandidate || strings.Contains(string(raw), "verdict") || strings.Contains(string(raw), "pass") {
		t.Fatalf("diff must be a candidate signal only: %s", raw)
	}
	states := map[string]string{}
	for _, s := range d.Screens {
		states[s.Screen] = s.State + "/" + s.Reason
	}
	want := map[string]string{"screen:a": "same-structure/", "screen:b": "structure-changed/", "screen:c": "UNKNOWN/observed-one-side", "unknown:unplaced-snapshot": "UNKNOWN/screen-unresolved"}
	for k, v := range want {
		if states[k] != v {
			t.Fatalf("%s: got %q want %q", k, states[k], v)
		}
	}
	if c := d.Counts; c.Screens != 4 || c.Changed != 1 || c.Same != 1 || c.Unknown != 2 || c.HeadNoAria != 1 || c.BaseNoAria != 0 {
		t.Fatalf("counts: %+v", c)
	}
}

// OUE-V0-013, 014: a Playwright 1.64-shaped multi-context trace keys its stacks pairs by string
// call ID (matched as written, with the entry prefix) and binds through them; a malformed stacks
// entry is recorded as stacks-unavailable while the trace's screens, transitions and endpoints
// are still read.
func TestObserveUIRecordsPlaywright164Stacks(t *testing.T) {
	in := []UITraceInput{{TestKey: "login.spec.ts:two-users", File: traceZip(t, pw164Entries(t, nil))}}
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", in, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeUIRecords(t, raw)
	got := map[string]bool{}
	for _, b := range doc.Bindings {
		got[b.Kind+":"+b.Method] = true
	}
	if !got["page-object-method:open"] || !got["page-object-method:signIn"] || !got["spec:"] || len(doc.Unknowns) != 0 {
		t.Fatalf("string-id bindings: %+v unknowns %+v", doc.Bindings, doc.Unknowns)
	}
	if tr := doc.Traces[0]; tr.Actions != 5 || tr.StackFrames == 0 || tr.Navigations != 4 || len(doc.Transitions) == 0 || len(doc.Endpoints) == 0 {
		t.Fatalf("multi-context trace: %+v transitions %+v endpoints %+v", tr, doc.Transitions, doc.Endpoints)
	}

	broken := map[string]string{"0-trace.stacks": "not json", "1-trace.stacks": `{"files":[],"stacks":[[{"id":1},[]]]}`, "2-trace.stacks": `{"files":["/a.ts"],"stacks":[["wcuw@690"]]}`}
	in[0].File = traceZip(t, pw164Entries(t, broken))
	raw, err = ObserveUIRecords(uiTestMap(), nil, "", in, uiTestRevision)
	if err != nil {
		t.Fatalf("a malformed stacks entry discarded the trace: %v", err)
	}
	doc = decodeUIRecords(t, raw)
	if len(doc.Unknowns) != 3 || len(doc.Bindings) != 0 || len(doc.Transitions) == 0 || len(doc.Endpoints) == 0 || len(doc.Screens) == 0 {
		t.Fatalf("malformed stacks: unknowns %+v bindings %+v", doc.Unknowns, doc.Bindings)
	}
	for _, u := range doc.Unknowns {
		if u.Kind != "stacks" || u.Ref != in[0].TestKey || !strings.HasPrefix(u.Reason, "trace-without-stacks") || !strings.HasSuffix(u.Path, "-trace.stacks") {
			t.Fatalf("stacks unknown: %+v", u)
		}
	}
	for _, u := range doc.Unbound {
		if u.Reason != "trace-without-stacks" {
			t.Fatalf("unbound with no readable stacks: %+v", u)
		}
	}

	// A null document or null call ID is malformed, and so is an entry whose bytes fail to inflate
	// or fail their CRC; each still leaves the trace read.
	nulls := map[string]string{"0-trace.stacks": "null", "1-trace.stacks": `{"files":["/ci/repo/e2e/pages/login.ts"],"stacks":[[null,[[0,4,11,"x"]]]]}`}
	in[0].File = corruptZipEntry(t, traceZip(t, pw164Entries(t, nulls)), "2-trace.stacks")
	raw, err = ObserveUIRecords(uiTestMap(), nil, "", in, uiTestRevision)
	if err != nil {
		t.Fatalf("an unreadable stacks entry discarded the trace: %v", err)
	}
	doc = decodeUIRecords(t, raw)
	if len(doc.Unknowns) != 3 || len(doc.Bindings) != 0 || len(doc.Transitions) == 0 {
		t.Fatalf("null and corrupt stacks: unknowns %+v bindings %+v", doc.Unknowns, doc.Bindings)
	}

	// The integer form older versions wrote still binds by `call@N`, beside a string-keyed entry.
	mixed := map[string]string{"2-trace.stacks": `{"files":["/ci/repo/e2e/pages/login.ts"],"stacks":[[692,[[0,12,11,"x"]]]]}`, "2-trace.trace": strings.ReplaceAll(pw164Entries(t, nil)["2-trace.trace"], "wcuw@692", "call@692")}
	in[0].File = traceZip(t, pw164Entries(t, mixed))
	raw, err = ObserveUIRecords(uiTestMap(), nil, "", in, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	doc = decodeUIRecords(t, raw)
	n := 0
	for _, b := range doc.Bindings {
		if b.Method == "signIn" {
			n = len(b.Provenance)
		}
	}
	if n != 2 || len(doc.Unknowns) != 0 {
		t.Fatalf("integer stacks id beside string ids: %+v", doc.Bindings)
	}
}

// corruptZipEntry flips one stored byte of the named entry, so reading it fails.
func corruptZipEntry(t *testing.T, path, name string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			off, err := f.DataOffset()
			if err != nil || f.CompressedSize64 < 2 {
				t.Fatal("entry offset", err)
			}
			b[off+int64(f.CompressedSize64)/2] ^= 0xff
			if err = os.WriteFile(path, b, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}
	}
	t.Fatal("no entry", name)
	return ""
}
