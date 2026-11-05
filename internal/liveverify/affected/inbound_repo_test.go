// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"strings"
	"testing"
)

// AZU-V0-004, AZU-V0-8901: actual one-file Go and shell probes close the
// incoming shell frontier while keeping their declared caller selections.
func TestRepositoryInboundCallers_AZU_V0_8901(t *testing.T) {
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"internal/gateselect/allowlist.go", "script/heavy.sh", ".corvint/change.cem.json"} {
		plan := affected.Select(graph, []string{dirty})
		for _, u := range plan.Unknown {
			if u.Detail == "shell:foreign-invocation-unmodelled" {
				var sites []string
				for _, b := range plan.FrontierBearers {
					if b.Detail == u.Detail {
						sites = append(sites, b.Unit+":"+b.Via)
					}
				}
				t.Errorf("%s retains inbound shell uncertainty at %s", dirty, strings.Join(sites, ", "))
			}
		}
	}
}
