package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// AMAP-V0-212..214: `flows appmap endpoints` writes application-map-endpoints/0 from the map's
// revision; a missing --map is a usage error and no server declaration refuses as a query.
func TestAMAPV0212EndpointsCLI(t *testing.T) {
	root, mapFile := railsCLIRepo(t, true)
	code, out, diag := runFlowsCLI(root, "appmap", "endpoints", "--map", mapFile, "--source", "app", "--rails-routes", "config/routes.rb")
	if code != 0 {
		t.Fatalf("endpoints %d %s", code, diag)
	}
	var doc struct {
		Schema    string `json:"schema"`
		Complete  bool   `json:"complete"`
		Endpoints []struct {
			ID      string `json:"id"`
			Handler string `json:"handler"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.Schema != "application-map-endpoints/0" || !doc.Complete || len(doc.Endpoints) != 2 ||
		doc.Endpoints[0].ID != "GET /orders" || doc.Endpoints[1].Handler != "orders#create" {
		t.Fatalf("%v %s", err, out)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "endpoints", "--source", "app"); code != 2 || !strings.Contains(diag, "--rails-routes [ALIAS:]FILE") {
		t.Fatalf("missing --map: %d %s", code, diag)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "endpoints", "--map", mapFile, "--source", "app"); code != 2 || !strings.Contains(diag, "appmap-invalid-query") {
		t.Fatalf("no declaration: %d %s", code, diag)
	}
	// AMAP-V0-222: an ALIAS:PATH input names a root the map pinned; --repo binds it, and an alias
	// this map never read refuses rather than reading the bound directory's worktree.
	if code, _, diag := runFlowsCLI(root, "appmap", "endpoints", "--map", mapFile, "--source", "app", "--rails-routes", "e2e:config/routes.rb",
		"--repo", "e2e="+root); code == 0 || !strings.Contains(diag, "appmap-root-unavailable") {
		t.Fatalf("unread alias: %d %s", code, diag)
	}
}
