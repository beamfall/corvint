//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestATRV0161_RepeatedConfigMemberRefuses: a member repeated at the top level
// or inside a role is refused with its path, even when both values agree,
// instead of the last value silently winning (V1-0951).
func TestATRV0161_RepeatedConfigMemberRefuses(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat = "claude-code"
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	cases := map[string]struct{ old, new, path string }{
		"top-level member":     {`"globalCap":`, `"globalCap":1,"globalCap":`, `"/globalCap"`},
		"member inside a role": {`"usageFormat":"claude-code"`, `"usageFormat":"claude-code","usageFormat":"claude-code"`, `"/roles/0/usageFormat"`},
		"escaped spelling":     {`"tickSeconds":`, `"tick\u0053econds":1,"tickSeconds":`, `"/tickSeconds"`},
	}
	for name, tc := range cases {
		if !strings.Contains(string(raw), tc.old) {
			t.Fatalf("%s: fixture lacks %s: %s", name, tc.old, raw)
		}
		_, err := DecodeConfig([]byte(strings.Replace(string(raw), tc.old, tc.new, 1)))
		if err == nil || !strings.Contains(err.Error(), "repeated member "+tc.path) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}
