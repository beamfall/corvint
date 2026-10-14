package appmap

import (
	"strings"
	"testing"
)

// TestManifestRegistriesMember is the manifest arm of NGI-V0-007: the closed
// manifest admits a valid `registries` member, which the map does not read,
// and refuses an invalid one with appmap-invalid-manifest.
func TestManifestRegistriesMember(t *testing.T) {
	manifest := `{"schema": "application-map-manifest/0", "app": "shop",
		"routers": [{"path": "src/routes.ts", "dialect": "react-router/0"}],
		"tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]},
		"registries": [REGISTRY]}`
	valid := `{"path": "e2e/data/registry.ts", "export": "registry", "dialect": "string-key-registry/0", "accessors": ["createTestData"]}`
	m, err := decodeManifest([]byte(strings.Replace(manifest, "REGISTRY", valid, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Registries) != 1 || m.Registries[0].Accessors[0] != "createTestData" {
		t.Fatalf("registries = %+v", m.Registries)
	}
	for _, bad := range []string{
		strings.Replace(valid, "string-key-registry/0", "string-key-registry/9", 1),
		strings.Replace(valid, `"e2e/data/registry.ts"`, `"../registry.ts"`, 1),
		strings.Replace(valid, `["createTestData"]`, `[]`, 1),
		strings.Replace(valid, `"accessors"`, `"accessor_names"`, 1),
	} {
		if _, err := decodeManifest([]byte(strings.Replace(manifest, "REGISTRY", bad, 1))); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: err = %v, want appmap-invalid-manifest", bad, err)
		}
	}
}
