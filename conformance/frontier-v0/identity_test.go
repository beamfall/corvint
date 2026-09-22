// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"regexp"
	"strings"
	"testing"
)

var (
	universeIDPattern = regexp.MustCompile(`^frontier-universe:sha256:[0-9a-f]{64}$`)
	itemIDPattern     = regexp.MustCompile(`^frontier-item:sha256:[0-9a-f]{64}$`)
	frontierIDPattern = regexp.MustCompile(`^frontier:sha256:[0-9a-f]{64}$`)
)

func loadIdentityVectorsT(t *testing.T) IdentityVectors {
	t.Helper()
	iv, err := LoadIdentityVectors(".")
	if err != nil {
		t.Fatalf("load identity vectors: %v", err)
	}
	return iv
}

// TestUniverseIDVectors re-derives every CF-V0-006 universe ID from its recorded
// preimage using the independent reference codec and the domain-separated hash,
// and requires it to equal the hand-derived value in the vector file.
func TestUniverseIDVectors(t *testing.T) {
	iv := loadIdentityVectorsT(t)
	if len(iv.Universes) < 4 {
		t.Fatalf("expected at least four universe vectors, got %d", len(iv.Universes))
	}
	seen := map[string]string{}
	for _, u := range iv.Universes {
		u := u
		t.Run(u.ID, func(t *testing.T) {
			if u.Domain != "corvint-frontier-universe/0" {
				t.Fatalf("CF-V0-006 fixes the domain to corvint-frontier-universe/0, got %q", u.Domain)
			}
			// The recorded preimage codec must itself be canonical: the ID is
			// defined over codec output, not over arbitrary equivalent JSON.
			ok, err := IsCanonical([]byte(u.PreimageCodec))
			if err != nil || !ok {
				t.Fatalf("preimageCodec is not canonical: ok=%v err=%v", ok, err)
			}
			// The structured preimage and its codec form must agree.
			val, err := Parse(u.Preimage)
			if err != nil {
				t.Fatalf("preimage does not parse under the codec: %v", err)
			}
			if got := string(Codec(val)); got != u.PreimageCodec {
				t.Fatalf("preimage and preimageCodec disagree\n want %q\n  got %q", u.PreimageCodec, got)
			}
			want := "frontier-universe:sha256:" + DomainHash(u.Domain, []byte(u.PreimageCodec))
			if want != u.UniverseID {
				t.Fatalf("hand-derived universe ID does not reproduce\n want %q\n  got %q", u.UniverseID, want)
			}
			if !universeIDPattern.MatchString(u.UniverseID) {
				t.Fatalf("universe ID does not match its profile grammar: %q", u.UniverseID)
			}
			if prev, dup := seen[u.UniverseID]; dup {
				t.Fatalf("universes %s and %s collide on one ID", prev, u.ID)
			}
			seen[u.UniverseID] = u.ID
		})
	}
}

// TestUniverseIdentitySensitivity asserts the two matrix rows about identity:
// a patch or intent change yields a NEW universe, and everything else about the
// evidence does not enter the preimage at all.
func TestUniverseIdentitySensitivity(t *testing.T) {
	iv := loadIdentityVectorsT(t)
	byID := map[string]UniverseVector{}
	for _, u := range iv.Universes {
		byID[u.ID] = u
	}
	b, okB := byID["B-distinct"]
	c, okC := byID["C-patch-changed"]
	d, okD := byID["D-intent-span-changed"]
	if !okB || !okC || !okD {
		t.Fatal("the B/C/D universe vectors are required for the identity-sensitivity rows")
	}
	if b.UniverseID == c.UniverseID {
		t.Error("CF-V0-006: changing patchSha256 MUST change the universe ID")
	}
	if b.UniverseID == d.UniverseID {
		t.Error("CF-V0-006: changing an intent offset MUST change the universe ID")
	}
	// CF-V0-006 lists exactly which fields enter the preimage. Map, LRF, TCQ,
	// command, observation and report identities are deliberately absent, which
	// is what makes item IDs survive evidence repair.
	forbidden := []string{"lrfSha256", "ocmSha256", "cemSha256", "tcqId", "mapSha256",
		"commandSha256", "observationSha256", "reportSha256", "policy", "testMode"}
	for _, u := range iv.Universes {
		for _, f := range forbidden {
			if strings.Contains(u.PreimageCodec, `"`+f+`"`) {
				t.Errorf("%s: %q must not enter the CF-V0-006 preimage; evidence identity is an "+
					"attempt to satisfy the universe, not part of it", u.ID, f)
			}
		}
	}
}

