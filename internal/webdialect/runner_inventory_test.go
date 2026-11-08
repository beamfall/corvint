// SPDX-License-Identifier: Apache-2.0
package webdialect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/browsercov"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func TestRSPV0052OnePlaywrightInventory(t *testing.T) {
	d := jslinecov.Discovery{Key: testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: "playwright", ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}, Root: "/repo", Version: "1.63.0", Tracked: []string{"a.spec.ts"}, ConfigKnown: true, ListOK: true}
	raw := `{"config":{"rootDir":"/repo"},"suites":[{"specs":[{"id":"native","file":"a.spec.ts","title":"page","tests":[{"projectName":"chromium"}]}]}]}`
	a, e := E2EInventory(d, raw)
	if e != nil {
		t.Fatal(e)
	}
	b, e := browsercov.PlaywrightInventory(d, raw)
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatal(a, b, e)
	}
	for _, raw := range []string{`{}`, `null`} {
		in, e := E2EInventory(d, raw)
		if e == nil || in.Complete {
			t.Fatal("separate discovery accepted", in, e)
		}
	}
}
