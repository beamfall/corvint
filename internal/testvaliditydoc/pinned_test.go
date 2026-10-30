package testvaliditydoc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProjectPinnedKeepsNativeAxesAndUnknownBuild(t *testing.T) {
	for _, kind := range []string{"unit", "e2e"} {
		for _, state := range []string{"passed", "failed", "skipped", "flaky"} {
			raw, _ := json.Marshal(map[string]any{"receipt": map[string]any{"kind": kind, "identity": map[string]any{"testFileDigests": map[string]string{"test.ts": digestHex([]byte("test"))}}, "appBuildAtPublish": map[string]any{"unknown": true}, "tests": []any{map[string]any{"fullName": "exact", "state": state}}}})
			input, err := Decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			base := Project(input)
			got := ProjectPinned(input, func(p string) ([]byte, bool) { return []byte("test"), p == "test.ts" })
			if !reflect.DeepEqual(got.Tests[0].Projection.Execution, base.Tests[0].Projection.Execution) {
				t.Fatal("native execution changed")
			}
			want := "CURRENT"
			if kind == "e2e" {
				want = "UNKNOWN"
			}
			if got.Run.Freshness.State != want || got.Tests[0].Projection.Freshness.State != want {
				t.Fatalf("%s %s freshness: %+v", kind, state, got)
			}
			stale := ProjectPinned(input, func(string) ([]byte, bool) { return []byte("changed"), true })
			if stale.Run.Freshness.State != "STALE" {
				t.Fatal("digest mismatch accepted")
			}
		}
	}
}
