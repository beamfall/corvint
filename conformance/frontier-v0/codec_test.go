// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"testing"
)

// The vectors in vectors/codec.json are hand-authored from CF-V0-019. This file
// runs them twice: once against the independent reference codec in refcodec.go,
// which must agree today, and once against the implementation's codec, which
// skips until the binding exists. Two implementations agreeing on a
// hand-authored expectation is what CF-V0-028 asks for; the reference codec
// agreeing alone proves only that the vectors are internally consistent.

func loadCodecVectorsT(t *testing.T) CodecVectors {
	t.Helper()
	cv, err := LoadCodecVectors(".")
	if err != nil {
		t.Fatalf("load codec vectors: %v", err)
	}
	if len(cv.Vectors) == 0 {
		t.Fatal("codec vector file is empty")
	}
	return cv
}

func TestCodecVectorsAgainstReferenceCodec(t *testing.T) {
	cv := loadCodecVectorsT(t)
	seen := map[string]bool{}
	for _, v := range cv.Vectors {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			if seen[v.ID] {
				t.Fatalf("duplicate vector id %q", v.ID)
			}
			seen[v.ID] = true
			if v.Note == "" {
				t.Fatal("vector carries no note explaining which clause phrase it holds")
			}
			in, err := v.InputBytes()
			if err != nil {
				t.Fatalf("input: %v", err)
			}
			switch v.Mode {
			case "serialize":
				want, err := v.ExpectBytes()
				if err != nil {
					t.Fatalf("expect: %v", err)
				}
				val, err := Parse(in)
				if err != nil {
					t.Fatalf("Parse rejected a valid vector: %v", err)
				}
				got := Codec(val)
				if !bytes.Equal(got, want) {
					t.Fatalf("canonical bytes differ\n want %q\n  got %q", want, got)
				}
				// The recorded digest must digest the recorded bytes: a vector
				// whose hash and bytes disagree is worse than no vector.
				if h := Sha256Hex(want); h != v.ExpectSha256 {
					t.Fatalf("expectSha256 %s does not digest the expected bytes (%s)", v.ExpectSha256, h)
				}
				// Canonical output must itself be canonical.
				ok, err := IsCanonical(got)
				if err != nil || !ok {
					t.Fatalf("codec output is not a fixed point: ok=%v err=%v", ok, err)
				}
				// CF-V0-019: serialization emits no whitespace and no terminal LF.
				if bytes.ContainsAny(got, " \t\n\r") && !containsOnlyInsideStrings(got) {
					t.Fatalf("codec output carries structural whitespace: %q", got)
				}
				if got[len(got)-1] == '\n' {
					t.Fatalf("codec output ends with LF: %q", got)
				}
			case "invalid":
				if v.Reason == "" {
					t.Fatal("invalid vector names no rejection reason")
				}
				val, err := Parse(in)
				if err == nil {
					t.Fatalf("Parse accepted an invalid vector and produced %q", Codec(val))
				}
				if got := Reason(err); got != v.Reason {
					t.Fatalf("rejected for the wrong reason: want %q got %q (%v)", v.Reason, got, err)
				}
			case "verify":
				ok, err := IsCanonical(in)
				if err != nil {
					t.Fatalf("verify vector did not parse: %v", err)
				}
				if ok != v.Canonical {
					t.Fatalf("IsCanonical = %v, want %v", ok, v.Canonical)
				}
			case "document":
				ok, err := IsCompleteDocument(in)
				if err != nil {
					t.Fatalf("document vector did not parse: %v", err)
				}
				if ok != v.Canonical {
					t.Fatalf("IsCompleteDocument = %v, want %v", ok, v.Canonical)
				}
				if v.InputSha256 != "" {
					if h := Sha256Hex(in); h != v.InputSha256 {
						t.Fatalf("inputSha256 %s does not digest the input (%s)", v.InputSha256, h)
					}
				}
			default:
				t.Fatalf("unknown vector mode %q", v.Mode)
			}
		})
	}
}

// containsOnlyInsideStrings reports whether every whitespace byte in b sits
// inside a string literal. Structural whitespace is forbidden; a space inside a
// string value is ordinary content.
func containsOnlyInsideStrings(b []byte) bool {
	inString, escaped := false, false
	for _, c := range b {
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case !inString && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			return false
		}
	}
	return true
}

// TestCodecVectorCoverage asserts that every CF-V0-019 edge case the clause
// names by name has at least one vector. A vector file that quietly loses a
// case would otherwise still pass.
func TestCodecVectorCoverage(t *testing.T) {
	cv := loadCodecVectorsT(t)
	have := map[string]bool{}
	for _, v := range cv.Vectors {
		have[v.ID] = true
		if v.Mode == "invalid" {
			have["reason:"+v.Reason] = true
		}
	}
	required := []struct {
		id   string
		what string
	}{
		{"reason:duplicate-key", "duplicate object key is invalid"},
		{"reason:invalid-utf8", "invalid UTF-8 is invalid"},
		{"reason:bom", "a BOM is invalid"},
		{"reason:unpaired-surrogate", "an unpaired surrogate is invalid"},
		{"reason:number-forbidden", "JSON numbers are forbidden"},
		{"serialize/escape-controls-lowercase-u00xx", "U+0000..U+001F escape as lowercase \\u00xx"},
		{"serialize/key-sort-raw-utf8-not-utf16", "keys sort by raw UTF-8 bytes"},
		{"serialize/key-sort-ascii-raw-bytes", "raw byte order, not collation"},
		{"serialize/whitespace-stripped", "serialization emits no whitespace"},
		{"serialize/no-optional-escaping", "no optional escaping, no normalization"},
		{"verify/trailing-lf-inside-codec", "codec() emits no terminal LF"},
		{"document/exactly-one-lf", "a complete document is codec plus exactly one LF"},
		{"serialize/spec-item-example", "the CF-V0-017 literal example"},
		{"serialize/spec-result-example", "the CF-V0-018 literal example"},
	}
	for _, r := range required {
		if !have[r.id] {
			t.Errorf("no vector covers %s (expected id or reason %q)", r.what, r.id)
		}
	}
}

func TestImplementationCodecMatchesVectors(t *testing.T) {
	if registeredCodecer == nil {
		t.Skip(unboundReason)
	}
	cv := loadCodecVectorsT(t)
	for _, v := range cv.Vectors {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			in, err := v.InputBytes()
			if err != nil {
				t.Fatalf("input: %v", err)
			}
			got, err := registeredCodecer.CodecCanonicalize(in)
			switch v.Mode {
			case "serialize":
				want, wErr := v.ExpectBytes()
				if wErr != nil {
					t.Fatalf("expect: %v", wErr)
				}
				if err != nil {
					t.Fatalf("implementation rejected a valid vector: %v", err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("implementation bytes differ from the hand-derived vector\n want %q\n  got %q", want, got)
				}
			case "invalid":
				if err == nil {
					t.Fatalf("implementation accepted an invalid vector and produced %q", got)
				}
			case "verify":
				if err != nil {
					t.Fatalf("implementation rejected a parseable vector: %v", err)
				}
				if bytes.Equal(got, in) != v.Canonical {
					t.Fatalf("implementation canonicality = %v, want %v", bytes.Equal(got, in), v.Canonical)
				}
			case "document":
				// A complete document carries a terminal LF, which codec() does
				// not produce; the runner-level fixtures cover that surface.
				t.Skip("document framing is asserted through the fixture runner, not the codec seam")
			}
		})
	}
}
