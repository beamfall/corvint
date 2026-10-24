package companionrelease

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

const committedDeclaration = "../../tools/cem-release-suites/READINESS.json"

func readinessDeclarationFixture() StableReleaseDeclaration {
	return StableReleaseDeclaration{Profile: StableReleaseDeclarationProfile, Runner: []string{"corvint:neutral-criterion"}, Platform: []string{"linux/amd64"}, External: []string{"independent-external-consumer"}}
}

func refusalClass(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var refusal *StableReleaseRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("refusal %v is not a *StableReleaseRefusal", err)
	}
	return refusal.Class
}

// TestCEMV1600ReadinessDeclaration pins the closed cem-release-readiness/1
// shape: the committed declaration reads, and every malformed variant is a
// release-declaration refusal.
func TestCEMV1600ReadinessDeclaration(t *testing.T) {
	raw, err := os.ReadFile(committedDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ReadStableReleaseDeclaration(raw)
	if err != nil {
		t.Fatalf("committed declaration: %v", err)
	}
	if len(d.Runner) != 11 || len(d.Platform) != 2 || len(d.External) != 1 {
		t.Fatalf("committed declaration rows = %d/%d/%d", len(d.Runner), len(d.Platform), len(d.External))
	}
	valid := `{"profile":"cem-release-readiness/1","runner":["a"],"platform":["linux/amd64"],"external":["x"]}`
	if _, err := ReadStableReleaseDeclaration([]byte(valid + "\n")); err != nil {
		t.Fatalf("valid declaration: %v", err)
	}
	for name, raw := range map[string]string{
		"empty":           ``,
		"not an object":   `["a"]`,
		"wrong profile":   strings.Replace(valid, "readiness/1", "readiness/2", 1),
		"unknown member":  strings.Replace(valid, `"external"`, `"extra":[],"external"`, 1),
		"duplicate key":   strings.Replace(valid, `"runner":["a"]`, `"runner":["a"],"runner":["b"]`, 1),
		"trailing value":  valid + `{}`,
		"missing class":   `{"profile":"cem-release-readiness/1","runner":["a"],"platform":["linux/amd64"]}`,
		"empty class":     strings.Replace(valid, `["x"]`, `[]`, 1),
		"repeated row":    strings.Replace(valid, `["a"]`, `["a","a"]`, 1),
		"empty row":       strings.Replace(valid, `["a"]`, `[""]`, 1),
		"row with space":  strings.Replace(valid, `["a"]`, `["a b"]`, 1),
		"row too long":    strings.Replace(valid, `["a"]`, `["`+strings.Repeat("a", 129)+`"]`, 1),
		"too many rows":   strings.Replace(valid, `["a"]`, `[`+strings.TrimSuffix(strings.Repeat(`"r",`, 65), ",")+`]`, 1),
		"oversized":       valid + strings.Repeat(" ", 64<<10),
		"non-string rows": strings.Replace(valid, `["a"]`, `[1]`, 1),
		"case alias":      strings.Replace(valid, `"external":["x"]`, `"external":["x"],"Runner":["b"]`, 1),
		"recased member":  strings.Replace(valid, `"runner"`, `"Runner"`, 1),
		"reordered":       `{"profile":"cem-release-readiness/1","platform":["linux/amd64"],"runner":["a"],"external":["x"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ReadStableReleaseDeclaration([]byte(raw))
			if got := refusalClass(t, err); got != StableRefusalDeclaration {
				t.Fatalf("refusal class = %q (%v), want %s", got, err, StableRefusalDeclaration)
			}
		})
	}
}

// TestCEMV1601ReadinessReadsEveryResultFirst pins the release-run judgement:
// no accepted result is refused, every result is read before any outcome, and
// each class of row must be observed PASSED.
func TestCEMV1601ReadinessReadsEveryResultFirst(t *testing.T) {
	accept, err := os.ReadFile(stableAcceptResult)
	if err != nil {
		t.Fatal(err)
	}
	d := readinessDeclarationFixture()
	passed := func(row string) map[string]string { return map[string]string{row: "PASSED"} }
	members := stableResultMembersOf(t)
	members["verificationMode"], _ = json.Marshal("reference-integrity")
	wrongMode := emitStableResult(t, members, stableResultMembers, true)
	for _, c := range []struct {
		name                       string
		results                    [][]byte
		runner, platform, external map[string]string
		class                      string
	}{
		{"ready", [][]byte{accept, accept}, passed("corvint:neutral-criterion"), passed("linux/amd64"), passed("independent-external-consumer"), ""},
		{"no accepted result", nil, passed("corvint:neutral-criterion"), passed("linux/amd64"), passed("independent-external-consumer"), StableRefusalNotAccept},
		{"later result read before outcomes", [][]byte{accept, wrongMode}, nil, nil, nil, StableRefusalMode},
		{"runner NOT_RUN", [][]byte{accept}, map[string]string{"corvint:neutral-criterion": "NOT_RUN"}, passed("linux/amd64"), passed("independent-external-consumer"), StableRefusalRunner},
		{"platform not observed", [][]byte{accept}, passed("corvint:neutral-criterion"), passed("darwin/arm64"), passed("independent-external-consumer"), StableRefusalPlatform},
		{"external not observed", [][]byte{accept}, passed("corvint:neutral-criterion"), passed("linux/amd64"), map[string]string{}, StableRefusalExternal},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := StableReleaseReadiness(d, c.results, c.runner, c.platform, c.external)
			if got := refusalClass(t, err); got != c.class {
				t.Fatalf("refusal class = %q (%v), want %q", got, err, c.class)
			}
		})
	}
}
