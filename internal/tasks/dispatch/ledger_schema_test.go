//go:build darwin || linux

package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ledgerSchemas pins the encoded shape of each dispatcher ledger version
// this build has written (CAL-V0-131): a member or nested field added,
// removed or retyped changes the digest, so the change must also move
// StateProfile to a new version and pin it here, never repin an old one.
var ledgerSchemas = map[string]string{
	"taskman-dispatch-state/0": "78ada6b29ff738fa1dd2ed230742bd645204431325983cbc6fc06682391b1bb0",
	"taskman-dispatch-state/1": "fcc36ae74c3b4a2c4824ac58610c68bd0c232cb819298db263068a07e95438b9",
	"taskman-dispatch-state/2": "8d851eaa19d55f5dc4727701b1f1ce829fde819762e22224a013a647c12a263d",
	"taskman-dispatch-state/3": "64cfe28f71ae06f3f8ada72fdb692bba84c928437e371729ade9a987186c6f0e",
}

// encodedSchema renders t's JSON shape: every encoded field's name, options
// and type, recursively, in declaration order.
func encodedSchema(t reflect.Type, seen map[reflect.Type]bool) string {
	switch {
	case t == reflect.TypeFor[time.Time]():
		return "time"
	case t.Kind() == reflect.Pointer:
		return "*" + encodedSchema(t.Elem(), seen)
	case t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		return "[]" + encodedSchema(t.Elem(), seen)
	case t.Kind() == reflect.Map:
		return "map[" + encodedSchema(t.Key(), seen) + "]" + encodedSchema(t.Elem(), seen)
	case t.Kind() != reflect.Struct:
		return t.Kind().String()
	case seen[t]:
		return t.Name()
	}
	seen[t] = true
	var b strings.Builder
	b.WriteString(t.Name() + "{")
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if !f.IsExported() || tag == "-" {
			continue
		}
		fmt.Fprintf(&b, "%s:%s;", tag, encodedSchema(f.Type, seen))
	}
	return b.String() + "}"
}

func TestCALV0131_LedgerSchemaChangeMovesTheStateVersion(t *testing.T) {
	sum := sha256.Sum256([]byte(encodedSchema(reflect.TypeFor[Ledger](), map[reflect.Type]bool{})))
	got := hex.EncodeToString(sum[:])
	if os.Getenv("PRINT_LEDGER_SCHEMA") != "" {
		t.Logf("%s %s", StateProfile, got)
	}
	want, ok := ledgerSchemas[StateProfile]
	if !ok || want != got {
		t.Fatalf("%s encodes schema %s, pinned %q: a changed ledger encoding needs a new taskman-dispatch-state version (CAL-V0-131)", StateProfile, got, want)
	}
}
