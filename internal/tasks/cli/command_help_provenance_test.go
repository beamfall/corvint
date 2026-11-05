// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// AFP-V0-9002: exact bytes of the two usage strings at lane base
// 8e2c13aa926fcaf1731fae71b27c41c41c8f09af. Moving display text into an
// embed must not change any published help output, including whitespace.
func TestEmbeddedParentPathHelpIsByteIdentical_AFPV09002(t *testing.T) {
	for key, want := range map[string]string{
		"queue snapshot":  "7ebfd4b0a861203da2891e3db2dc783d7fd46f78f10786e10ed512ebc60b4175",
		"archive restore": "53381624a4f222167ee40996c9e2f7d641d0e0df0a732b4e1d98d6b55de59af0",
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(commandUsage[key]))); got != want {
			t.Errorf("%s usage digest %s, want %s", key, got, want)
		}
	}
}
