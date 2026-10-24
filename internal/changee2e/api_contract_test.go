package changee2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const slotsContract = `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"/api/slots"}`

func contractDeclarations(contracts string) *string {
	return text(`{"schema":"e2e-block-declarations/0","api_contracts":[` + contracts + `]}` + "\n")
}

// contractRepo commits a page object naming the slots endpoint and a backend handler at base, with
// declarations decl (nil: none), then changes the handler at head.
func contractRepo(t *testing.T, decl *string) (root, base, head string) {
	t.Helper()
	root, _ = fixtureRepo(t)
	src, err := os.ReadFile(filepath.Join(root, "e2e", "pages", "teesheet.page.ts"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(src) + "\n// The tee sheet reads GET /api/slots.\n"
	files := map[string]*string{"e2e/pages/teesheet.page.ts": &page, "server/slots.go": text("package server\n")}
	if decl != nil {
		files[DeclarationsPath] = decl
	}
	base = commitFiles(t, root, files, "contract base")
	head = commitFiles(t, root, map[string]*string{"server/slots.go": text("package server\n\nconst limit = 2\n")}, "backend change")
	return root, base, head
}

func obligationsFor(t *testing.T, o DiffOptions) *Obligations {
	t.Helper()
	r, err := BuildObligations(bg, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// keepsUnknowns fails unless every unknown span of plain is still an unknown span of wide and wide
// still reads UNKNOWN. A span keeps one reason, the highest ranked (oneReasonPerSpan), so a source
// that also lands the path on a screen without an outcome may restate its reason, never drop it.
func keepsUnknowns(t *testing.T, name string, plain, wide *Obligations) {
	t.Helper()
	kept := map[string]bool{}
	for _, u := range wide.Unknown {
		b, _ := json.Marshal(u.Span)
		kept[string(b)] = true
	}
	for _, u := range plain.Unknown {
		if b, _ := json.Marshal(u.Span); !kept[string(b)] {
			t.Fatalf("%s removed the unknown span %s:\n%s", name, b, wide.Encoded())
		}
	}
	if wide.State != Unknown {
		t.Fatalf("%s: state %s, want UNKNOWN:\n%s", name, wide.State, wide.Encoded())
	}
}

// CCE-V0-114: an API contract declared at base joins a changed path matching its glob to each
// screen whose placed head file names the endpoint, with the api-contract basis in the chain. The
// join only widens reach: the changed path's own unknown span stands, a contract the diff itself
// declares joins nothing, an endpoint no placed file names joins nothing, and a malformed contract
// refuses the declarations.
func TestCCEV0114APIContractWidensReach(t *testing.T) {
	root, base, head := contractRepo(t, nil)
	plain := obligationsFor(t, diffOpts(root, base, head))
	if plain.State != Unknown || len(plain.Obligations) != 0 || unknownWith(plain, ReasonUnplacedPath, "server/slots.go") == nil ||
		slices.Contains(plain.Relations.Consulted, APIContractRelation) {
		t.Fatalf("baseline: %s", plain.Encoded())
	}
	root, base, head = contractRepo(t, contractDeclarations(slotsContract))
	wide := obligationsFor(t, diffOpts(root, base, head))
	keepsUnknowns(t, "the API contract join", plain, wide)
	if unknownWith(wide, ReasonUnplacedPath, "server/slots.go") == nil {
		t.Fatalf("the joined path lost its unplaced-path reason: %s", wide.Encoded())
	}
	var ob *Obligation
	for i := range wide.Obligations {
		if wide.Obligations[i].ID == "obligation:book-tee-time/booked" {
			ob = &wide.Obligations[i]
		}
	}
	if ob == nil || ob.State != Reached || ob.Chain[0] != "paths:server/slots.go" ||
		ob.Chain[1] != teesheetsScreen+" (api-contract:contract:slots:e2e/pages/teesheet.page.ts)" ||
		len(ob.Spans) != 1 || ob.Spans[0].Path != "server/slots.go" || !slices.Contains(wide.Relations.Consulted, APIContractRelation) {
		t.Fatalf("the contract did not reach the page object's screen: %s", wide.Encoded())
	}
	// A contract declared only at head joins nothing: a diff cannot widen its own reach.
	root, base, _ = contractRepo(t, nil)
	self := commitFiles(t, root, map[string]*string{DeclarationsPath: contractDeclarations(slotsContract),
		"server/slots.go": text("package server\n\nconst limit = 3\n")}, "self-declared contract")
	if o := obligationsFor(t, diffOpts(root, base, self)); len(o.Obligations) != 0 {
		t.Fatalf("a head-only contract reached: %s", o.Encoded())
	}
	for name, c := range map[string]string{
		"unnamed endpoint": `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"/api/slot"}`,
		"unmatched glob":   `{"contract_id":"contract:slots","paths":["client/*.go"],"endpoint":"/api/slots"}`,
	} {
		root, base, head = contractRepo(t, contractDeclarations(c))
		if o := obligationsFor(t, diffOpts(root, base, head)); len(o.Obligations) != 0 || o.State != Unknown {
			t.Fatalf("%s reached: %s", name, o.Encoded())
		}
	}
	for name, c := range map[string]string{
		"relative endpoint": `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"api/slots"}`,
		"root endpoint":     `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"/"}`,
		"quoted endpoint":   `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"/api/'x"}`,
		"no paths":          `{"contract_id":"contract:slots","paths":[],"endpoint":"/api/slots"}`,
		"bad glob":          `{"contract_id":"contract:slots","paths":["server/["],"endpoint":"/api/slots"}`,
		"absolute glob":     `{"contract_id":"contract:slots","paths":["/server/*.go"],"endpoint":"/api/slots"}`,
		"bad id":            `{"contract_id":"slots","paths":["server/*.go"],"endpoint":"/api/slots"}`,
		"unknown member":    `{"contract_id":"contract:slots","paths":["server/*.go"],"endpoint":"/api/slots","method":"GET"}`,
	} {
		if _, err := decodeDeclarations([]byte(*contractDeclarations(c))); refusalCode(err) != "e2e-blocks-invalid" {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	for data, want := range map[string]bool{"GET /api/slots": true, "'/api/slots/1'": true, "/api/slots?x": true, "/api/slotsx": false, "/api/slots_v2": false, "/api/slot": false} {
		if got := namesEndpoint([]byte(data), "/api/slots"); got != want {
			t.Fatalf("namesEndpoint(%q) = %v", data, got)
		}
	}
}

// CCE-V0-115: no reach source turns UNKNOWN into NONE. With runtime coverage naming the changed
// path on every screen, the module graph and an API contract join all enabled, every unknown span
// of the plain report is still an unknown span and the state reads UNKNOWN; with none of them
// reaching, the report still reads UNKNOWN, never NONE.
func TestCCEV0115NoReachSourceTurnsUnknownToNone(t *testing.T) {
	root, base, head := contractRepo(t, contractDeclarations(slotsContract))
	plainRoot, plainBase, plainHead := contractRepo(t, nil)
	plain := obligationsFor(t, diffOpts(plainRoot, plainBase, plainHead))
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	var every []string
	for _, s := range cat.m.Screens {
		every = append(every, `{"screen":"`+s.ID+`","paths":["server/slots.go"]}`)
	}
	for name, cov := range map[string]string{"every screen": strings.Join(every, ","), "no screen": ``} {
		o := diffOpts(root, base, head)
		o.ModuleGraph = true
		o.Coverage = writeCoverage(t, coverageDoc(head, cov))
		wide := obligationsFor(t, o)
		keepsUnknowns(t, name, plain, wide)
		if !slices.Contains(wide.Relations.Consulted, CoverageBasis) || !slices.Contains(wide.Relations.Consulted, ModuleGraphBasis) ||
			!slices.Contains(wide.Relations.Consulted, APIContractRelation) {
			t.Fatalf("%s: relations %+v", name, wide.Relations)
		}
	}
	none := commitFiles(t, plainRoot, map[string]*string{"server/other.go": text("package server\n")}, "unjoined backend")
	o := diffOpts(plainRoot, plainHead, none)
	o.ModuleGraph = true
	o.Coverage = writeCoverage(t, coverageDoc(none, ``))
	if r := obligationsFor(t, o); r.State != Unknown || unknownWith(r, ReasonUnplacedPath, "server/other.go") == nil {
		t.Fatalf("an unreached backend change: %s", r.Encoded())
	}
}
