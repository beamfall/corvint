package genesis

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalUTF8MatchesGenesisPythonSpelling(t *testing.T) {
	encoded, err := canonicalUTF8(map[string]any{"z": "<>&\u2028😀", "a": "é"})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("{\"a\":\"é\",\"z\":\"<>&\u2028😀\"}")
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded=%q want=%q", encoded, want)
	}
}

func TestNormalizeExclusionsMatchesCompilerContract(t *testing.T) {
	got, err := normalizeExclusions([]string{"vendor/z", "docs/private"}, 1024)
	if err != nil || !reflect.DeepEqual(got, []string{"docs/private", "vendor/z"}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, values := range [][]string{{"vendor", "vendor"}, {"../outside"}, {"trailing/"}, {"bad\\path"}} {
		if _, err := normalizeExclusions(values, 1024); err == nil {
			t.Fatalf("accepted exclusions %q", values)
		}
	}
}

func TestParseTreePreservesRawPathEvidenceAndBudget(t *testing.T) {
	oidA := strings.Repeat("a", 40)
	oidB := strings.Repeat("b", 40)
	raw := []byte("100644 blob " + oidA + " 4\tbad\npath\x00" +
		"100755 blob " + oidB + " 5\tsrc/main.go\x00")
	entries, tracked, err := parseTree(raw, 1024, "sha1", 1)
	if err != nil || tracked != 2 || len(entries) != 1 {
		t.Fatalf("entries=%#v tracked=%d err=%v", entries, tracked, err)
	}
	if entries[0].Path != nil || entries[0].PathSHA256 == "" {
		t.Fatalf("unsafe raw path evidence=%#v", entries[0])
	}
}

func TestInvalidReceiptAndSummaryAreSealed(t *testing.T) {
	authority := "repo:fixture"
	receipt := invalidReceipt("init", &authority, "invalid-exclusions", defaultLimits())
	if !VerifyInventoryReceipt(receipt) || receipt["operationalState"] != "INVALID" {
		t.Fatalf("receipt=%#v", receipt)
	}
	summary, err := SummarizeInventoryReceipt(receipt, 5)
	if err != nil {
		t.Fatal(err)
	}
	if summary["summaryReceiptId"] == nil || summary["receiptId"] != receipt["receiptId"] || summary["sourceSamples"] == nil {
		t.Fatalf("summary=%#v", summary)
	}
	if _, present := summary["entries"]; present {
		t.Fatal("summary leaked full entries")
	}
}

func TestSemanticFrontierDoesNotClaimAbsenceOverOmittedEntries(t *testing.T) {
	counts := zeroCounts(SourceClassOrder())
	for _, value := range semanticFrontier(counts, 1) {
		if reason := value.(map[string]any)["reason"]; reason == "declared-source-absent" {
			t.Fatalf("absence claimed over an entry-budget-limited inventory: %#v", value)
		}
	}
	if complete := semanticFrontier(counts, 0); len(complete) != 5 {
		t.Fatalf("complete inventory frontier=%#v", complete)
	}
}
