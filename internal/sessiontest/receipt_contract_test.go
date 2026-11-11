// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// receiptContractFixture is the committed corvint-test-run/0 receipt whose bytes the 1.x
// compatibility promise freezes (CMP-V1-003 item 2, docs/COMPATIBILITY.md). Every member,
// including each optional one, is present.
var receiptContractFixture = filepath.Join("testdata", "corvint-test-run-0.contract.json")

// receiptContractMembers is the documented member set of corvint-test-run/0, sorted bytewise, with
// the JSON type of each: a path names a member of the receipt, "[]" an array element. A stable
// format changes only by addition (CMP-V1-004), so a rename or removal here, or of a Go field's
// JSON name, is a breaking change; an addition updates this list and the fixture in the same
// change.
var receiptContractMembers = []string{
	"base string",
	"checks array",
	"checks[] string",
	"completedAt string",
	"excluded array",
	"excluded[] string",
	"graphDigest string",
	"incomplete array",
	"incomplete[] string",
	"invocations array",
	"invocations[] object",
	"invocations[].adapter string",
	"invocations[].adapterManifest string",
	"invocations[].argv array",
	"invocations[].argv[] string",
	"invocations[].dir string",
	"invocations[].elapsedMilliseconds number",
	"invocations[].env array",
	"invocations[].envDigest string",
	"invocations[].env[] string",
	"invocations[].exit number",
	"invocations[].outcomes array",
	"invocations[].outcomes[] object",
	"invocations[].outcomes[].outcome string",
	"invocations[].outcomes[].slice string",
	"invocations[].outcomes[].unit string",
	"invocations[].runner string",
	"invocations[].version string",
	"manifestDigest string",
	"partitionDigest string",
	"placement string",
	"planDigest string",
	"profile string",
	"reason string",
	"result string",
	"revision string",
	"runnerProfile string",
	"scope string",
	"shardCount number",
	"shardIndex number",
	"source object",
	"source.head string",
	"source.paths array",
	"source.paths[] object",
	"source.paths[].digest string",
	"source.paths[].path string",
	"source.tree string",
	"sourceDigest string",
	"summary string",
	"tool string",
}

// contractReceipt is the receipt the fixture encodes.
func contractReceipt() Receipt {
	digest := "sha256:" + strings.Repeat("a", 64)
	return Receipt{
		CompletedAt: "2026-11-11T00:00:00Z",
		Base:        strings.Repeat("1", 40),
		Checks:      []string{"go vet ./..."},
		Excluded:    []string{"go:./tools/x: excluded by declaration"},
		GraphDigest: digest,
		Incomplete:  []string{"declared:ruby: the declared runner was not found"},
		Invocations: []Invocation{{
			Adapter:             AdapterGo,
			AdapterManifest:     "corvint-go-test-provider/1",
			Argv:                []string{"go", "test", "-json", "./internal/x"},
			Dir:                 "nested",
			ElapsedMilliseconds: 1250,
			Env:                 []string{"GOFLAGS="},
			EnvDigest:           digest,
			Exit:                0,
			Outcomes:            []Outcome{{Outcome: ResultPass, Slice: SliceWhole, Unit: "go:./internal/x"}},
			Runner:              "go",
			Version:             "go1.27.3",
		}},
		ManifestDigest:  digest,
		PartitionDigest: digest,
		Placement:       PlacementUniverse,
		PlanDigest:      digest,
		Profile:         ReceiptProfile,
		Reason:          "",
		Result:          ResultIncomplete,
		Revision:        strings.Repeat("2", 40),
		RunnerProfile:   ProfileLocal,
		Scope:           ScopeNarrowed,
		ShardCount:      1,
		ShardIndex:      0,
		Source: SourceIdentity{
			Head:  strings.Repeat("2", 40),
			Paths: []PathDigest{{Digest: digest, Path: "internal/x/x.go"}},
			Tree:  strings.Repeat("3", 40),
		},
		SourceDigest: digest,
		Summary:      "selected units passed: 1 of 2; the full suite runs in CI",
		Tool:         "corvint test",
	}
}

// TestCMPV1003TestRunReceiptContract (CMP-V1-003, CMP-V1-004, STR-V0-010; V1-5434): the encoder
// writes the committed fixture byte for byte, the strict reader reads it back unchanged, and its
// members and their types are exactly the documented set.
func TestCMPV1003TestRunReceiptContract(t *testing.T) {
	want, err := os.ReadFile(receiptContractFixture)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(contractReceipt())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("corvint-test-run/0 encoding differs from %s:\n got %s\nwant %s", receiptContractFixture, encoded, want)
	}
	read, err := ReadReceipt(receiptContractFixture)
	if err != nil || !reflect.DeepEqual(read, contractReceipt()) {
		t.Fatalf("strict reader: %v", err)
	}
	var document any
	if err := json.Unmarshal(want, &document); err != nil {
		t.Fatal(err)
	}
	var got []string
	collectMembers(document, "", &got)
	sort.Strings(got)
	if !reflect.DeepEqual(got, receiptContractMembers) {
		t.Fatalf("corvint-test-run/0 members:\n got %q\nwant %q", got, receiptContractMembers)
	}
	for label, body := range map[string]string{
		"unknown member": strings.Replace(string(want), `"base":`, `"added":1,"base":`, 1),
		"other profile":  strings.Replace(string(want), ReceiptProfile, "corvint-test-run/1", 1),
	} {
		bad := filepath.Join(t.TempDir(), "receipt.json")
		if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadReceipt(bad); err == nil {
			t.Errorf("%s: the strict reader accepted it", label)
		}
	}
}

// collectMembers appends "path type" for value and everything under it.
func collectMembers(value any, path string, out *[]string) {
	kind := "null"
	switch typed := value.(type) {
	case map[string]any:
		kind = "object"
		for name, member := range typed {
			child := name
			if path != "" {
				child = path + "." + name
			}
			collectMembers(member, child, out)
		}
	case []any:
		kind = "array"
		seen := map[string]bool{}
		for _, element := range typed {
			var below []string
			collectMembers(element, path+"[]", &below)
			for _, entry := range below {
				if !seen[entry] {
					seen[entry] = true
					*out = append(*out, entry)
				}
			}
		}
	case string:
		kind = "string"
	case float64:
		kind = "number"
	case bool:
		kind = "boolean"
	}
	if path != "" {
		*out = append(*out, path+" "+kind)
	}
}
