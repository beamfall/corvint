package companionrelease

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCoreNativeJSONNumberBoundary(t *testing.T) {
	for _, raw := range []string{`{"durationMs":1202.922583}`, `{"nativeNegative":-1}`, `{"durationMs":1.2e3}`, `{"s":"\\uD800","durationMs":0.5}`, `{"s":"\uD83D\uDE00","durationMs":0.5}`} {
		before := []byte(raw)
		input := append([]byte(nil), before...)
		if err := validateCoreNativeJSON(input); err != nil {
			t.Fatalf("valid native JSON %s: %v", raw, err)
		}
		if !bytes.Equal(before, input) {
			t.Fatal("native bytes changed")
		}
	}
	for _, raw := range []string{
		`{"x":0.5,"\u0078":1}`, `{"a":{"x":0.5,"x":1}}`,
		`{"s":"\uD800","durationMs":0.5}`, `{"s":"\uDC00"}`, `{"s":"\uD800\u0041"}`,
		`{"n":NaN}`, `{"n":Infinity}`, `{"n":1e999}`, `{"n":-1e999}`, `{"n":01.2}`,
		`{"n":0.5} {}`, string([]byte{'"', 0xff, '"'}), strings.Repeat("[", 65) + "0.5" + strings.Repeat("]", 65),
	} {
		if err := validateCoreNativeJSON([]byte(raw)); err == nil {
			t.Fatalf("invalid native JSON accepted: %q", raw)
		}
	}
	raw := `{"durationMs":1202.922583}`
	if _, err := evidenceMap([]InstalledEvidence{{Name: "native", SHA256: sha256Hex([]byte(raw)), Raw: raw}}, []string{"native"}); err != nil {
		t.Fatal(err)
	}
}

func TestCoreCompleteReportReplay(t *testing.T) {
	path := os.Getenv("CORVINT_TEST_CORE_FULL_REPORT")
	if path == "" {
		t.Skip("retained or explicitly reconstructed diagnostic report required")
	}
	body, err := readBoundedRegular(path, installedTotalOutputLimit)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCoreInstalledReport(body); err != nil {
		t.Fatal(err)
	}
	var report CoreInstalledReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	original := report.Providers[0]
	duration := regexp.MustCompile(`"durationMs"\s*:\s*-?[0-9]+\.[0-9]+(?:[eE][+-]?[0-9]+)?`)
	if !duration.MatchString(original.Raw) {
		t.Fatal("actual unit duration evidence missing")
	}
	report.Providers[0].Raw = duration.ReplaceAllString(original.Raw, `"durationMs":1e999`)
	report.Providers[0].SHA256 = sha256Hex([]byte(report.Providers[0].Raw))
	bad, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCoreInstalledReport(bad); err == nil {
		t.Fatal("nonfinite native duration accepted in full report")
	}
}
