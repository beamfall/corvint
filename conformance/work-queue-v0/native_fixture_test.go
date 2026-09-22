//go:build unix

package workqueuev0

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeDocumentsMatchFrozenIndependentFixture(t *testing.T) {
	read := func(name string) map[string]any {
		t.Helper()
		raw, err := os.ReadFile("testdata/cli_" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	policy, snapshot := read("policy"), read("snapshot")
	queue := fixtureQueue("/private/tmp/native-fixture-spies", false)
	docs, err := Documents(policy, queue, snapshot["repositorySource"].(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]map[string]any{"snapshot": snapshot, "details": read("details"), "verify": read("verify")} {
		if !equalCanonical(docs[name], want) {
			gotRaw, _ := Canonical(docs[name])
			wantRaw, _ := Canonical(want)
			t.Fatalf("native %s differs\ngot %s\nwant %s", name, gotRaw, wantRaw)
		}
	}
}

func TestCanonicalIdentityLiteral(t *testing.T) {
	body := map[string]any{"b": "é", "a": nil}
	raw, err := Canonical(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"a":null,"b":"é"}` {
		t.Fatalf("canonical bytes: %q", raw)
	}
	id, err := Identity("kind", "profile/0", body)
	if err != nil {
		t.Fatal(err)
	}
	if id != "kind:sha256:"+Digest(append([]byte("kind\x00profile/0\x00"), raw...)) {
		t.Fatal("identity preimage drift")
	}
}
