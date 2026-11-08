// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"encoding/json"
	"reflect"
	"testing"
)

// CAP-V0-8800: both generated root help and the embedded snapshot expose
// offline MCP discovery; callers cannot mutate the authored inventory.
func TestCAPV08800MCPToolsReachable(t *testing.T) {
	want := MCPTools()
	if len(want) == 0 {
		t.Fatal("empty tool inventory")
	}
	catalog := Build(Registry{})
	encoded, err := Encode(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{encoded, Snapshot()} {
		var decoded Catalog
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decoded.MCPTools, want) {
			t.Fatal("root catalog does not carry the tool inventory")
		}
	}
	catalog.MCPTools[0].Name = "changed"
	if !reflect.DeepEqual(MCPTools(), want) {
		t.Fatal("caller mutated the authored inventory")
	}
}
