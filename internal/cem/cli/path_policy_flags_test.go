package cli

import "testing"

// TestCAXV0060PathPolicyFlagIsBaseOnly checks CAX-V0-060: --path-policy is
// admitted on prepare and status with the single value base, refused with any
// other value, and not admitted on verify or report.
func TestCAXV0060PathPolicyFlagIsBaseOnly(t *testing.T) {
	parse := func(action string, args ...string) error {
		spec := cemActions[action]
		flags, unrecognized, err := parseCEMFlags(spec, args)
		if err == nil {
			err = validateParsed(spec, flags, unrecognized)
		}
		return err
	}
	for _, accepted := range [][]string{
		{"prepare", "--base", "B", "--target", "T", "--path-policy", "base"},
		{"status", "--map", "m.json", "--path-policy", "base"},
	} {
		if err := parse(accepted[0], accepted[1:]...); err != nil {
			t.Fatalf("%v: %v", accepted, err)
		}
	}
	for _, refused := range [][]string{
		{"prepare", "--base", "B", "--target", "T", "--path-policy", "worktree"},
		{"status", "--map", "m.json", "--path-policy", "head"},
		{"verify", "--map", "m.json", "--path-policy", "base"},
		{"report", "--map", "m.json", "--path-policy", "base"},
	} {
		if err := parse(refused[0], refused[1:]...); err == nil {
			t.Fatalf("%v admitted", refused)
		}
	}
}
