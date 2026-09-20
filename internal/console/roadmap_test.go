package console

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const roadmapEnvelope = `{"profile":"taskman-command-result/0","command":["roadmap"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"page":{"offset":"0","limit":"50","total":3,"truncated":false},
"items":[
{"ticketId":"ticket:acme:main:AT-1","title":"First","milestone":"M1","order":"0","owner":"alice","priority":"P1","status":"OPEN","kind":"FEATURE","eligibility":"READY","nextAction":"admit","requiredGates":["GATE_A"],"gateResults":"NOT_OBSERVED"},
{"ticketId":"ticket:acme:main:AT-2","title":"Second","milestone":"M1","order":"1","owner":"bob","priority":"P2","status":"DRAFT","kind":"FEATURE","eligibility":"BLOCKED","nextAction":null,"requiredGates":[],"gateResults":"NOT_OBSERVED"},
{"ticketId":"ticket:acme:main:AT-3","title":"Third","milestone":"","order":"0","owner":"","priority":"P3","status":"OPEN","kind":"CHORE","eligibility":"READY","nextAction":"review","requiredGates":[],"gateResults":"NOT_OBSERVED"}
]}`

const roadmapBlockersEnvelope = `{"profile":"taskman-command-result/0","command":["ticket","blockers"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"items":[{"ticketId":"ticket:acme:main:AT-9","title":"Blocking one","status":"OPEN"}]}`

// TestRoadmapGroupingAndBlockers covers IPR-02: `atm roadmap` items grouped by
// the milestone the tool assigned (an unassigned ticket still renders, in its
// own group), and a BLOCKED ticket's blocker closure fetched and shown. A
// ticket that is not BLOCKED must not trigger a blocker read.
func TestRoadmapGroupingAndBlockers(t *testing.T) {
	binary := stubTaskman(t, map[string]string{"roadmap": roadmapEnvelope, "ticket blockers": roadmapBlockersEnvelope})
	server := newTestServer(t, binary)
	page := get(t, server, "/roadmap")

	for _, want := range []string{
		"M1", "Unassigned",
		"ticket:acme:main:AT-1", "ticket:acme:main:AT-2", "ticket:acme:main:AT-3",
		"ticket:acme:main:AT-9", // the blocker of the BLOCKED ticket
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("roadmap page missing %q:\n%s", want, page)
		}
	}
}

// TestRoadmapGateResultsNotObserved covers IPR-02: a gate result the tool
// reports as NOT_OBSERVED renders as "not observed", never upgraded to a
// pass or fail the tool did not state. All three fixture tickets carry
// "gateResults":"NOT_OBSERVED", so the count must be exact: a per-ticket
// upgrade (one row silently rendered otherwise) would still leave "not
// observed" present from the other two rows, which a bare Contains check
// would miss.
func TestRoadmapGateResultsNotObserved(t *testing.T) {
	binary := stubTaskman(t, map[string]string{"roadmap": roadmapEnvelope, "ticket blockers": roadmapBlockersEnvelope})
	server := newTestServer(t, binary)
	page := get(t, server, "/roadmap")

	if count := strings.Count(page, "not observed"); count != 3 {
		t.Fatalf("expected 3 gate-result cells rendered as \"not observed\", got %d:\n%s", count, page)
	}
	if strings.Contains(page, ">PASS<") || strings.Contains(page, ">FAIL<") {
		t.Fatal("gate results were rendered as pass/fail the tool never reported")
	}
}

