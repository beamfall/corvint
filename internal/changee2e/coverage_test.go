package changee2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const teesheetsScreen = "screen:admin:app.clubs.teesheets"

func writeCoverage(t *testing.T, doc string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(p, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func coverageDoc(rev, screens string) string {
	return `{"schema":"e2e-runtime-coverage/0","revision":"` + rev + `","screens":[` + screens + `]}`
}

// CCE-V0-046, CCE-V0-047: runtime coverage only widens reach. A changed path impact cannot place
// stays an unknown span whatever the coverage says, so coverage never turns UNKNOWN into NONE (nor
// into REACHED); a covered changed path adds the obligations of the screen it ran on, with the
// runtime-coverage basis in the chain; coverage naming no changed path adds nothing.
func TestCCEV0046CoverageOnlyWidensReach(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitFiles(t, root, map[string]*string{"server/handler.go": text("package server\n")}, "backend")
	plain, err := BuildObligations(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if plain.State != Unknown || len(plain.Obligations) != 0 || unknownWith(plain, ReasonUnplacedPath, "server/handler.go") == nil ||
		!strings.Contains(strings.Join(plain.Relations.NotConsulted, ","), CoverageBasis) || plain.CoverageDigest != "" {
		t.Fatalf("baseline: %s", plain.Encoded())
	}
	baseline, _ := json.Marshal(plain.Unknown)
	for name, screens := range map[string]string{
		"empty":        ``,
		"other path":   `{"screen":"` + teesheetsScreen + `","paths":["app/app.js"]}`,
		"changed path": `{"screen":"` + teesheetsScreen + `","paths":["server/handler.go","server/handler.go"]}`,
		"every screen": `{"screen":"` + teesheetsScreen + `","paths":["server/handler.go"]},{"screen":"screen:admin:app.home","paths":["server/handler.go"]}`,
		"empty paths":  `{"screen":"screen:admin:app.home","paths":[]}`,
	} {
		o := buildCoverage(t, root, base, head, coverageDoc(head, screens))
		got, _ := json.Marshal(o.Unknown)
		if o.State != Unknown || string(got) != string(baseline) {
			t.Fatalf("%s: coverage changed the unknown spans or the state:\n%s", name, o.Encoded())
		}
		if len(o.CoverageDigest) != 64 || !strings.Contains(strings.Join(o.Relations.Consulted, ","), CoverageBasis) {
			t.Fatalf("%s: coverage not named as consulted: %s", name, o.Encoded())
		}
		widened := strings.Contains(screens, `"server/handler.go"`) && strings.Contains(screens, teesheetsScreen)
		if !widened {
			if len(o.Obligations) != 0 {
				t.Fatalf("%s: coverage naming no changed path reached: %s", name, o.Encoded())
			}
			continue
		}
		var ob *Obligation
		for i := range o.Obligations {
			if o.Obligations[i].ID == "obligation:book-tee-time/booked" {
				ob = &o.Obligations[i]
			}
		}
		if ob == nil || ob.State != Reached || ob.Chain[1] != teesheetsScreen+" (runtime-coverage)" ||
			ob.Chain[0] != "paths:server/handler.go" || len(ob.Spans) != 1 || ob.Spans[0].Path != "server/handler.go" {
			t.Fatalf("%s: covered path did not reach the screen's outcome: %s", name, o.Encoded())
		}
	}
	// A path impact already places keeps impact's chain; coverage adds no second obligation.
	book := commitBehaviourChange(t, root)
	o := buildCoverage(t, root, head, book, coverageDoc(book, `{"screen":"`+teesheetsScreen+`","paths":["app/components/teesheet.html"]}`))
	if o.State != Reached || len(o.Obligations) != 1 || !strings.Contains(o.Obligations[0].Chain[1], "template-reference") {
		t.Fatalf("impact-placed path: %s", o.Encoded())
	}
}

func buildCoverage(t *testing.T, root, base, head, doc string) *Obligations {
	t.Helper()
	opts := diffOpts(root, base, head)
	opts.Coverage = writeCoverage(t, doc)
	o, err := BuildObligations(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// CCE-V0-045: a coverage document is bound to the head revision and the head map; any other
// document refuses `e2e-coverage-invalid`, and compile reads the same coverage as obligations.
func TestCCEV0045CoverageBoundToHead(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitFiles(t, root, map[string]*string{"server/handler.go": text("package server\n")}, "backend")
	ok := `{"screen":"` + teesheetsScreen + `","paths":["server/handler.go"]}`
	for name, doc := range map[string]string{
		"stale revision": coverageDoc(base, ok),
		"wrong schema":   strings.Replace(coverageDoc(head, ok), "/0", "/1", 1),
		"unknown member": strings.Replace(coverageDoc(head, ok), `"screens"`, `"extra":1,"screens"`, 1),
		"unknown screen": coverageDoc(head, `{"screen":"screen:admin:nope","paths":[]}`),
		"repeat screen":  coverageDoc(head, ok+","+ok),
		"escaping path":  coverageDoc(head, `{"screen":"`+teesheetsScreen+`","paths":["../server/handler.go"]}`),
		"unclean path":   coverageDoc(head, `{"screen":"`+teesheetsScreen+`","paths":["server//handler.go"]}`),
		"trailing data":  coverageDoc(head, ok) + "{}",
	} {
		opts := diffOpts(root, base, head)
		opts.Coverage = writeCoverage(t, doc)
		if _, err := BuildObligations(bg, opts); refusalCode(err) != "e2e-coverage-invalid" {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	opts := diffOpts(root, base, head)
	opts.Coverage = filepath.Join(t.TempDir(), "missing.json")
	if _, err := BuildObligations(bg, opts); refusalCode(err) != "e2e-coverage-invalid" {
		t.Fatalf("missing file: %v", err)
	}
	opts.Coverage = writeCoverage(t, coverageDoc(head, ok))
	obs, err := BuildObligations(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compile(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.ObligationsDigest != obs.Digest || r.State != "INCOMPLETE" {
		t.Fatalf("compile did not read the coverage obligations: %s", r.Encoded())
	}
}
