package sessiontest

import (
	"reflect"
	"strings"
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

// adapterManifestContent is what a capability manifest version names: its
// profile and its operation set, whichever adapter carries it.
type adapterManifestContent struct {
	Operations []string `json:"operations"`
	Profile    string   `json:"profile"`
}

// adapterManifestVersionPins is the append-only history of shipped
// capability manifest versions and the digest of their content
// (STR-V0-058). A pin is never edited or removed: a manifest whose
// operations change needs a new version and a new pin beside the old ones.
var adapterManifestVersionPins = map[string]string{
	"corvint-test-adapter/0": "sha256:4378e5864ff8bce9f1ee4f01540134197dff5e39ff424f5d06418fa0a3e1066b",
}

// manifestVersionDrift names why manifest's version does not identify its
// content under pins, or "" when its version is pinned to exactly this
// content.
func manifestVersionDrift(pins map[string]string, manifest AdapterManifest) string {
	digest, err := Digest(adapterManifestContent{Operations: manifest.Operations, Profile: manifest.Profile})
	if err != nil {
		return manifest.Adapter + ": " + err.Error()
	}
	pinned, ok := pins[manifest.Profile]
	switch {
	case !ok:
		return manifest.Adapter + ": capability manifest version " + manifest.Profile + " has no pin; a new version needs its content digest pinned"
	case pinned != digest:
		return manifest.Adapter + ": capability manifest " + manifest.Profile + " changed without a version bump: content " + digest + ", pinned " + pinned
	}
	return ""
}

// STR-V0-058: the capability manifest version identifies its content. Every
// registered manifest matches its version's pin, and the check refuses a
// changed operation set under an existing version and a new version without
// a pin.
func TestSTRV0058AdapterManifestVersionPinned(t *testing.T) {
	for adapter, manifest := range AdapterManifests() {
		if drift := manifestVersionDrift(adapterManifestVersionPins, manifest); drift != "" {
			t.Errorf("registered adapter %s: %s", adapter, drift)
		}
	}
	good := AdapterManifests()[AdapterGo]
	with := func(profile string, operations ...string) AdapterManifest {
		return AdapterManifest{Adapter: AdapterGo, Operations: operations, Profile: profile}
	}
	for name, test := range map[string]struct {
		manifest AdapterManifest
		drift    string
	}{
		"operation added, same version":   {with(good.Profile, "cancel", "discover", "execute", "health", "plan"), "go: capability manifest corvint-test-adapter/0 changed without a version bump"},
		"operation dropped, same version": {with(good.Profile, "discover", "execute", "plan"), "go: capability manifest corvint-test-adapter/0 changed without a version bump"},
		"new version without a pin":       {with("corvint-test-adapter/1", "cancel", "discover", "execute", "health", "plan"), "go: capability manifest version corvint-test-adapter/1 has no pin"},
	} {
		if drift := manifestVersionDrift(adapterManifestVersionPins, test.manifest); !strings.HasPrefix(drift, test.drift) {
			t.Errorf("%s: drift %q, want prefix %q", name, drift, test.drift)
		}
	}
	// A bump that pins its new content passes, and the old pin still holds.
	bumped := with("corvint-test-adapter/1", "cancel", "discover", "execute", "health", "plan")
	digest, err := Digest(adapterManifestContent{Operations: bumped.Operations, Profile: bumped.Profile})
	if err != nil {
		t.Fatal(err)
	}
	pins := map[string]string{bumped.Profile: digest}
	for version, pin := range adapterManifestVersionPins {
		pins[version] = pin
	}
	if drift := manifestVersionDrift(pins, bumped); drift != "" {
		t.Fatalf("pinned bump: %s", drift)
	}
	if drift := manifestVersionDrift(pins, good); drift != "" {
		t.Fatalf("earlier version after the bump: %s", drift)
	}
}
