package workqueue

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDetailRequestDerivation(t *testing.T) {
	t.Run("WQO-V0-001", func(t *testing.T) {
		digest := SHA256Hex(nil)
		// Skipped ACTIVE/null tickets do not consume the limit; semantic order of
		// eligible IDs is z,c,a, deliberately different from canonical order.
		tickets := []TicketSummary{
			{Rank: 1, TicketVersionID: "b", Lifecycle: "ACTIVE", DetailPayloadSHA256: &digest},
			{Rank: 2, TicketVersionID: "d", Lifecycle: "READY"},
			{Rank: 3, TicketVersionID: "z", Lifecycle: "READY", DetailPayloadSHA256: &digest},
			{Rank: 4, TicketVersionID: "c", Lifecycle: "READY", DetailPayloadSHA256: &digest},
			{Rank: 5, TicketVersionID: "a", Lifecycle: "READY", DetailPayloadSHA256: &digest},
		}
		for _, c := range []struct {
			name, limit string
			requests    []string
			want        string
		}{
			{"zero", "0", []string{}, "VALIDATED_AT"},
			{"semantic prefix", "1", []string{"z"}, "VALIDATED_AT"},
			{"sorted selected prefix", "2", []string{"c", "z"}, "VALIDATED_AT"},
			{"max bound", "512", []string{"a", "c", "z"}, "VALIDATED_AT"},
			{"wrong prefix", "1", []string{"a"}, "CONFLICTED"},
			{"active", "512", []string{"a", "b", "c", "z"}, "CONFLICTED"},
			{"null", "512", []string{"a", "c", "d", "z"}, "CONFLICTED"},
			{"missing", "512", []string{"c", "z"}, "CONFLICTED"},
			{"over limit", "1", []string{"c", "z"}, "CONFLICTED"},
			{"duplicate", "2", []string{"z", "z"}, "CONFLICTED"},
			{"wrong canonical order", "2", []string{"z", "c"}, "CONFLICTED"},
			{"zero with request", "0", []string{"z"}, "CONFLICTED"},
		} {
			t.Run(c.name, func(t *testing.T) {
				snapshot := &Snapshot{Tickets: tickets, DetailRequestTicketVersionIDs: c.requests}
				policy := &Policy{DetailLimit: c.limit}
				before, err := json.Marshal([]any{snapshot, policy})
				if err != nil {
					t.Fatal(err)
				}
				got := ValidateDetailRequests(snapshot, policy)
				if got.State != c.want || len(got.Unknowns) != 0 {
					t.Fatalf("got %+v; want %s", got, c.want)
				}
				after, err := json.Marshal([]any{snapshot, policy})
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(after) {
					t.Fatal("validator mutated its input")
				}
			})
		}
		for _, tickets := range [][]TicketSummary{nil, {{Lifecycle: "ACTIVE", DetailPayloadSHA256: &digest}}, {{Lifecycle: "READY"}}} {
			got := ValidateDetailRequests(&Snapshot{Tickets: tickets}, &Policy{DetailLimit: "512"})
			if got.State != "VALIDATED_AT" {
				t.Fatalf("empty eligible set: %+v", got)
			}
		}
	})
}

func TestDetailRequestUnusablePolicy(t *testing.T) {
	for _, limit := range []string{"", "01", "-1", "one", "513", "2147483648", "9999999999999999999999999"} {
		t.Run(limit, func(t *testing.T) { assertDetailRequestUnable(t, &Snapshot{}, &Policy{DetailLimit: limit}) })
	}
	assertDetailRequestUnable(t, nil, &Policy{DetailLimit: "1"})
	assertDetailRequestUnable(t, &Snapshot{}, nil)
	assertDetailRequestUnable(t, nil, nil)
}

func assertDetailRequestUnable(t *testing.T, snapshot *Snapshot, policy *Policy) {
	t.Helper()
	want := ValidationResult{State: "UNKNOWN", Unknowns: []string{"ADAPTER_INVALID"}}
	if got := ValidateDetailRequests(snapshot, policy); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}
