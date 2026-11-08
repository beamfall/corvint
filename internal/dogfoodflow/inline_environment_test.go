// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"context"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// DCW-V0-8760: admit only equality with the complete child contract, including
// a context-introduced store that a caller-environment-only guard cannot see.
func TestInlineCitationEnvironmentContract(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv(lrfrepo.StableReuseEnv, "")
	if err := os.Unsetenv(lrfrepo.StableReuseEnv); err != nil {
		t.Fatal(err)
	}
	if !ChildEnvironmentMatchesProcess(context.Background()) {
		t.Fatal("identical environment refused")
	}
	if ChildEnvironmentMatchesProcess(withStableReuse(context.Background(), "/run/unknown-store")) {
		t.Fatal("context-introduced environment difference admitted")
	}
	t.Setenv("LC_ALL", "tr_TR.UTF-8")
	t.Setenv(lrfrepo.StableReuseEnv, "/inherited/store")
	if ChildEnvironmentMatchesProcess(context.Background()) {
		t.Fatal("hostile environment admitted")
	}
	t.Setenv("LC_ALL", "C")
	if ChildEnvironmentMatchesProcess(context.Background()) {
		t.Fatal("inherited store admitted with matching locale")
	}
}
