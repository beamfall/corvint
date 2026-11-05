// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestDTLV0010MissingRootNamesRecovery(t *testing.T) {
	t.Run("DTL-V0-010", func(t *testing.T) {
		t.Setenv("CORVINT_CONFINE_ROOT", "")
		err := run([]string{"never-executed"})
		if err == nil || !strings.Contains(err.Error(), "CORVINT_CONFINE_ROOT is unset") || !strings.Contains(err.Error(), "fix: export CORVINT_CONFINE_ROOT=") {
			t.Fatalf("%v", err)
		}
	})
}
