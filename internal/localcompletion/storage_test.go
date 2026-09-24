package localcompletion

import "testing"

// TestValidatePlanRefusesFinalCheckInAnyForm pins LCP-V0-015: a selected check
// never runs the final check, whether by script name or by subverb.
func TestValidatePlanRefusesFinalCheckInAnyForm(t *testing.T) {
	cases := map[string]struct {
		argv []string
		want string
	}{
		"script name":       {[]string{"script/dogfood-check.sh", "BASE"}, "final-check-not-prerequisite"},
		"make target":       {[]string{"make", "dogfood-check"}, "final-check-not-prerequisite"},
		"check subverb":     {[]string{"/usr/local/bin/corvint", "dogfood", "check", "BASE"}, "final-check-not-prerequisite"},
		"seal subverb":      {[]string{"corvint", "--root", ".", "dogfood", "seal", "BASE"}, "final-check-not-prerequisite"},
		"change subverb":    {[]string{"corvint", "dogfood", "change", "BASE"}, ""},
		"non-adjacent pair": {[]string{"echo", "dogfood", "then", "check"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan := Plan{
				Base:    "0123456789abcdef0123456789abcdef01234567",
				Intents: []string{"docs/specs/example.md"},
				Checks:  []Check{{ID: "check", Argv: tc.argv, TimeoutSeconds: 60}},
			}
			got := ""
			if err := validatePlan(plan); err != nil {
				got = err.Error()
			}
			if got != tc.want {
				t.Fatalf("validatePlan(%v) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}
