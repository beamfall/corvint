package doccorpus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nativeDocsBuild commits one docs page and builds its native corpus.
func nativeDocsBuild(t *testing.T, page string, schema string) (*Artifact, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "page.md"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	git(t, root, "add", "docs")
	git(t, root, "-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "docs")
	m, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "docs", "2026-10-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if schema != "" {
		m.Schema = schema
	}
	return Build(context.Background(), root, m)
}

func TestDCPV1146OversizedParagraphStaysVerbatim(t *testing.T) {
	var table strings.Builder
	table.WriteString("| id | text |\n|---|---|\n")
	for i := 0; table.Len() < 3*maxSpanBytes; i++ {
		fmt.Fprintf(&table, "| %d | %s |\n", i, strings.Repeat("x", 200))
	}
	a, err := nativeDocsBuild(t, "# Index\n\n"+table.String()+"\nAfter.\n", ManifestSchemaV2)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	parts, next := 0, 3
	for _, claim := range a.Claims {
		if !strings.Contains(claim.ID, ":file:docs/page.md:paragraph:") || claim.Text == "After.\n" || strings.HasPrefix(claim.Text, "# Index") {
			continue
		}
		anchor := claim.Evidence.Anchors[0]
		if anchor.Start != next || len(claim.Text) > maxSpanBytes || !strings.HasSuffix(claim.ID, fmt.Sprintf(":paragraph:%d", anchor.Start)) {
			t.Fatalf("part %d: id=%s span=%d-%d bytes=%d, want start %d", parts, claim.ID, anchor.Start, anchor.End, len(claim.Text), next)
		}
		next = anchor.End + 1
		joined.WriteString(claim.Text)
		parts++
	}
	if parts < 3 || joined.String() != table.String() {
		t.Fatalf("oversized paragraph not kept verbatim across %d parts", parts)
	}
}

func TestDCPV1146RefusalsNameTheSpan(t *testing.T) {
	_, err := nativeDocsBuild(t, "# Page\n\n"+strings.Repeat("y", maxSpanBytes+1)+"\n", ManifestSchemaV2)
	if err == nil || !strings.Contains(err.Error(), "vacuous or oversized anchor at docs/page.md:3-3") {
		t.Fatalf("oversized line refusal is not located: %v", err)
	}
	var page strings.Builder
	for i := range MaxRecords + 1 {
		fmt.Fprintf(&page, "Paragraph %d.\n\n", i)
	}
	_, err = nativeDocsBuild(t, page.String(), "")
	if err == nil || !strings.Contains(err.Error(), "native paragraph bound exceeded: more than 4096 records at docs/page.md:") ||
		!strings.Contains(err.Error(), ManifestSchemaV2+" manifest raises the bound") {
		t.Fatalf("record bound refusal is not located: %v", err)
	}
}
