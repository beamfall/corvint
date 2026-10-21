package doccorpus

import (
	"errors"
	"strings"
	"testing"
)

func TestDCPV1162BoundRefusalNamesBoundScopeAndCount(t *testing.T) {
	t.Parallel()
	// DCP-V1-162: a /2 inventory above 100,000 records names the schema bound, scopes and count.
	scopes := []string{"a", "b"}
	err := checkInventoryBound(Manifest{Schema: ManifestSchemaV2}, scopes, map[string]int{"a": 60000, "b": 40001}, 100001)
	var e *Error
	if !errors.As(err, &e) || e.Code != "corpus-refused" || e.Message != "oversized scope: a, b holds 100001 tracked files, over the corvint-corpus-input/2 bound of 100000 records" {
		t.Fatalf("%v", err)
	}
	if err := checkInventoryBound(Manifest{Schema: ManifestSchemaV2}, scopes, map[string]int{"a": 60000, "b": 40000}, 100000); err != nil {
		t.Fatalf("bound itself refused: %v", err)
	}
	err = checkInventoryBound(Manifest{Schema: ManifestSchema}, scopes, map[string]int{"a": 4000, "b": 97}, 4097)
	if err == nil || !strings.Contains(err.Error(), "bound of 4096 records; --schema 2 admits 100000") {
		t.Fatalf("%v", err)
	}
	if _, err := inventoryScopeSet([]string{"a"}, "corvint-corpus-input/3"); err == nil {
		t.Fatal("unknown schema admitted")
	}
}
