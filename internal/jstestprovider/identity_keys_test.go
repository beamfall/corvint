package jstestprovider

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// The qualified reporter resolves browser/device identity at runtime; the static
// --playwright-config profile resolves the same `use` keys from source (TJAA-V0-018).
func TestQualifiedReporterIdentityKeysMatchStaticProfile(t *testing.T) {
	match := regexp.MustCompile(`(?m)^const identityKeys = \[([^\]]*)\];$`).FindSubmatch(qualifiedReporter)
	if match == nil {
		t.Fatal("qualified reporter does not declare identityKeys on one line")
	}
	keys := []string{}
	for _, item := range strings.Split(string(match[1]), ",") {
		keys = append(keys, strings.Trim(strings.TrimSpace(item), "'"))
	}
	if !slices.Equal(keys, typescript.PlaywrightUseIdentityKeys) {
		t.Fatalf("reporter %v != static profile %v", keys, typescript.PlaywrightUseIdentityKeys)
	}
}
