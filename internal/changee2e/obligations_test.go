package changee2e

import (
	"strings"
	"testing"
)

// CCE-V0-004: a one-screen diff reaches exactly one outcome with its chain.
func TestCCEV0004OneScreenDiffReachesOneOutcome(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	o, err := BuildObligations(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if o.State != Reached || len(o.Obligations) != 1 || len(o.Unknown) != 0 {
		t.Fatalf("%s", o.Encoded())
	}
	ob := o.Obligations[0]
	want := []string{"paths:app/components/teesheet.html", "screen:admin:app.clubs.teesheets (template-reference:app/routes.js)",
		"step:book-tee-time/book", "outcome:book-tee-time/booked"}
	if ob.ID != "obligation:book-tee-time/booked" || ob.State != Reached || strings.Join(ob.Chain, "|") != strings.Join(want, "|") {
		t.Fatalf("%+v", ob)
	}
	if len(ob.Spans) != 1 || ob.Spans[0].Path != "app/components/teesheet.html" || ob.Spans[0].Start != 6 || ob.Spans[0].End != 6 {
		t.Fatalf("spans %+v", ob.Spans)
	}
}

// CCE-V0-004: a diff impact cannot place reads UNKNOWN naming its spans; a non-UI diff reads NONE.
func TestCCEV0004UnboundedUnknownAndNonUINone(t *testing.T) {
	root, base := fixtureRepo(t)
	docs := commitFiles(t, root, map[string]*string{"README.md": text("docs\n")}, "docs")
	o, err := BuildObligations(bg, diffOpts(root, base, docs))
	if err != nil {
		t.Fatal(err)
	}
	if o.State != None || len(o.Obligations) != 0 || len(o.Unknown) != 0 {
		t.Fatalf("non-UI diff: %s", o.Encoded())
	}
	orphan := commitFiles(t, root, map[string]*string{"app/shared/orphan.html": text("<p>a</p>\n<p>b</p>\n")}, "orphan")
	o, err = BuildObligations(bg, diffOpts(root, docs, orphan))
	if err != nil {
		t.Fatal(err)
	}
	if o.State != Unknown || len(o.Unknown) != 1 || o.Unknown[0].Span.Path != "app/shared/orphan.html" || o.Unknown[0].Span.Start != 1 ||
		o.Unknown[0].Span.End != 2 || o.Unknown[0].Reason == "" {
		t.Fatalf("unbounded diff: %s", o.Encoded())
	}
}
