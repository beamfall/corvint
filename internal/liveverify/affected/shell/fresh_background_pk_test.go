// SPDX-License-Identifier: Apache-2.0
package shell

import "testing"

// TSP-V0-7317: a backgrounded AND/OR list assigns in a child shell, so the
// following source cannot rely on the parent's variable holding a fresh name.
func TestFreshDirectoryRejectsBackgroundAndOrList(t *testing.T) {
	for _, op := range []string{"&&", "||"} {
		for _, producer := range []string{"mktemp", "mktemp -d"} {
			t.Run(producer+op, func(t *testing.T) {
				root := t.TempDir()
				target := "$x"
				if producer == "mktemp -d" {
					target = "$x/meta.env"
				}
				write(t, root, "background.sh", "#!/bin/sh\nx=$("+producer+") "+op+" true &\n. \""+target+"\"\n")
				write(t, root, "foreground.sh", "#!/bin/sh\nx=$("+producer+") "+op+" true\n. \""+target+"\"\n")
				result := units(t, root)
				if got := unitOf(t, result, "background.sh"); got.UnboundedReads != UnboundedDynamicSource {
					t.Fatalf("unsafe fresh name was trusted: %+v", got)
				}
				if producer == "mktemp -d" {
					if got := unitOf(t, result, "foreground.sh"); got.UnboundedReads != "" {
						t.Fatalf("foreground directory list unnecessarily widened: %+v", got)
					}
				}
			})
		}
	}
}