// TestRoadmapSafeAutoRecheck covers LAC-V0-032: only the roadmap periodically
// repeats its existing read-only request, and the page states the authority
// decisions that remain hard stops.
func TestRoadmapSafeAutoRecheck(t *testing.T) {
	binary := stubTaskman(t, map[string]string{"roadmap": roadmapEnvelope, "ticket blockers": roadmapBlockersEnvelope, "help ": helpEnvelope, "ticket list": listEnvelope})
	server := newTestServer(t, binary)
	roadmap := get(t, server, "/roadmap?page=2")
	for _, want := range []string{
		`<meta http-equiv="refresh" content="30">`,
		"Safe auto-recheck is active.",
		`href="/roadmap?page=2&amp;refresh=off"`,
		"unknown evidence, admission, release candidacy, attestation, and promotion remain hard stops",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("roadmap auto-recheck contract missing %q:\n%s", want, roadmap)
		}
	}
	board := get(t, server, "/")
	if strings.Contains(board, `http-equiv="refresh"`) {
		t.Fatal("a page with mutation forms must not refresh automatically")
	}
	paused := get(t, server, "/roadmap?page=2&refresh=off")
	if strings.Contains(paused, `http-equiv="refresh"`) || !strings.Contains(paused, "Safe auto-recheck is paused.") || !strings.Contains(paused, `href="/roadmap?page=2"`) {
		t.Fatalf("paused roadmap must remain stable and offer resume:\n%s", paused)
	}
	refused := stubTaskman(t, map[string]string{"roadmap": refusalEnvelope})
	refusalPage := get(t, newTestServer(t, refused), "/roadmap")
	if !strings.Contains(refusalPage, "Safe auto-recheck is active.") || !strings.Contains(refusalPage, "admission, release candidacy, attestation, and promotion remain hard stops") {
		t.Fatalf("refusal page omitted auto-recheck authority boundary:\n%s", refusalPage)
	}
}

// TestRoadmapPageFallback covers the pagination arithmetic in isolation: a
// full page with no stated total still offers "next" (there may be more), a
// short page without a stated total does not, and a stated total governs
// "next" exactly once every row is known to be shown.
func TestRoadmapPageFallback(t *testing.T) {
	full := &Roadmap{Total: roadmapPageSize, Page: 1, PageSize: roadmapPageSize}
	full.applyPage(map[string]any{})
	if !full.HasNext {
		t.Error("a full page with no stated total should still offer a next page")
	}

	short := &Roadmap{Total: 3, Page: 1, PageSize: roadmapPageSize}
	short.applyPage(map[string]any{})
	if short.HasNext {
		t.Error("a short page with no stated total should not offer a next page")
	}

	middle := &Roadmap{Total: 10, Page: 2, PageSize: 10}
	middle.applyPage(map[string]any{"total": float64(25)})
	if !middle.HasNext || !middle.TotalKnown || middle.TotalCount != 25 {
		t.Errorf("middle page: HasNext=%v TotalKnown=%v TotalCount=%d", middle.HasNext, middle.TotalKnown, middle.TotalCount)
	}

	last := &Roadmap{Total: 5, Page: 3, PageSize: 10}
	last.applyPage(map[string]any{"total": float64(25)})
	if last.HasNext {
		t.Error("the last page should not offer a next page once the stated total is reached")
	}
}

// TestRoadmapPageOffsetCannotOverflow covers LAC-V0-029: a page number whose
// offset does not fit an int must not wrap into a negative or unrelated
// `--offset` handed to the tool and labelled with the requested page.
func TestRoadmapPageOffsetCannotOverflow(t *testing.T) {
	binary := stubTaskman(t, map[string]string{"roadmap": roadmapEnvelope, "ticket blockers": roadmapBlockersEnvelope})
	tool := &Taskman{Binary: binary, Repo: t.TempDir()}
	roadmap := tool.ReadRoadmap(context.Background(), math.MaxInt/roadmapPageSize+2)
	offset := ""
	for index, arg := range roadmap.Source.Argv {
		if arg == "--offset" && index+1 < len(roadmap.Source.Argv) {
			offset = roadmap.Source.Argv[index+1]
		}
	}
	parsed, err := strconv.Atoi(offset)
	if err != nil || parsed < 0 || parsed != (roadmap.Page-1)*roadmapPageSize {
		t.Fatalf("page %d read with --offset %q", roadmap.Page, offset)
	}
}

