// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestStructuralVectorsAgainstRealParser offers every frozen byte-level vector
// to the real structural OCM parser (adapter.go, DecodeStructural) and asserts
// the exact outcome each vector declares.
func TestStructuralVectorsAgainstRealParser(t *testing.T) {
	vectors, err := LoadStructuralVectors(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateVectors(vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.ID, func(t *testing.T) {
			input, err := v.Input()
			if err != nil {
				t.Fatal(err)
			}
			obligation, reason := markSelector(v)
			decoded, err := DecodeStructural(input, obligation, reason)
			if err != nil {
				t.Fatalf("decoder returned a non-refusal error: %v", err)
			}
			assertStructural(t, v, input, decoded)
		})
	}
}

// markSelector names the mark each vector is offered through. Vectors that
// declare no round trip mark the first row unassessed, which only a parsed
// map can reach.
func markSelector(v StructuralVector) (string, string) {
	if v.Obligation != "" {
		return v.Obligation, v.Reason
	}
	return "1", "unassessed"
}

func assertStructural(t *testing.T, v StructuralVector, input []byte, decoded Decoded) {
	t.Helper()
	switch v.Mode {
	case "valid":
		if decoded.Code != "" {
			t.Fatalf("refused with %s: %s", decoded.Code, decoded.Message)
		}
		if !bytes.Equal(decoded.Output, input) {
			t.Fatalf("round trip changed the bytes:\n got %s\nwant %s", decoded.Output, input)
		}
	case "produced":
		if decoded.Code != "" {
			t.Fatalf("refused with %s: %s", decoded.Code, decoded.Message)
		}
		if _, err := wire.Parse(decoded.Output); err != nil {
			t.Fatalf("republished output is not JSON: %v", err)
		}
	case "oversize":
		if decoded.Code != v.Code || !strings.Contains(decoded.Message, "exceeds the 1048576-byte limit") {
			t.Fatalf("got %q %q, want %q naming the byte limit", decoded.Code, decoded.Message, v.Code)
		}
	default:
		if decoded.Code != v.Code {
			t.Fatalf("got %q (%s), want %q", decoded.Code, decoded.Message, v.Code)
		}
	}
}

// TestValidVectorsAreCanonical proves each frozen valid map is its own
// canonical encoding: sorted members, no insignificant whitespace, one
// terminal LF. This is the wire's byte contract, checked without the OCM
// parser.
func TestValidVectorsAreCanonical(t *testing.T) {
	vectors, err := LoadStructuralVectors(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		if v.Mode != "valid" && v.Mode != "produced" {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			root, err := wire.Parse([]byte(v.Raw))
			if err != nil {
				t.Fatal(err)
			}
			canonical := append(wire.CanonicalValue(root), '\n')
			if v.Raw != string(canonical) {
				t.Fatalf("frozen bytes are not canonical:\n got %s\nwant %s", v.Raw, canonical)
			}
		})
	}
}

// TestStructuralVectorsCoverEveryDisposition proves the frozen valid maps
// carry both dispositions and every unknown reason the wire admits.
func TestStructuralVectorsCoverEveryDisposition(t *testing.T) {
	vectors, err := LoadStructuralVectors(".")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		`"disposition":"linked","hunkIds":["hunk:sha256:`:                                          false,
		`"disposition":"unknown","hunkIds":[],"id":"CONF-V0-001","reason":"unassessed"`:            false,
		`"disposition":"unknown","hunkIds":[],"id":"CONF-V0-002","reason":"no-test-claim"`:         false,
		`"disposition":"unknown","hunkIds":[],"id":"CONF-V0-001","reason":"insufficient-evidence"`: false,
		`"disposition":"unknown","hunkIds":[],"id":"CONF-V0-001","reason":"conflicting-evidence"`:  false,
	}
	for _, v := range vectors {
		if v.Mode != "valid" && v.Mode != "produced" {
			continue
		}
		for needle := range want {
			if strings.Contains(v.Raw, needle) {
				want[needle] = true
			}
		}
	}
	for needle, seen := range want {
		if !seen {
			t.Errorf("no valid vector carries %s", needle)
		}
	}
}
