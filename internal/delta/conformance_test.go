package delta

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"testing"
)

func TestDeltaPublishedDecisionVectors(t *testing.T) {
	raw, err := os.ReadFile("../../conformance/delta-v0/decision-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name   string `json:"name"`
		Record Record `json:"record"`
	}
	if err := json.Unmarshal(raw, &vectors, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			r := v.Record
			r.finalize()
			if r.Decision != v.Name {
				t.Fatalf("got %s", r.Decision)
			}
			a, err := r.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			var round Record
			if err := json.Unmarshal(a, &round, json.RejectUnknownMembers(true)); err != nil {
				t.Fatal(err)
			}
			b, _ := round.Canonical()
			if !bytes.Equal(a, b) {
				t.Fatal("canonical drift")
			}
			bad := append([]byte(`{"unexpected":"PROSE_SENTINEL",`), a[1:]...)
			if err := json.Unmarshal(bad, &round, json.RejectUnknownMembers(true)); err == nil {
				t.Fatal("extra field accepted")
			}
		})
	}
}
