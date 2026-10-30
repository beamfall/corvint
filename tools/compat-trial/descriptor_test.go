package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func TestExactSchemaAtEveryOwnedObject(t *testing.T) {
	path, _ := fixtureManifest(t)
	data, _ := os.ReadFile(path)
	for _, scope := range []string{"manifest", "task", "old", "build_identity", "stdin"} {
		for _, mutation := range []string{"add", "drop", "null"} {
			t.Run(scope+"-"+mutation, func(t *testing.T) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				target := m
				switch scope {
				case "task":
					target = tasks(m)
				case "old":
					target = tasks(m)["old"].(map[string]any)
				case "build_identity":
					target = tasks(m)["old"].(map[string]any)["build_identity"].(map[string]any)
				case "stdin":
					target = tasks(m)["stdin"].(map[string]any)
				}
				key := ""
				for k := range target {
					key = k
					break
				}
				switch mutation {
				case "add":
					target["extra"] = 1
				case "drop":
					delete(target, key)
				case "null":
					target[key] = nil
				}
				changed, _ := json.Marshal(m)
				var decoded manifest
				if strictDecode(changed, &decoded) == nil {
					t.Fatal("accepted schema mutation")
				}
			})
		}
	}
	for _, name := range []string{"resource.json", "gold.json"} {
		b, _ := os.ReadFile(filepath.Join(filepath.Dir(path), name))
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		m["extra"] = true
		b, _ = json.Marshal(m)
		if name == "resource.json" {
			var v resourceProfile
			if strictDecode(b, &v) == nil {
				t.Fatal("profile extra field accepted")
			}
		} else {
			var v gold
			if strictDecode(b, &v) == nil {
				t.Fatal("gold extra field accepted")
			}
		}
	}
	var m manifest
	if strictDecode(append(data, []byte(" {}")...), &m) == nil {
		t.Fatal("trailing JSON accepted")
	}
	if strictDecode([]byte(`{"source":"a","source":"b"}`), &m) == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestPinnedInputAndSnapshotBounds(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	for _, n := range []int{inputLimit, inputLimit + 1} {
		b := bytes.Repeat([]byte{'x'}, n)
		_ = os.WriteFile(filepath.Join(base, "stdin"), b, 0600)
		task := p.Manifest.Tasks[0]
		task.Stdin.SHA256 = digest(b)
		pt := prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
		want := procgroup.RefusalNone
		if n > inputLimit {
			want = procgroup.RefusalResourceBound
		}
		if pt.Refusal != want {
			t.Fatalf("input %d: %s %s", n, pt.Refusal, pt.Detail)
		}
	}
	for _, n := range []int{fixtureEntryLimit, fixtureEntryLimit + 1} {
		entries := []fileEntry{}
		for i := 0; i < n; i++ {
			entries = append(entries, fileEntry{fmt.Sprintf("f%03d", i), 0644, nil, false})
		}
		b, err := canonicalSnapshot(entries)
		if err != nil {
			t.Fatal(err)
		}
		_, err = readSnapshot(b)
		if (err != nil) != (n > fixtureEntryLimit) {
			t.Fatalf("entries %d: %v", n, err)
		}
	}
	for _, n := range []int{fixtureByteLimit, fixtureByteLimit + 1} {
		b, err := canonicalSnapshot([]fileEntry{{"file", 0644, bytes.Repeat([]byte{'x'}, n), false}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = readSnapshot(b)
		if (err != nil) != (n > fixtureByteLimit) {
			t.Fatalf("bytes %d: %v", n, err)
		}
	}
}
func TestDigestMismatchDistinctFromUnregistered(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	task := p.Manifest.Tasks[0]
	_ = os.WriteFile(filepath.Join(base, "stdin"), []byte("changed"), 0600)
	pt := prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
	if pt.Refusal != procgroup.RefusalDigestMismatch {
		t.Fatalf("%s %s", pt.Refusal, pt.Detail)
	}
	task.Old.ExecutableSHA256 = strings.Repeat("a", 64)
	pt = prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
	if pt.Refusal != procgroup.RefusalFixtureNotRegistered {
		t.Fatalf("%s %s", pt.Refusal, pt.Detail)
	}
	task = p.Manifest.Tasks[0]
	task.SnapshotSHA256 = strings.Repeat("F", 64)
	_ = os.WriteFile(filepath.Join(base, "stdin"), nil, 0600)
	pt = prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
	if pt.Refusal != procgroup.RefusalDescriptorInvalid {
		t.Fatalf("uppercase digest accepted: %s %s", pt.Refusal, pt.Detail)
	}
}
func TestMalformedDigestIsDescriptorInvalidNotUnregistered(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	task := p.Manifest.Tasks[0]
	task.Old.ExecutableSHA256 = strings.Repeat("a", 63)
	pt := prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
	if pt.Refusal != procgroup.RefusalDescriptorInvalid {
		t.Fatalf("%s %s", pt.Refusal, pt.Detail)
	}
}
func TestDigestMismatchForMutatedExecutable(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	task := p.Manifest.Tasks[0]
	mutated := append(append([]byte{}, fixtureBinary...), 'x')
	if err := os.WriteFile(filepath.Join(base, task.Old.Executable), mutated, 0700); err != nil {
		t.Fatal(err)
	}
	pt := prepareTask(base, task, p.Profile, ownedRegistry(fixtureBinary))
	if pt.Refusal != procgroup.RefusalDigestMismatch {
		t.Fatalf("%s %s", pt.Refusal, pt.Detail)
	}
}
func TestProfileConstantsAndEffectGrammar(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	for _, kind := range []string{"create", "write", "delete", "mode"} {
		e := effect{"foo!mode", kind}
		if !validEffect(e) {
			t.Fatal(e)
		}
	}
	for _, path := range []string{"/outside", "../outside", "a/../b", "https://outside", "x\\..\\y"} {
		if validEffect(effect{path, "write"}) {
			t.Fatal(path)
		}
	}
	task := p.Manifest.Tasks[0]
	task.ExpectedEffects = []effect{{"a.txt", "write"}}
	r := p.Profile
	r.AllowedEffects = []effect{{"a.txt", "create"}}
	if got := prepareTask(base, task, r, ownedRegistry(fixtureBinary)); got.Refusal != procgroup.RefusalDescriptorInvalid {
		t.Fatal("effect kinds conflated")
	}
	profilePath := filepath.Join(base, "resource.json")
	original, _ := os.ReadFile(profilePath)
	for _, key := range []string{"repetitions", "input_bytes", "output_bytes_per_stream", "fixture_entries", "fixture_bytes"} {
		var m map[string]any
		_ = json.Unmarshal(original, &m)
		m[key] = m[key].(float64) + 1
		b, _ := json.Marshal(m)
		_ = os.WriteFile(profilePath, b, 0600)
		if _, err := loadManifest(path, ownedRegistry(fixtureBinary)); err == nil {
			t.Fatalf("changed profile %s accepted", key)
		}
	}
}
