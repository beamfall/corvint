package sessiontest

import (
	"reflect"
	"testing"
)

// STR-V0-004: every adapter the planner emits ships a versioned capability
// manifest stating exactly the discover, plan, execute and cancel subset of
// LPCV-V0-021. The version and operations are pinned here, so a removed,
// renamed or re-versioned manifest fails this test.
func TestSTRV0004AdapterCapabilityManifest(t *testing.T) {
	want := map[string]AdapterManifest{
		"go":       {Adapter: "go", Operations: []string{"cancel", "discover", "execute", "plan"}, Profile: "corvint-test-adapter/0"},
		"declared": {Adapter: "declared", Operations: []string{"cancel", "discover", "execute", "plan"}, Profile: "corvint-test-adapter/0"},
	}
	manifests := AdapterManifests()
	if !reflect.DeepEqual(manifests, want) {
		t.Fatalf("adapter manifests %+v, want %+v", manifests, want)
	}
	for _, adapter := range []string{AdapterGo, AdapterDeclared} {
		if reason := adapterRefusal(manifests, adapter); reason != "" {
			t.Fatalf("registered adapter %s refused: %s", adapter, reason)
		}
	}
	// The registry is a fresh copy: a caller's edit cannot weaken it.
	manifests[AdapterGo] = AdapterManifest{}
	if AdapterManifests()[AdapterGo].Profile != AdapterManifestProfile {
		t.Fatal("AdapterManifests shares state with its callers")
	}
}

// STR-V0-004 with LPCV-V0-021: an absent, mismatched or incomplete manifest
// is refused; capability absence is never read as support.
func TestSTRV0004AdapterManifestRefusals(t *testing.T) {
	good := AdapterManifests()[AdapterGo]
	with := func(change func(*AdapterManifest)) map[string]AdapterManifest {
		manifest := good
		manifest.Operations = append([]string{}, good.Operations...)
		change(&manifest)
		return map[string]AdapterManifest{AdapterGo: manifest}
	}
	for name, test := range map[string]struct {
		manifests map[string]AdapterManifest
		reason    string
	}{
		"absent":        {map[string]AdapterManifest{}, "go: no registered capability manifest; nothing launched for it"},
		"other version": {with(func(m *AdapterManifest) { m.Profile = "corvint-test-adapter/1" }), `go: capability manifest "corvint-test-adapter/1" is not corvint-test-adapter/0; nothing launched for it`},
		"no version":    {with(func(m *AdapterManifest) { m.Profile = "" }), `go: capability manifest "" is not corvint-test-adapter/0; nothing launched for it`},
		"other adapter": {with(func(m *AdapterManifest) { m.Adapter = AdapterDeclared }), `go: its capability manifest names adapter "declared"; nothing launched for it`},
		"no cancel":     {with(func(m *AdapterManifest) { m.Operations = []string{"discover", "execute", "plan"} }), "go: its capability manifest does not state cancel; nothing launched for it"},
		"no operations": {with(func(m *AdapterManifest) { m.Operations = nil }), "go: its capability manifest does not state cancel; nothing launched for it"},
		"unknown":       {with(func(m *AdapterManifest) { m.Operations = append(m.Operations, "health") }), "go: capability manifest operations [cancel discover execute plan health] are not exactly [cancel discover execute plan]; nothing launched for it"},
		"duplicate":     {with(func(m *AdapterManifest) { m.Operations = append(m.Operations, "plan") }), "go: capability manifest operations [cancel discover execute plan plan] are not exactly [cancel discover execute plan]; nothing launched for it"},
	} {
		if got := adapterRefusal(test.manifests, AdapterGo); got != test.reason {
			t.Errorf("%s: refusal %q, want %q", name, got, test.reason)
		}
	}
}
