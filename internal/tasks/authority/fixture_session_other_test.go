//go:build !darwin && !linux

package authority

import (
	"errors"
	"testing"
)

func TestTMV0010_AS10_FixtureUnsupportedPlatform(t *testing.T) {
	if s, err := newFixtureSession(nil, nil); s != nil || !errors.Is(err, fixtureUnsupported) {
		t.Fatal("unsupported session accepted")
	}
	if _, err := fixtureObserveMount(nil); !errors.Is(err, fixtureUnsupported) {
		t.Fatal("unsupported mount observed")
	}
	for _, err := range []error{fixtureLink(nil, "", nil, "", nil), fixtureRename(nil, "", nil, "", nil, nil), fixtureUnlink(nil, "", nil), fixtureMkdir(nil, "")} {
		if !errors.Is(err, fixtureUnsupported) {
			t.Fatal("unsupported syscall accepted")
		}
	}
}