// TestItemIDVectors re-derives every CF-V0-007 item ID.
func TestItemIDVectors(t *testing.T) {
	iv := loadIdentityVectorsT(t)
	if len(iv.Items) == 0 {
		t.Fatal("no item vectors")
	}
	seen := map[string]string{}
	kinds := map[string]bool{}
	for _, it := range iv.Items {
		it := it
		t.Run(it.ID, func(t *testing.T) {
			if it.Domain != "corvint-frontier-item/0" {
				t.Fatalf("CF-V0-007 fixes the domain to corvint-frontier-item/0, got %q", it.Domain)
			}
			if !itemKinds[it.Kind] {
				t.Fatalf("kind %q is outside CF-V0-007", it.Kind)
			}
			kinds[it.Kind] = true
			ok, err := IsCanonical([]byte(it.PreimageCodec))
			if err != nil || !ok {
				t.Fatalf("preimageCodec is not canonical: ok=%v err=%v", ok, err)
			}
			// CF-V0-007's preimage is exactly {kind, subjectId, universeId}.
			val, err := Parse([]byte(it.PreimageCodec))
			if err != nil {
				t.Fatalf("preimage does not parse: %v", err)
			}
			if val.Kind != KindObject || len(val.Obj) != 3 {
				t.Fatalf("CF-V0-007 preimage has exactly three keys, got %d", len(val.Obj))
			}
			for _, m := range val.Obj {
				switch m.Key {
				case "kind", "subjectId", "universeId":
				default:
					t.Fatalf("unexpected preimage key %q", m.Key)
				}
			}
			want := "frontier-item:sha256:" + DomainHash(it.Domain, []byte(it.PreimageCodec))
			if want != it.ItemID {
				t.Fatalf("hand-derived item ID does not reproduce\n want %q\n  got %q", it.ItemID, want)
			}
			if !itemIDPattern.MatchString(it.ItemID) {
				t.Fatalf("item ID does not match its profile grammar: %q", it.ItemID)
			}
			if prev, dup := seen[it.ItemID]; dup {
				t.Fatalf("items %s and %s collide on one ID", prev, it.ID)
			}
			seen[it.ItemID] = it.ID
		})
	}
	for _, k := range []string{"HUNK_BASIS", "INTENT_CHANGE", "INTENT_TEST"} {
		if !kinds[k] {
			t.Errorf("no item vector covers kind %s", k)
		}
	}
}

// TestItemIDDependsOnKindNotOnlySubject catches the concrete implementation slip
// this vector set exists to expose: INTENT_CHANGE and INTENT_TEST share a
// subjectId (the obligation ID), so an implementation that hashes only the
// subject and universe would emit two items with one identity.
func TestItemIDDependsOnKindNotOnlySubject(t *testing.T) {
	iv := loadIdentityVectorsT(t)
	type key struct{ universe, subject string }
	byKey := map[key]map[string]string{}
	for _, it := range iv.Items {
		k := key{it.Universe, it.SubjectID}
		if byKey[k] == nil {
			byKey[k] = map[string]string{}
		}
		byKey[k][it.Kind] = it.ItemID
	}
	checked := 0
	for k, kinds := range byKey {
		change, hasChange := kinds["INTENT_CHANGE"]
		test, hasTest := kinds["INTENT_TEST"]
		if !hasChange || !hasTest {
			continue
		}
		checked++
		if change == test {
			t.Errorf("universe %s subject %s: INTENT_CHANGE and INTENT_TEST share one item ID",
				k.universe, k.subject)
		}
	}
	if checked == 0 {
		t.Fatal("no universe carries both intent kinds for one subject, so this invariant is untested")
	}
}

// TestFrontierIDVector re-derives the CF-V0-019 document identity. CF-V0-018
// prints a zero placeholder for `id`; this asserts the real value for that exact
// document and that the placeholder is not it.
func TestFrontierIDVector(t *testing.T) {
	iv := loadIdentityVectorsT(t)
	f := iv.FrontierID
	if f.Domain != "corvint-frontier/0" {
		t.Fatalf("CF-V0-019 fixes the domain to corvint-frontier/0, got %q", f.Domain)
	}
	ok, err := IsCanonical([]byte(f.DocumentWithoutIDCodec))
	if err != nil || !ok {
		t.Fatalf("documentWithoutIdCodec is not canonical: ok=%v err=%v", ok, err)
	}
	val, err := Parse([]byte(f.DocumentWithoutIDCodec))
	if err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, m := range val.Obj {
		if m.Key == "id" {
			t.Fatal("CF-V0-019 hashes the document WITHOUT its id; the recorded preimage still carries one")
		}
	}
	// CF-V0-018: the top-level keys are exactly these seven, so the preimage has
	// exactly six.
	want := map[string]bool{"frontierState": true, "inputs": true, "items": true,
		"profile": true, "scope": true, "universeId": true}
	if len(val.Obj) != len(want) {
		t.Fatalf("expected %d keys in the id preimage, got %d", len(want), len(val.Obj))
	}
	for _, m := range val.Obj {
		if !want[m.Key] {
			t.Fatalf("unexpected top-level key %q", m.Key)
		}
	}
	got := "frontier:sha256:" + DomainHash(f.Domain, []byte(f.DocumentWithoutIDCodec))
	if got != f.FrontierID {
		t.Fatalf("hand-derived frontier ID does not reproduce\n want %q\n  got %q", f.FrontierID, got)
	}
	if !frontierIDPattern.MatchString(f.FrontierID) {
		t.Fatalf("frontier ID does not match its profile grammar: %q", f.FrontierID)
	}
	if f.FrontierID == f.PlaceholderIDInSpec {
		t.Fatal("the spec's zero placeholder is not a real identity and must not equal the derived one")
	}
}

func TestImplementationIdentityMatchesVectors(t *testing.T) {
	if registeredIdentifier == nil {
		t.Skip(unboundReason)
	}
	iv := loadIdentityVectorsT(t)
	for _, u := range iv.Universes {
		u := u
		t.Run("universe/"+u.ID, func(t *testing.T) {
			got, err := registeredIdentifier.UniverseID([]byte(u.PreimageCodec))
			if err != nil {
				t.Fatalf("implementation UniverseID: %v", err)
			}
			if got != u.UniverseID {
				t.Fatalf("implementation universe ID differs from the hand-derived vector\n want %q\n  got %q",
					u.UniverseID, got)
			}
		})
	}
	for _, it := range iv.Items {
		it := it
		t.Run("item/"+it.ID, func(t *testing.T) {
			got, err := registeredIdentifier.ItemID([]byte(it.PreimageCodec))
			if err != nil {
				t.Fatalf("implementation ItemID: %v", err)
			}
			if got != it.ItemID {
				t.Fatalf("implementation item ID differs from the hand-derived vector\n want %q\n  got %q",
					it.ItemID, got)
			}
		})
	}
}
