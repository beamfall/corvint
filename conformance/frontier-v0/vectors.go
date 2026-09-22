// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The vector files under vectors/ are the reference data this suite exists to
// hold. Every `expect` string in them was authored by hand from the clause
// text; nothing in this package or in the emitting script reads or executes
// internal/frontier. Keeping the vectors as data rather than as Go literals is
// deliberate: they must survive whatever entry-point signature the
// implementation eventually lands (see adapter.go).

// CodecVector is one CF-V0-019 vector.
//
// Mode selects the obligation under test:
//
//	serialize  parse Input and require Codec output to equal Expect exactly
//	invalid    require Parse to reject Input with reason Reason
//	verify     require IsCanonical(Input) to equal Canonical
//	document   require IsCompleteDocument(Input) to equal Canonical
//
// Exactly one of Input and InputHex is present, and for serialize vectors
// exactly one of Expect and ExpectHex.
type CodecVector struct {
	ID           string `json:"id"`
	Mode         string `json:"mode"`
	Clause       string `json:"clause"`
	Note         string `json:"note"`
	Input        string `json:"input"`
	InputHex     string `json:"inputHex"`
	Expect       string `json:"expect"`
	ExpectHex    string `json:"expectHex"`
	ExpectSha256 string `json:"expectSha256"`
	InputSha256  string `json:"inputSha256"`
	Reason       string `json:"reason"`
	Canonical    bool   `json:"canonical"`
}

// InputBytes returns the vector's raw input bytes. Hex encoding is used only
// where the input is not valid UTF-8 or carries a BOM, which no JSON string
// can hold.
func (v CodecVector) InputBytes() ([]byte, error) {
	return oneOf(v.ID, "input", v.Input, v.InputHex)
}

// ExpectBytes returns the vector's hand-authored canonical output bytes.
func (v CodecVector) ExpectBytes() ([]byte, error) {
	return oneOf(v.ID, "expect", v.Expect, v.ExpectHex)
}

func oneOf(id, field, plain, hexed string) ([]byte, error) {
	switch {
	case plain != "" && hexed != "":
		return nil, fmt.Errorf("vector %s: both %s and %sHex are set", id, field, field)
	case hexed != "":
		b, err := hex.DecodeString(hexed)
		if err != nil {
			return nil, fmt.Errorf("vector %s: %sHex: %w", id, field, err)
		}
		return b, nil
	default:
		// An empty plain string is a legitimate value only for inputs that are
		// genuinely empty, which no vector declares; callers treat "" as absent.
		return []byte(plain), nil
	}
}

// CodecVectors is the loaded vectors/codec.json document.
type CodecVectors struct {
	Spec       string        `json:"spec"`
	Clause     string        `json:"clause"`
	Derivation string        `json:"derivation"`
	Vectors    []CodecVector `json:"vectors"`
}

// UniverseVector is one CF-V0-006 universe-identity vector.
type UniverseVector struct {
	ID            string          `json:"id"`
	Clause        string          `json:"clause"`
	Note          string          `json:"note"`
	Preimage      json.RawMessage `json:"preimage"`
	PreimageCodec string          `json:"preimageCodec"`
	Domain        string          `json:"domain"`
	UniverseID    string          `json:"universeId"`
}

// ItemVector is one CF-V0-007 item-identity vector.
type ItemVector struct {
	ID            string `json:"id"`
	Clause        string `json:"clause"`
	Universe      string `json:"universe"`
	Kind          string `json:"kind"`
	SubjectID     string `json:"subjectId"`
	UniverseID    string `json:"universeId"`
	PreimageCodec string `json:"preimageCodec"`
	Domain        string `json:"domain"`
	ItemID        string `json:"itemId"`
}

// FrontierIDVector is the CF-V0-019 document-identity vector. CF-V0-018 prints
// a zero placeholder for `id`; this vector records the real identity of that
// exact document.
type FrontierIDVector struct {
	Clause                 string `json:"clause"`
	Note                   string `json:"note"`
	DocumentWithoutIDCodec string `json:"documentWithoutIdCodec"`
	Domain                 string `json:"domain"`
	PlaceholderIDInSpec    string `json:"frontierIdPlaceholderInSpec"`
	FrontierID             string `json:"frontierId"`
}

// IdentityVectors is the loaded vectors/identity.json document.
type IdentityVectors struct {
	Spec       string           `json:"spec"`
	Derivation string           `json:"derivation"`
	Universes  []UniverseVector `json:"universes"`
	Items      []ItemVector     `json:"items"`
	FrontierID FrontierIDVector `json:"frontierId"`
}

// DomainHash is the domain-separated digest shape shared by CF-V0-006,
// CF-V0-007 and CF-V0-019: SHA-256 over UTF8(domain), one NUL byte, then the
// codec bytes.
func DomainHash(domain string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0x00})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// Sha256Hex is the plain digest used for the vectors' `expectSha256` fields.
func Sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func loadJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// LoadCodecVectors reads vectors/codec.json relative to dir.
func LoadCodecVectors(dir string) (CodecVectors, error) {
	var cv CodecVectors
	err := loadJSON(filepath.Join(dir, "vectors", "codec.json"), &cv)
	return cv, err
}

// LoadIdentityVectors reads vectors/identity.json relative to dir.
func LoadIdentityVectors(dir string) (IdentityVectors, error) {
	var iv IdentityVectors
	err := loadJSON(filepath.Join(dir, "vectors", "identity.json"), &iv)
	return iv, err
}
