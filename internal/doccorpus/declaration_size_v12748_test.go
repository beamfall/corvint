package doccorpus

import (
	"fmt"
	"strings"
	"testing"
)

// TestDCSV1814DerivedDeclarationEncodedSize measures DCS-V1-814 (V1-2748): the encoded bytes one
// derived declaration (its subject and its declaration claim) adds to an artifact, over a
// records-owned Ruby source of 400 methods with realistic names. It logs the per-declaration size
// and its parts and pins the bound the spec records, so an encoding change that grows a
// declaration fails here.
func TestDCSV1814DerivedDeclarationEncodedSize(t *testing.T) {
	t.Parallel()
	const methods = 400
	var source strings.Builder
	source.WriteString("class OrdersController\n")
	for i := 0; i < methods; i++ {
		fmt.Fprintf(&source, "  def sync_order_with_point_of_sale_%04d\n  end\n", i)
	}
	source.WriteString("end\n")
	files := map[string]string{"src/app/controllers/orders_controller.rb": source.String()}
	root, m := ownedSourceCorpus(t, files, []string{"src/app/controllers/orders_controller.rb"}, false)
	a := buildOwned(t, root, m)
	with, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	var subjects, claims, anchors int
	declarations := 0
	for _, s := range subjectsWithPrefix(a, "native:symbol:src/app/controllers/orders_controller.rb:") {
		declarations++
		encoded, err := Encode(s)
		if err != nil {
			t.Fatal(err)
		}
		subjects += len(encoded) + 1
		for _, anchor := range s.Evidence.Anchors {
			encoded, err := Encode(anchor)
			if err != nil {
				t.Fatal(err)
			}
			anchors += len(encoded)
		}
	}
	for _, c := range a.Claims {
		if strings.HasPrefix(c.ID, "native:symbol:src/app/controllers/orders_controller.rb:") {
			encoded, err := Encode(c)
			if err != nil {
				t.Fatal(err)
			}
			claims += len(encoded) + 1
			for _, anchor := range c.Evidence.Anchors {
				encoded, err := Encode(anchor)
				if err != nil {
					t.Fatal(err)
				}
				anchors += len(encoded)
			}
		}
	}
	if declarations < methods {
		t.Fatalf("derived %d declarations, want at least %d", declarations, methods)
	}
	per := (subjects + claims) / declarations
	t.Logf("artifact %d bytes, %d derived declarations: %d bytes each (subject %d, claim %d; anchors %d of them)",
		len(with), declarations, per, subjects/declarations, claims/declarations, anchors/declarations)
	if per > 2048 {
		t.Fatalf("a derived declaration encodes in %d bytes, over the 2,048 the spec records", per)
	}
}
