// SPDX-License-Identifier: Apache-2.0

package lrfrepo

import (
	"errors"
	"reflect"
	"testing"
)

// TestOCMV0952IntentAcceptsFourDigitRequirementIDs pins OCM-V0-952: a
// Requirements section whose IDs all have four digits is an OCM intent, as
// script/gen-spec-requirements.sh already lists them, and three-digit IDs,
// mixed sections and every refusal keep their earlier result.
func TestOCMV0952IntentAcceptsFourDigitRequirementIDs(t *testing.T) {
	accepted := map[string]struct {
		body string
		want []string
	}{
		"four digits only": {
			"# Spec\n\n## Requirements\n\n- `ASD-V0-2230`: one.\n- **TCP-V0-2220:** two.\n- **TCP-V0-2221.** three.\n\n## Non-goals\n",
			[]string{"ASD-V0-2230", "TCP-V0-2220", "TCP-V0-2221"},
		},
		"three digits unchanged": {
			"## Requirements\n\n- `OCM-V0-001`: one.\n- **ABC-123:** two.\n",
			[]string{"OCM-V0-001", "ABC-123"},
		},
		"mixed widths": {
			"## Requirements\n\n- `ASD-V0-999`: one.\n- `ASD-V0-1000`: two.\n- `ASD-V0-12345`: three.\n",
			[]string{"ASD-V0-999", "ASD-V0-1000", "ASD-V0-12345"},
		},
		"non-IDs still skipped": {
			"## Requirements\n\n- `AB-V0-12`: two digits.\n- `ABC-V0-123A`: suffix.\n- `abc-V0-1234`: lower case.\n- `ASD-V0-2230`: kept.\n",
			[]string{"ASD-V0-2230"},
		},
	}
	for name, test := range accepted {
		t.Run(name, func(t *testing.T) {
			_, got, _, err := scanRequirements("docs/specs/x.md", "", []byte(test.body), maxObligations)
			if err != nil {
				t.Fatalf("scanRequirements() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("requirements = %q, want %q", got, test.want)
			}
		})
	}
	refused := map[string]struct{ body, code string }{
		"two digits only":         {"## Requirements\n\n- `AB-V0-12`: x.\n", "missing-requirements"},
		"no IDs":                  {"## Requirements\n\nProse only.\n", "missing-requirements"},
		"duplicate four digits":   {"## Requirements\n\n- `ASD-V0-2230`: a.\n- **ASD-V0-2230:** b.\n", "duplicate-requirement"},
		"duplicate three digits":  {"## Requirements\n\n- `ASD-V0-223`: a.\n- `ASD-V0-223`: b.\n", "duplicate-requirement"},
		"no Requirements heading": {"## Other\n\n- `ASD-V0-2230`: a.\n", "invalid-requirements-section"},
		"over the limit":          {"## Requirements\n\n- `ASD-V0-2230`: a.\n- `ASD-V0-2231`: b.\n", "too-many-obligations"},
	}
	for name, test := range refused {
		t.Run(name, func(t *testing.T) {
			limit := maxObligations
			if name == "over the limit" {
				limit = 1
			}
			_, _, _, err := scanRequirements("docs/specs/x.md", "", []byte(test.body), limit)
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != test.code {
				t.Fatalf("scanRequirements() error = %v, want code %s", err, test.code)
			}
		})
	}
}
