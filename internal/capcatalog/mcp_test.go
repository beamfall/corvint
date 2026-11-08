// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
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

// CAP-V0-8800 / AC2: the offline entries retain tasks-work's role boundary.
func TestCAPV08800TasksWorkRoleConditions(t *testing.T) {
	for _, entry := range MCPTools() {
		if !strings.HasPrefix(entry.Name, "tasks_work_") {
			continue
		}
		if !strings.Contains(entry.Availability, "--tool-profile tasks-work") ||
			!strings.Contains(entry.Availability, "OPERATOR") || !strings.Contains(entry.Availability, "OWNER") {
			t.Fatalf("missing profile or roles: %+v", entry)
		}
		privileged := entry.Name == "tasks_work_close_merged" || entry.Name == "tasks_work_release_sync"
		if strings.Contains(entry.Availability, "AGENT") == privileged {
			t.Fatalf("incorrect agent availability: %+v", entry)
		}
	}
}
