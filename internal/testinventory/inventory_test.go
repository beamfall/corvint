// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
)

const (
	fixtureCommit = "0123456789abcdef0123456789abcdef01234567"
	blobA         = "1111111111111111111111111111111111111111"
	blobB         = "2222222222222222222222222222222222222222"
	blobC         = "3333333333333333333333333333333333333333"
)

// sampleInventory is a Go and a Vitest instance plus one failed pytest
// lister, deliberately out of order so Encode must sort it.
func sampleInventory() Inventory {
	return Inventory{
		Profile: Profile,
		Instances: []Instance{
			{
				Key: "vitest@web", Runner: "vitest", Root: "web", Config: "web/vitest.config.ts", ConfigDigest: blobA,
				Lockfile: "package-lock.json", LockfileDigest: blobB,
				Lister: Lister{Name: "vitest-list", Version: "1", Kind: KindExecuting, Command: []string{"node_modules/.bin/vitest", "list", "--json"}},
				Commit: fixtureCommit,
				Files:  []File{{Path: "web/b.test.ts", Blob: blobB}, {Path: "web/a.test.ts", Blob: blobA}},
				Tests: []Test{
					{ID: "vitest:web/b.test.ts::b", File: "web/b.test.ts"},
					{ID: "vitest:web/a.test.ts::sum::adds", File: "web/a.test.ts"},
				},
			},
			{
				Key: "go@.", Runner: "go", Root: ".", Config: "go.mod", ConfigDigest: blobC,
				Lister: Lister{Name: "go-ast", Version: "1", Kind: KindStatic},
				Commit: fixtureCommit,
				Files:  []File{{Path: "calc/calc_test.go", Blob: blobA}},
				Tests:  []Test{{ID: "go:calc::TestAdd", File: "calc/calc_test.go"}},
			},
		},
		Failures: []Failure{{
			Key: "pytest@.", Runner: "pytest", Root: ".", Config: "pytest.ini",
			Lister: Lister{Name: "pytest-collect", Version: "1", Kind: KindExecuting, Command: []string{"python3", "-m", "pytest", "--collect-only", "-q"}},
			Error:  "exit status 2: collection error",
		}},
	}
}

// TIV-V1-001, TIV-V1-003: the inventory round-trips byte for byte, keys
// every test by its canonical ID with the file it lives in, and keys every
// instance `<runner>@<root>` with its config beside it.
func TestTIVV1001RoundTripCanonicalBytes(t *testing.T) {
	data, err := Encode(sampleInventory())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatalf("round trip changed bytes:\n%s\nvs\n%s", data, again)
	}
	if !bytes.HasSuffix(data, []byte("}\n")) || !strings.HasPrefix(string(data), "{\n  \"profile\": \"corvint-test-inventory/0\",\n") {
		t.Fatalf("not canonical layout:\n%s", data)
	}
	if decoded.Instances[0].Key != "go@." || decoded.Instances[1].Key != "vitest@web" {
		t.Fatalf("instances not sorted by key: %+v", decoded.Instances)
	}
	vitest := decoded.Instances[1]
	if vitest.Files[0].Path != "web/a.test.ts" || vitest.Tests[0].ID != "vitest:web/a.test.ts::sum::adds" || vitest.Tests[0].File != "web/a.test.ts" {
		t.Fatalf("files and tests not sorted: %+v", vitest)
	}
	if vitest.Config != "web/vitest.config.ts" || Key(vitest.Runner, vitest.Root) != vitest.Key {
		t.Fatalf("instance identity: %+v", vitest)
	}
}

// TIV-V1-003: several instances of one runner are allowed, each keyed by its
// root; two records with one key are refused.
func TestTIVV1003SeveralInstancesOfOneRunner(t *testing.T) {
	jest := func(root string) Instance {
		return Instance{
			Key: Key("jest", root), Runner: "jest", Root: root, Config: root + "/jest.config.js", ConfigDigest: blobA,
			Lister: Lister{Name: "jest-list-tests", Version: "1", Kind: KindExecuting, Command: []string{"node_modules/.bin/jest", "--listTests"}},
			Commit: fixtureCommit, Files: []File{}, Tests: []Test{},
		}
	}
	inventory := Inventory{Profile: Profile, Instances: []Instance{jest("apps/web"), jest("apps/admin")}, Failures: []Failure{}}
	data, err := Encode(inventory)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil || len(decoded.Instances) != 2 || decoded.Instances[0].Key != "jest@apps/admin" {
		t.Fatalf("decoded %+v, %v", decoded.Instances, err)
	}
	inventory.Instances[1] = jest("apps/web")
	if _, err := Encode(inventory); !errors.Is(err, ErrInvalid) {
		t.Fatalf("repeated key accepted: %v", err)
	}
}

