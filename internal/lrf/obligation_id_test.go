// SPDX-License-Identifier: Apache-2.0

package lrf

import "testing"

// TestOCMV0952ObligationIDsAcceptFourDigits pins that every ID OCM-V0-952
// admits as an obligation passes the stable request projection, and that the
// earlier shapes keep their result.
func TestOCMV0952ObligationIDsAcceptFourDigits(t *testing.T) {
	for id, want := range map[string]bool{
		"ASD-V0-2230": true, "TCP-V0-2220": true, "OCM-V0-001": true, "ABC-123": true, "ASD-V0-12345": true,
		"AB-V0-12": false, "ABC-V0-123A": false, "abc-V0-1234": false, "AB-1234": false,
	} {
		if got := obligationIDPattern.MatchString(id); got != want {
			t.Errorf("obligationIDPattern(%q) = %v, want %v", id, got, want)
		}
	}
}
