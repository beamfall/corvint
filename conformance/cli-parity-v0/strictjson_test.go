package main

import (
	"encoding/json"
	"strings"
	"testing"
)

type strictJSONFixture struct {
	Name   string              `json:"name"`
	Nested strictJSONNested    `json:"nested"`
	Items  []strictJSONNested  `json:"items"`
	Lookup map[string][]string `json:"lookup"`
}

type strictJSONNested struct {
	Enabled bool `json:"enabled"`
}

func TestDecodeStrictJSONAcceptsOneClosedJSONValue(t *testing.T) {
	raw := []byte(" \n\t{\"name\":\"fixture\",\"nested\":{\"enabled\":true},\"items\":[{\"enabled\":false}],\"lookup\":{\"commands\":[\"query\",\"impact\"]}} \r\n")
	var got strictJSONFixture
	if err := decodeStrictJSON(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "fixture" || !got.Nested.Enabled || len(got.Items) != 1 || got.Items[0].Enabled {
		t.Fatalf("decoded fixture = %#v", got)
	}
	if values := got.Lookup["commands"]; len(values) != 2 || values[0] != "query" || values[1] != "impact" {
		t.Fatalf("decoded lookup = %#v", got.Lookup)
	}
}

func TestDecodeStrictJSONValidationPreservesJSONNumberRange(t *testing.T) {
	var got struct {
		Number json.Number `json:"number"`
	}
	if err := decodeStrictJSON([]byte(`{"number":1e1000}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Number != "1e1000" {
		t.Fatalf("number = %q", got.Number)
	}
}

func TestDecodeStrictJSONRejectsDuplicateObjectKeysRecursively(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"root", `{"name":"first","name":"second"}`},
		{"nested-object", `{"nested":{"enabled":true,"enabled":false}}`},
		{"object-in-array", `{"items":[{"enabled":true,"enabled":false}]}`},
		{"escaped-equivalent", `{"name":"first","\u006eame":"second"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got strictJSONFixture
			err := decodeStrictJSON([]byte(test.raw), &got)
			if err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("error = %v, want duplicate-key rejection", err)
			}
		})
	}
}

func TestDecodeStrictJSONRejectsTrailingData(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"second-value", `{"name":"fixture"}{}`},
		{"non-json-data", `{"name":"fixture"} trailing`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got strictJSONFixture
			if err := decodeStrictJSON([]byte(test.raw), &got); err == nil {
				t.Fatal("expected trailing-data rejection")
			}
		})
	}
}

func TestDecodeStrictJSONRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"root", `{"unknown":true}`},
		{"nested", `{"nested":{"unknown":true}}`},
		{"array-member", `{"items":[{"unknown":true}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got strictJSONFixture
			err := decodeStrictJSON([]byte(test.raw), &got)
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("error = %v, want unknown-field rejection", err)
			}
		})
	}
}

func TestDecodeStrictJSONRejectsMalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"truncated", []byte(`{"nested":`)},
		{"trailing-comma", []byte(`{"name":"fixture",}`)},
		{"mismatched-delimiter", []byte(`{"items":[}`)},
		{"invalid-utf8", append([]byte(`{"name":"`), 0xff, '"', '}')},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got strictJSONFixture
			if err := decodeStrictJSON(test.raw, &got); err == nil {
				t.Fatal("expected malformed-JSON rejection")
			}
		})
	}
}
