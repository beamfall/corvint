package sessiontest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// STR-V0-069: a Go unit the caller excludes from the root universe is
// recorded sorted with its reason in the manifest, the manifest binds it and
// reads back strictly, and a manifest with nothing excluded has no key.
func TestSTRV0069ExcludedUnitsAreRecorded(t *testing.T) {
	input := fixtureInput()
	plain, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(plain)
	if err != nil || strings.Contains(string(encoded), `"excluded"`) {
		t.Fatalf("a manifest with nothing excluded carries the key: %s %v", encoded, err)
	}
	input.Excluded = []string{"go:example.test/z: second", "go:example.test/nested: a dependent nested module"}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go:example.test/nested: a dependent nested module", "go:example.test/z: second"}
	if !reflect.DeepEqual(m.Excluded, want) || !reflect.DeepEqual(m.Units, plain.Units) || m.Scope != plain.Scope {
		t.Fatalf("excluded %q units %+v scope %s", m.Excluded, m.Units, m.Scope)
	}
	first, _ := Digest(plain)
	second, _ := Digest(m)
	if first == second {
		t.Fatal("the manifest does not bind its exclusions")
	}
	encoded, err = Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(name, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	read, digest, err := ReadManifest(name)
	if err != nil || digest != second || !reflect.DeepEqual(read.Excluded, want) {
		t.Fatalf("read back %q %s %v", read.Excluded, digest, err)
	}
	// Each shard receipt carries the list too; one with nothing excluded has no key.
	if receipt := NewReceipt(m, second, 0, "test"); !reflect.DeepEqual(receipt.Excluded, want) {
		t.Fatalf("receipt excluded %q", receipt.Excluded)
	}
	if encoded, err := Encode(NewReceipt(plain, first, 0, "test")); err != nil || strings.Contains(string(encoded), `"excluded"`) {
		t.Fatalf("a receipt with nothing excluded carries the key: %v", err)
	}
}