// TIV-V1-012: a member a newer writer added, at the top level and inside a
// record, is ignored by the reader and kept on a round trip.
func TestTIVV1012UnknownAddedFieldPreserved(t *testing.T) {
	data, err := Encode(sampleInventory())
	if err != nil {
		t.Fatal(err)
	}
	newer := bytes.Replace(data, []byte("\"profile\": \"corvint-test-inventory/0\","), []byte("\"profile\": \"corvint-test-inventory/0\",\n  \"shardHints\": {\"go\": 4},"), 1)
	newer = bytes.Replace(newer, []byte("\"key\": \"go@.\","), []byte("\"key\": \"go@.\",\n      \"buildTags\": [\"integration\"],"), 1)
	decoded, err := Decode(newer)
	if err != nil {
		t.Fatalf("newer field refused: %v", err)
	}
	if len(decoded.Instances) != 2 || decoded.Instances[0].Tests[0].ID != "go:calc::TestAdd" {
		t.Fatalf("known fields lost: %+v", decoded)
	}
	out, err := Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"shardHints": {`, `"buildTags": [`, `"integration"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("added field %s not preserved:\n%s", want, out)
		}
	}
	// A change of meaning needs a new profile, which this reader refuses.
	other := bytes.Replace(data, []byte("corvint-test-inventory/0"), []byte("corvint-test-inventory/1"), 1)
	if _, err := Decode(other); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("new profile accepted: %v", err)
	}
}

// TIV-V1-009: Decode refuses every input it cannot read as this profile.
func TestTIVV1009DecodeRefusesInvalid(t *testing.T) {
	data, err := Encode(sampleInventory())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"truncated":         data[:len(data)/2],
		"empty":             nil,
		"not an object":     []byte("[]"),
		"duplicate member":  bytes.Replace(data, []byte(`"profile": "corvint-test-inventory/0",`), []byte(`"profile": "corvint-test-inventory/0", "profile": "corvint-test-inventory/0",`), 1),
		"bad test id":       bytes.Replace(data, []byte(`"go:calc::TestAdd"`), []byte(`"go:calc:TestAdd"`), 1),
		"foreign runner id": bytes.Replace(data, []byte(`"go:calc::TestAdd"`), []byte(`"jest:calc::TestAdd"`), 1),
		"unlisted file":     bytes.Replace(data, []byte(`"file": "calc/calc_test.go"`), []byte(`"file": "other/calc_test.go"`), 1),
		"bad blob":          bytes.Replace(data, []byte(blobC), []byte("xyz"), 1),
		"unknown kind":      bytes.Replace(data, []byte(`"kind": "static"`), []byte(`"kind": "guessing"`), 1),
		"oversize":          append(append([]byte{}, data...), bytes.Repeat([]byte(" "), MaxBytes)...),
		"missing instances": []byte(`{"profile":"corvint-test-inventory/0","failures":[]}`),
	}
	for name, input := range cases {
		if _, err := Decode(input); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Decode = %v, want ErrInvalid", name, err)
		}
	}
}

// TIV-V1-011: Encode refuses an inventory over the cap.
func TestTIVV1011EncodeRefusesOversize(t *testing.T) {
	inventory := sampleInventory()
	instance := &inventory.Instances[1]
	for index := 0; len(instance.Tests)*80 < MaxBytes; index++ {
		instance.Tests = append(instance.Tests, Test{ID: "go:calc::TestAdd" + strings.Repeat("x", 40) + strconv.Itoa(index), File: "calc/calc_test.go"})
	}
	if _, err := Encode(inventory); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Encode = %v, want ErrTooLarge", err)
	}
}
