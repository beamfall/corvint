package cli

import (
	"strings"
	"testing"
)

// TestTCLV0801_DeclaredBooleanFlags: a declared boolean flag parses without
// a value and leaves the next argument positional; a value on it is refused
// naming the flag; an undeclared flag is still unknown (V1-3606).
func TestTCLV0801_DeclaredBooleanFlags(t *testing.T) {
	fl, pos, err := flagsWithBooleans([]string{"V1-0001", "--lane-refs", "V1-0002", "--ref", "main", "--json"}, []string{"lane-refs", "json"}, "ref")
	if err != nil || fl["lane-refs"] != "true" || fl["json"] != "true" || fl["ref"] != "main" || strings.Join(pos, ",") != "V1-0001,V1-0002" {
		t.Fatalf("parsed %v %v %v", fl, pos, err)
	}
	if _, _, err := flagsWithBooleans([]string{"--lane-refs=no"}, []string{"lane-refs"}); err == nil || !strings.Contains(err.Error(), "--lane-refs takes no value") {
		t.Fatalf("valued boolean: %v", err)
	}
	if _, _, err := flags([]string{"--lane-refs"}, "ref"); err == nil || !strings.Contains(err.Error(), "unknown flag --lane-refs") {
		t.Fatalf("undeclared boolean: %v", err)
	}
	if fl, _, err := flagsWithBooleans([]string{"--ref", "x"}, []string{"lane-refs"}, "ref"); err != nil || fl["lane-refs"] != "" {
		t.Fatalf("absent boolean: %v %v", fl, err)
	}
}