// postMutate submits one /mutate form as the ticket-form or deps-form would.
func postMutate(t *testing.T, server *Server, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	fields.Set("token", server.token)
	r := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(fields.Encode()))
	r.Host = server.host
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)
	return w
}

// setDependenciesHelp declares set-dependencies implemented, so a refusal in
// these tests comes from the ticket store, not from the console's own
// capability gate.
const setDependenciesHelp = `{"profile":"taskman-command-result/0","command":["help"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"items":[{"implemented":["ticket list","ticket show","ticket set-dependencies"],
"omitted":[],"statuses":["DRAFT","OPEN","HELD"],"eligibility":["BLOCKED","UNKNOWN"],"note":"stub"}]}`

// TestSetDependenciesStaleRevisionRefusalKeepsEntries covers IPR-02: a stale
// revision refuses using ATM's real code (STALE_TICKET, not the guessed
// STALE_REVISION) and the console renders it as a refusal that keeps the
// submitted dependency entries and the shown revision, rather than losing them.
func TestSetDependenciesStaleRevisionRefusalKeepsEntries(t *testing.T) {
	refusal := `{"profile":"taskman-command-result/0","command":["ticket","set-dependencies"],"outcome":"REFUSED",
"codes":["STALE_TICKET"],"warnings":["ticket revision has moved since it was read"],"items":[],"untrusted":[]}`
	binary := stubTaskman(t, map[string]string{"help ": setDependenciesHelp, "ticket set-dependencies": refusal})
	server := newTestServer(t, binary)

	w := postMutate(t, server, url.Values{
		"verb": {"set-dependencies"}, "ticket": {"ticket:acme:main:AT-1"}, "expected": {"7"},
		"dependencies": {"ticket:acme:main:AT-2\nticket:acme:main:AT-3"},
		"requestId":    {"r1"}, "issuedAt": {IssuedNow()},
	})
	body := w.Body.String()
	for _, want := range []string{"STALE_TICKET", "ticket:acme:main:AT-2", "ticket:acme:main:AT-3", `name="expected" value="7"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("stale refusal lost %q:\n%s", want, body)
		}
	}
}

// TestSetDependenciesCycleRefusalKeepsEntries covers IPR-02: a dependency
// cycle refuses using ATM's real code (CYCLE, not the guessed
// DEPENDENCY_CYCLE) and keeps the submitted entries.
func TestSetDependenciesCycleRefusalKeepsEntries(t *testing.T) {
	refusal := `{"profile":"taskman-command-result/0","command":["ticket","set-dependencies"],"outcome":"REFUSED",
"codes":["CYCLE"],"warnings":["this dependency would create a cycle"],"items":[],"untrusted":[]}`
	binary := stubTaskman(t, map[string]string{"help ": setDependenciesHelp, "ticket set-dependencies": refusal})
	server := newTestServer(t, binary)

	w := postMutate(t, server, url.Values{
		"verb": {"set-dependencies"}, "ticket": {"ticket:acme:main:AT-1"}, "expected": {"7"},
		"dependencies": {"ticket:acme:main:AT-1"},
		"requestId":    {"r1"}, "issuedAt": {IssuedNow()},
	})
	body := w.Body.String()
	for _, want := range []string{"CYCLE", "ticket:acme:main:AT-1", `name="expected" value="7"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("cycle refusal lost %q:\n%s", want, body)
		}
	}
}

// TestSetDependenciesValidateSkipsTitle covers the console-must-not-name
// obligation: set-dependencies has no title field, so validate must not
// reject a blank one the way refine and create do.
func TestSetDependenciesValidateSkipsTitle(t *testing.T) {
	f := &ticketForm{Verb: "set-dependencies", TicketID: "ticket:acme:main:AT-1", Expected: "3"}
	if err := f.validate(); err != nil {
		t.Fatalf("a valid set-dependencies form was rejected: %v", err)
	}
}

