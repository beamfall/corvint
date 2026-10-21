package main

// CEM-V1-226..229 portable conformance: every case of the shared stable
// tamper matrix under protocol/cem-1.0/stable/tamper-matrix returns its pinned
// verdict. internal/cem/verify asserts the same EXPECTED.json natively.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// tamperMatrixClasses are the tamper classes the shared matrix must cover;
// the native verifier checks the same list.
var tamperMatrixClasses = []string{"control", "PatchDigestMismatch", "FabricatedEvidenceID", "UncitedHunk", "InvalidSpan",
	"SpanOutOfRange", "EvidenceIDSwap", "EvidenceDrop", "HunkReorder", "SpanRangeEdit"}

type tamperMatrixCase struct {
	Name       string         `json:"name"`
	Tamper     string         `json:"tamper"`
	Map        string         `json:"map"`
	Exit       int            `json:"exit"`
	Outcome    string         `json:"outcome"`
	Stage      string         `json:"stage"`
	Code       *string        `json:"code"`
	IssueCodes []string       `json:"issueCodes"`
	Citation   map[string]any `json:"citation"`
}

// TestCEMV1226TamperMatrixPortable runs every case of the shared stable tamper
// matrix through the portable canonical verifier and asserts its pinned
// verdict: exit, outcome, stage, closed code and issue codes, and for an
// ACCEPT the whole citation block.
func TestCEMV1226TamperMatrixPortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := stableCaseFiles(t, "tamper-matrix")
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Profile        string             `json:"profile"`
		ArtifactRoot   string             `json:"artifactRoot"`
		BaseRevision   string             `json:"baseRevision"`
		TargetRevision string             `json:"targetRevision"`
		Cases          []tamperMatrixCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-tamper-matrix/1" || want.ArtifactRoot != "." {
		t.Fatalf("EXPECTED.json profile %q artifactRoot %q", want.Profile, want.ArtifactRoot)
	}
	covered := map[string]bool{}
	for _, tc := range want.Cases {
		covered[tc.Tamper] = true
	}
	for _, class := range tamperMatrixClasses {
		if !covered[class] {
			t.Errorf("tamper matrix has no %s case", class)
		}
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	if len(tamperMatrixVerdictDigests) != len(want.Cases) {
		t.Fatalf("%d pinned verdict digests for %d cases", len(tamperMatrixVerdictDigests), len(want.Cases))
	}
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapBytes, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, artifacts, nil)
			got := neutralResultMap(t, r)
			var gotCitation map[string]any
			if v, ok := got["citation"].(map[string]any); ok {
				gotCitation = v
			}
			if exit != tc.Exit || r.Outcome != tc.Outcome || r.Stage != tc.Stage || !reflect.DeepEqual(r.Code, tc.Code) ||
				!reflect.DeepEqual(append([]string{}, r.IssueCodes...), append([]string{}, tc.IssueCodes...)) || !reflect.DeepEqual(gotCitation, tc.Citation) {
				b, _ := json.Marshal(r)
				t.Fatalf("runStableCanonical exit=%d result=%s, want the pinned %s verdict", exit, b, tc.Name)
			}
			if digest, _ := got["verdictSha256"].(string); digest != tamperMatrixVerdictDigests[tc.Name] {
				t.Fatalf("verdictSha256 = %s, want the pinned %s", digest, tamperMatrixVerdictDigests[tc.Name])
			}
		})
	}
}

// tamperMatrixVerdictDigests pins every case's verdictSha256 (V1-1329,
// CEM-V1-243). The native and the portable test carry the same table, so both
// consumers must recompute the identical verdict for the same sealed case,
// including the two dropped-hunk REJECTs; EXPECTED.json is sealed and stays
// unchanged.
var tamperMatrixVerdictDigests = map[string]string{
	"basis-pair-duplicated":                 "f44b55e8f89f1a9820033f9561377fc320354b07b38a1b5d4372d4d3cbdce6a9",
	"clean":                                 "7b6aff25704b457f8b75040c50439236f0f4de5c1aa98e73d470e0e2a044d5c3",
	"discrimination-at-bound":               "a6f26e3c64455b546a66eab3d810eb888d410c9acb48fecc05373636b40c20f7",
	"discrimination-mutants-over-bound":     "b7ee8273c027b390fe475ac0bf51ace2c9cddad4c23550d990fb718983c35af3",
	"discrimination-survivors-inconsistent": "f8ead615c72cdb6414c07d6c4b2db5f0d267266617232012c6427abdffef5d94",
	"evidence-dropped":                      "ff7e7003636591d3d87e5ced80dd8232cdba7228e224366377eea045501a5732",
	"evidence-ids-swapped":                  "d1c0f18134ef63cd11b5505216ab957b1ff14d773cd4e09fc4fe80b5db45eced",
	"extra-basis":                           "134928175e2c3b60925398c95d31a87769094d45a4af2437b92c9c013f1e33b9",
	"fabricated-evidence-id":                "46ccbe4c2b63ff9df6691f981cbebeb7174df0f86cf8a2be1409de2b933c93b7",
	"hunk-dropped":                          "d3335f5a15b1d4078a47c2447d8d2d7e57b679cc67a92bf57dcb608b1f9d1dc9",
	"hunk-dropped-evidence-kept":            "e420f1590f65af7de8693a0e9eedfd3688488bdcf3c401e6b2cbbf3695fe98db",
	"hunk-id-forged":                        "c2025f9925b218ec03a34ef1366296375303a525c350d0089f3da6549ad13fd1",
	"hunk-ranges-moved":                     "0d249142d4daad4163434f2632eb1876100f80820203f6c7782f7446417204a8",
	"hunk-reorder":                          "76f4e615ab9d4e0d1a99ef6dbdb7e57cfa3290d4f39e80e47c2a8def54793931",
	"mechanical-unproven":                   "0dacac16480905dda9cce12e4eca79a6f8c1280878c2e0388165e307b7424511",
	"patch-digest-mismatch":                 "b52f8549e4d960c4673a558f57ce378e7ddfdafaf4dc4f06588f36593eb0f57e",
	"producer-tier-field":                   "ab6351ab3602a74eea4a131dbd5a69044c39743c0ac83ce8c66b008329836a7a",
	"span-empty":                            "f62c44e1b41f8f4c91ff5666533dabed84311a351b382dffb8be7cf0d63dc518",
	"span-end-edited":                       "429ba9ebc1923990980ff6aa09681c3e538c410a64818ee66ed200010ed8bb0f",
	"span-hash-mismatch":                    "0d267ff6392a9e291d5a9a8f1f59ef50ccafd8df3dbda70d872a286e8a14ec0d",
	"span-hash-recomputed":                  "fbd59cd4b266fdcbe338f8f1df92a6a0c05499b0cf31a2ebb1f87df6fb02af15",
	"span-out-of-range":                     "8a10347c4186fe4ce9f117b2c21140f9d749eaf6a3d12476a0720459d2bb2f69",
	"span-reversed":                         "f419cf238cffef2ca1b96f85cce7ad042e4ddf86c146f37579df31239e3cd9b1",
}
