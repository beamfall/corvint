package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestTCLV0737_FlagsParseDeclaredBooleans pins V1-3606: flagsBool parses a
// declared boolean as a bare flag in any position, refuses a value on it,
// and leaves value flags and positionals as flags does.
func TestTCLV0737_FlagsParseDeclaredBooleans(t *testing.T) {
	vals, set, pos, err := flagsBool([]string{"a", "--lane-refs", "--ref", "main", "b", "--json", "--limit=5"}, []string{"lane-refs", "json"}, "ref", "limit")
	if err != nil || !set["lane-refs"] || !set["json"] || vals["ref"] != "main" || vals["limit"] != "5" || strings.Join(pos, ",") != "a,b" {
		t.Fatalf("vals=%v set=%v pos=%v err=%v", vals, set, pos, err)
	}
	if _, _, _, err := flagsBool([]string{"--lane-refs=yes"}, []string{"lane-refs"}); err == nil || !strings.Contains(err.Error(), "takes no value") {
		t.Fatalf("a value on a boolean was accepted: %v", err)
	}
	if _, _, _, err := flagsBool([]string{"--bogus"}, []string{"lane-refs"}, "ref"); err == nil || !strings.Contains(err.Error(), "unknown flag --bogus") {
		t.Fatalf("an unknown flag was accepted: %v", err)
	}
	if _, _, err := flags([]string{"--lane-refs"}, "ref"); err == nil {
		t.Fatal("flags without declared booleans accepted --lane-refs")
	}
}

// TestTCLV0737_DeliveryScreenUsageMatchesParser checks that the flags the
// delivery-screen usage names are exactly the ones its parser declares: bare
// usage flags are declared booleans and `--flag VALUE` usage flags are value
// flags, so nothing is stripped by hand before the parser.
func TestTCLV0737_DeliveryScreenUsageMatchesParser(t *testing.T) {
	use := commandUsage["ticket delivery-screen"]
	var bools, values []string
	for _, m := range regexp.MustCompile(`\[--([a-z-]+)( [^\]]+)?\]`).FindAllStringSubmatch(use, -1) {
		if m[2] == "" {
			bools = append(bools, m[1])
		} else {
			values = append(values, m[1])
		}
	}
	slices.Sort(bools)
	slices.Sort(values)
	wantBools, wantValues := slices.Sorted(slices.Values(deliveryScreenBools)), slices.Sorted(slices.Values(deliveryScreenValues))
	if !slices.Equal(bools, wantBools) || !slices.Equal(values, wantValues) {
		t.Fatalf("usage %q names booleans %v and value flags %v; the parser declares %v and %v", use, bools, values, wantBools, wantValues)
	}
}