// prioritizeHelp declares prioritize implemented, so a refusal in this test
// comes from the ticket store, not from the console's own capability gate.
const prioritizeHelp = `{"profile":"taskman-command-result/0","command":["help"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"items":[{"implemented":["ticket list","ticket show","ticket prioritize"],
"omitted":[],"statuses":["DRAFT","OPEN","HELD"],"eligibility":["BLOCKED","UNKNOWN"],"note":"stub"}]}`

// TestPrioritizeStaleRevisionRefusalKeepsEntries covers LAC-V0-028: prioritize
// uses a generated order/priority form, not the raw-JSON control shared with
// hold/archive/reopen, so a stale-revision refusal renders using ATM's real
// code and keeps the submitted order and priority entries.
func TestPrioritizeStaleRevisionRefusalKeepsEntries(t *testing.T) {
	refusal := `{"profile":"taskman-command-result/0","command":["ticket","prioritize"],"outcome":"REFUSED",
"codes":["STALE_TICKET"],"warnings":["ticket revision has moved since it was read"],"items":[],"untrusted":[]}`
	binary := stubTaskman(t, map[string]string{"help ": prioritizeHelp, "ticket prioritize": refusal})
	server := newTestServer(t, binary)

	w := postMutate(t, server, url.Values{
		"verb": {"prioritize"}, "ticket": {"ticket:acme:main:AT-1"}, "expected": {"7"},
		"order": {"3"}, "priority": {"P0"},
		"requestId": {"r1"}, "issuedAt": {IssuedNow()},
	})
	body := w.Body.String()
	for _, want := range []string{"STALE_TICKET", `value="3"`, `value="P0"`, `name="expected" value="7"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("stale refusal lost %q:\n%s", want, body)
		}
	}
}

// TestTicketPageOffersOnePrioritizeForm covers LAC-V0-028: the ticket page
// offers exactly one order/priority form, so two submit buttons never race
// the same shown revision.
func TestTicketPageOffersOnePrioritizeForm(t *testing.T) {
	detail := `{"profile":"taskman-command-result/0","outcome":"OK","items":[{"ticketId":"ticket:acme:main:AT-1","title":"Shown title","revision":"7","record":{"body":"Shown body"}}]}`
	binary := stubTaskman(t, map[string]string{"help ": prioritizeHelp, "ticket show": detail})
	server := newTestServer(t, binary)

	ticket := get(t, server, "/ticket?id=ticket:acme:main:AT-1")
	if count := strings.Count(ticket, `name="verb" value="prioritize"`); count != 1 {
		t.Fatalf("ticket page renders %d prioritize forms, want 1:\n%s", count, ticket)
	}
}

// TestPrioritizePayloadCanonicalJSON covers the payload shape LAC-V0-028
// specifies: {"order":...,"priority":...}, both sent as strings the way
// ATM's own template does, not a console-invented enum.
func TestPrioritizePayloadCanonicalJSON(t *testing.T) {
	payload, err := prioritizePayload("3", "P0")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"order":"3","priority":"P0"}`; payload != want {
		t.Fatalf("payload = %s, want %s", payload, want)
	}
}

// TestDependenciesPayloadCanonicalJSON covers the payload shape IPR-02
// specifies: {"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":...}]},
// compact and with sorted keys, which Go's map-to-JSON marshal produces
// without help.
func TestDependenciesPayloadCanonicalJSON(t *testing.T) {
	payload, err := dependenciesPayload([]string{"ticket:acme:main:AT-2"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"ticket:acme:main:AT-2"}]}`
	if payload != want {
		t.Fatalf("payload = %s, want %s", payload, want)
	}
	empty, err := dependenciesPayload(nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty != `{"dependencies":[]}` {
		t.Fatalf("empty payload = %s", empty)
	}
}

// keyboardControlsHelp declares set-dependencies and prioritize implemented,
// so both dependency-editing forms render on the ticket page.
const keyboardControlsHelp = `{"profile":"taskman-command-result/0","command":["help"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"items":[{"implemented":["ticket list","ticket show","ticket set-dependencies","ticket prioritize"],
"omitted":[],"statuses":["DRAFT","OPEN","HELD"],"eligibility":["BLOCKED","UNKNOWN"],"note":"stub"}]}`

// TestRoadmapAndDependencyControlsKeyboardOperable covers LAC-V0-029: every
// roadmap and dependency-editing control must be reachable without a mouse —
// labelled fields and submit buttons only, never a control reachable solely
// by a pointer gesture such as an onclick-only element.
func TestRoadmapAndDependencyControlsKeyboardOperable(t *testing.T) {
	detail := `{"profile":"taskman-command-result/0","outcome":"OK","items":[{"ticketId":"ticket:acme:main:AT-1","title":"Shown title","revision":"7","record":{"body":"Shown body"}}]}`
	paginatedRoadmap := strings.Replace(roadmapEnvelope, `"total":3`, `"total":10`, 1)
	binary := stubTaskman(t, map[string]string{
		"help ": keyboardControlsHelp, "ticket show": detail,
		"roadmap": paginatedRoadmap, "ticket blockers": roadmapBlockersEnvelope,
	})
	server := newTestServer(t, binary)

	roadmap := get(t, server, "/roadmap")
	ticket := get(t, server, "/ticket?id=ticket:acme:main:AT-1")

	for _, page := range []string{roadmap, ticket} {
		lower := strings.ToLower(page)
		for _, banned := range []string{"onclick", "onmousedown", "onmouseup", "onkeypress=", `tabindex="-1"`} {
			if strings.Contains(lower, banned) {
				t.Fatalf("page has a pointer-only or keyboard-trap affordance (%q):\n%s", banned, page)
			}
		}
	}

	if !strings.Contains(roadmap, `<a href="/ticket?id=`) {
		t.Fatal("roadmap ticket links are missing or not plain anchors")
	}
	if !strings.Contains(roadmap, `<a href="/roadmap?page=2">Next page</a>`) {
		t.Fatal("roadmap pagination's next-page control is missing or not a plain anchor")
	}

	for _, want := range []string{
		`<label>Dependencies`, `<label>Order`, `<label>Priority`,
		`<button type="submit">Save dependencies</button>`,
		`<button type="submit">Save order and priority</button>`,
	} {
		if !strings.Contains(ticket, want) {
			t.Fatalf("ticket page missing keyboard-operable control %q:\n%s", want, ticket)
		}
	}
}

