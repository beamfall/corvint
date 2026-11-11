// SPDX-License-Identifier: Apache-2.0
package flows

import (
	"slices"
	"strings"
	"testing"
)

// CMS-V0-914
func TestCMSV0914TestEvidenceRefusesWithThePointerToCorvint(t *testing.T) {
	fx := newShopFixture(t)
	code, out, diag := runFlowsCLI(fx.root, "test-evidence", "--flows", "flows", "checkout/checkout.go")
	if code != 2 || out != "" || !strings.Contains(diag, `"code": "invalid-arguments"`) || !strings.Contains(diag, "corvint flows test-evidence") {
		t.Fatalf("%d %q %s", code, out, diag)
	}
	if !slices.Contains(Subcommands(), "test-evidence") {
		t.Fatal("test-evidence left the registered subcommands; its refusal would fall through to the manifest form")
	}
}
