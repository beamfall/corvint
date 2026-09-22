package roadmap

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureProjectionJSON = `{"association":{"state":"ASSOCIATED","reason":"","anchors":[]},` +
	`"hygiene":{"state":"ELIGIBLE","reason":"","anchors":[]},` +
	`"freshness":{"state":"CURRENT","reason":"","anchors":[]},` +
	`"execution":{"state":"PASSED","reason":"","anchors":[]},` +
	`"strength":{"state":"NOT_MEASURED","reason":"","anchors":[]}}`

func writeReceiptFixture(t *testing.T, dir, requirementID, inputIdentity string) {
	t.Helper()
	content := `{"inputIdentity":"` + inputIdentity + `","projection":` + fixtureProjectionJSON + `}`
	if err := os.WriteFile(filepath.Join(dir, requirementID+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReceiptCurrentWhenIdentityMatches(t *testing.T) {
	dir := t.TempDir()
	writeReceiptFixture(t, dir, "TCP-V0-001", "tree-abc")
	receipt := LoadReceipt(dir, "TCP-V0-001", "tree-abc", false)
	if receipt.State != receiptStateCurrent || receipt.Reason != "" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.Projection == nil || receipt.Projection.Association.State != "ASSOCIATED" {
		t.Fatalf("projection not decoded: %+v", receipt.Projection)
	}
}

func TestLoadReceiptStaleWhenIdentityMismatches(t *testing.T) {
	dir := t.TempDir()
	writeReceiptFixture(t, dir, "TCP-V0-001", "tree-old")
	receipt := LoadReceipt(dir, "TCP-V0-001", "tree-new", false)
	if receipt.State != receiptStateStale {
		t.Fatalf("state = %q, want STALE", receipt.State)
	}
}

func TestLoadReceiptNotObservedWithoutDirectory(t *testing.T) {
	receipt := LoadReceipt("", "TCP-V0-001", "tree-new", false)
	if receipt.State != notObserved || receipt.Reason == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestLoadReceiptNotObservedWithoutFile(t *testing.T) {
	receipt := LoadReceipt(t.TempDir(), "TCP-V0-001", "tree-new", false)
	if receipt.State != notObserved || receipt.Reason == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestLoadReceiptNotObservedOnMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TCP-V0-001.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt := LoadReceipt(dir, "TCP-V0-001", "tree-new", false)
	if receipt.State != notObserved || receipt.Reason == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestLoadReceiptNotObservedWithoutCurrentTreeDigest(t *testing.T) {
	dir := t.TempDir()
	writeReceiptFixture(t, dir, "TCP-V0-001", "tree-abc")
	receipt := LoadReceipt(dir, "TCP-V0-001", "", false)
	if receipt.State != notObserved || receipt.Reason == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func TestLoadReceiptRefusesRequirementIDOutsideDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "receipts")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReceiptFixture(t, parent, "outside", "tree-abc")
	receipt := LoadReceipt(dir, "../outside", "tree-abc", false)
	if receipt.State != notObserved || receipt.Projection != nil || receipt.InputIdentity != "" {
		t.Fatalf("receipt outside the receipts directory was read: %+v", receipt)
	}
}
