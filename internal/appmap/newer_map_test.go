package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-960: a map carrying a member this binary does not decode is refused as
// appmap-invalid-map with a message naming the member and the builder that wrote it, not read
// without the member; other malformed JSON keeps the generic refusal.
func TestAMAPV0960NewerMapMemberRefusalNamesMember(t *testing.T) {
	root, rev := fixtureRepo(t)
	encoded, err := Encode(build(t, root, rev))
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSuffix(string(encoded), "\n")
	if _, err := ParseMap([]byte(body)); err != nil {
		t.Fatalf("built map refused: %v", err)
	}
	top := strings.TrimSuffix(body, "}") + `,"future_member":true}`
	_, err = ParseMap([]byte(top))
	if codeOf(err) != "appmap-invalid-map" {
		t.Fatalf("map with an unknown member: %v, want appmap-invalid-map", err)
	}
	for _, want := range []string{`"future_member"`, BuilderVersion, "upgrade Corvint"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not name %q", err, want)
		}
	}
	other := strings.Replace(top, `"schema":"`+MapSchema+`"`, `"schema":"application-map/9"`, 1)
	if other == top {
		t.Fatal("fixture map has no schema member to replace")
	}
	if _, err = ParseMap([]byte(other)); err == nil || !strings.Contains(err.Error(), "is not application-map/0 JSON") {
		t.Fatalf("other schema with an unknown member: %v, want the generic refusal", err)
	}
	if _, err = ParseMap([]byte(body + "{}")); err == nil || !strings.Contains(err.Error(), "is not application-map/0 JSON") {
		t.Fatalf("trailing data: %v, want the generic refusal", err)
	}
}