// TestDependenciesFormKeepsGateObligation covers LAC-V0-028: set-dependencies
// replaces the whole list, so re-saving the form a ticket page rendered must
// send a GATE_PASSED edge back with its gate, not rewrite it as COMPLETED.
func TestDependenciesFormKeepsGateObligation(t *testing.T) {
	record := []any{
		map[string]any{"ticketId": "ticket:acme:main:AT-2", "obligation": "COMPLETED", "gateId": nil},
		map[string]any{"ticketId": "ticket:acme:main:AT-3", "obligation": "GATE_PASSED", "gateId": "verify"},
	}
	form := &ticketForm{Verb: "set-dependencies", TicketID: "ticket:acme:main:AT-1", Expected: "3", Dependencies: dependenciesText(record)}
	if err := form.validate(); err != nil {
		t.Fatalf("the rendered form was rejected: %v", err)
	}
	payload, err := dependenciesPayload(form.dependencies())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"ticket:acme:main:AT-2"},{"gateId":"verify","obligation":"GATE_PASSED","ticketId":"ticket:acme:main:AT-3"}]}`
	if payload != want {
		t.Fatalf("payload = %s, want %s", payload, want)
	}
	form.Dependencies = "ticket:acme:main:AT-2 verify extra"
	if form.validate() == nil {
		t.Fatal("a line with three fields must be refused, not guessed at")
	}
}
