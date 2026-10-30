package sessiontest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// pinLockDrift names every way pins departs from the append-only lock rows
// (STR-V0-058, V1-2026): a locked version dropped or re-pinned, a duplicate
// locked version, or a pin not yet appended to the lock. It returns nil when
// pins and the lock agree row for row.
func pinLockDrift(pins map[string]string, lock []byte) []string {
	var drift []string
	locked := map[string]string{}
	for number, line := range strings.Split(string(lock), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			drift = append(drift, fmt.Sprintf("lock line %d: want \"version digest\", got %q", number+1, line))
			continue
		}
		version, digest := fields[0], fields[1]
		if _, ok := locked[version]; ok {
			drift = append(drift, "lock pins "+version+" twice")
		}
		locked[version] = digest
		switch pinned, ok := pins[version]; {
		case !ok:
			drift = append(drift, "locked pin "+version+" was removed from the pin table")
		case pinned != digest:
			drift = append(drift, "locked pin "+version+" was edited: table "+pinned+", lock "+digest)
		}
	}
	for version := range pins {
		if _, ok := locked[version]; !ok {
			drift = append(drift, "pin "+version+" is not appended to the lock")
		}
	}
	slices.Sort(drift)
	return drift
}

// STR-V0-058, V1-2026: the pin table is append-only against the committed
// lock, so editing or removing a shipped pin fails here rather than relying
// on review alone.
func TestSTRV0058PinTableMatchesLock(t *testing.T) {
	lock, err := os.ReadFile(filepath.Join("testdata", "adapter-manifest-pins.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if drift := pinLockDrift(adapterManifestVersionPins, lock); len(drift) != 0 {
		t.Fatalf("adapter manifest pin table departs from its append-only lock: %q", drift)
	}
	const v0 = "corvint-test-adapter/0"
	other := "sha256:" + strings.Repeat("0", 64)
	clone := func(edit func(map[string]string)) map[string]string {
		pins := maps.Clone(adapterManifestVersionPins)
		edit(pins)
		return pins
	}
	for name, test := range map[string]struct {
		pins map[string]string
		lock string
		want string
	}{
		"row edited":       {clone(func(p map[string]string) { p[v0] = other }), string(lock), "locked pin " + v0 + " was edited"},
		"row removed":      {clone(func(p map[string]string) { delete(p, v0) }), string(lock), "locked pin " + v0 + " was removed"},
		"pin not appended": {clone(func(p map[string]string) { p["corvint-test-adapter/1"] = other }), string(lock), "pin corvint-test-adapter/1 is not appended"},
		"lock duplicated":  {adapterManifestVersionPins, string(lock) + v0 + " " + other + "\n", "lock pins " + v0 + " twice"},
		"lock malformed":   {adapterManifestVersionPins, string(lock) + "corvint-test-adapter/1\n", "lock line"},
	} {
		drift := pinLockDrift(test.pins, []byte(test.lock))
		if !slices.ContainsFunc(drift, func(d string) bool { return strings.HasPrefix(d, test.want) }) {
			t.Errorf("%s: drift %q, want an entry starting %q", name, drift, test.want)
		}
	}
	appended := clone(func(p map[string]string) { p["corvint-test-adapter/1"] = other })
	if drift := pinLockDrift(appended, []byte(string(lock)+"corvint-test-adapter/1 "+other+"\n")); len(drift) != 0 {
		t.Fatalf("an appended pin with its lock row: %q", drift)
	}
}
