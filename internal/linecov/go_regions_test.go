// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
)

func TestRSPV0005GoRegionsPhysicalBodiesAndParents(t *testing.T) {
	bodies, err := GoBodies([]byte("package p\n//line fake.go:900\nfunc F() {\n f := func() int { return 1 }; _ = f\n}\n"))
	if err != nil || len(bodies) != 2 {
		t.Fatalf("%+v %v", bodies, err)
	}
	if bodies[0].Range.Start != 3 || bodies[0].Range.StartColumn != 11 || bodies[0].Range.End != 5 || bodies[0].Range.EndColumn != 1 || bodies[1].Parent != bodies[0].ID {
		t.Fatalf("%+v", bodies)
	}
	record := testevidence.New("repo", "test", strings.Repeat("a", 40))
	record.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{{Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: "p.go", Resolved: "p.go"}, OID: strings.Repeat("b", 40)}, Bodies: bodies}}}
	if err := record.ValidateComponents(); err != nil {
		t.Fatal(err)
	}
}
func TestRSPV0005GoRegionsRefuseMalformedOrCappedSource(t *testing.T) {
	for _, raw := range []string{"package p\nfunc F( {", strings.Repeat(" ", 8<<20+1)} {
		if _, err := GoBodies([]byte(raw)); err == nil {
			t.Fatal("invalid regions admitted")
		}
	}
	bodies, err := GoBodies([]byte("package p\nconst N=1\nfunc F() {}\n"))
	if err != nil || len(bodies) != 0 {
		t.Fatalf("declaration or empty body admitted: %+v %v", bodies, err)
	}
}
