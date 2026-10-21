package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nativeDocsBuild commits docs files and builds their native corpus.
func nativeDocsBuild(t *testing.T, files map[string]string, schema string) (*Artifact, error) {
	t.Helper()
	root, m := nativeDocsManifest(t, files, schema)
	return Build(context.Background(), root, m)
}

// nativeDocsManifest commits files under docs/ and pins the inventory, so a rebuild can reuse
// the same revision.
func nativeDocsManifest(t *testing.T, files map[string]string, schema string) (string, Manifest) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, "docs", name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
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
	return root, m
}

func TestDCPV1146OversizedParagraphStaysVerbatim(t *testing.T) {
	t.Parallel()
	var table strings.Builder
	table.WriteString("| id | text |\n|---|---|\n")
	for i := 0; table.Len() < 3*maxSpanBytes; i++ {
		fmt.Fprintf(&table, "| %d | %s |\n", i, strings.Repeat("x", 200))
	}
	a, err := nativeDocsBuild(t, map[string]string{"page.md": "# Index\n\n" + table.String() + "\nAfter.\n"}, ManifestSchemaV2)
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

// An over-bound line is a located not-collected gap; the rest of the corpus still builds
// (GitHub #1242).
func TestDCPV1146OversizedLineIsAGap(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("z", maxSpanBytes-1) + "\n"
	files := map[string]string{
		"a.md":     "# A\n\nNormal prose.\n",
		"big.md":   "# Big\n\n" + strings.Repeat("y", maxSpanBytes+1) + "\nTail line.\n",
		"top.md":   strings.Repeat("w", maxSpanBytes+1) + "\n\nSecond paragraph.\n",
		"only.md":  strings.Repeat("v", maxSpanBytes+1) + "\n",
		"exact.md": exact,
	}
	root, m := nativeDocsManifest(t, files, ManifestSchemaV2)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]string{}
	for _, c := range a.Claims {
		claims[c.ID] = c.Text
	}
	gaps := map[string]string{}
	for _, g := range a.Gaps {
		if g.Kind == "not-collected" {
			gaps[g.Subject] += g.Reason + ";"
		}
	}
	bound := fmt.Sprintf("%d-byte anchor bound", maxSpanBytes)
	for id, want := range map[string]string{
		"native:file:docs/a.md:paragraph:3":     "Normal prose.\n",
		"native:file:docs/big.md:paragraph:1":   "# Big\n",
		"native:file:docs/big.md:paragraph:4":   "Tail line.\n",
		"native:file:docs/top.md:excerpt":       "Second paragraph.",
		"native:file:docs/top.md:paragraph:3":   "Second paragraph.\n",
		"native:file:docs/exact.md:paragraph:1": exact,
	} {
		if claims[id] != want {
			t.Errorf("claim %s = %.40q, want %.40q", id, claims[id], want)
		}
	}
	for id, want := range map[string]string{
		"native:file:docs/big.md":  "paragraph line 3 exceeds the " + bound,
		"native:file:docs/top.md":  "paragraph line 1 exceeds the " + bound,
		"native:file:docs/only.md": "no line within the " + bound,
	} {
		if !strings.Contains(gaps[id], want) {
			t.Errorf("gap %s = %q, want %q", id, gaps[id], want)
		}
	}
	if _, ok := claims["native:file:docs/only.md:excerpt"]; ok {
		t.Error("file with no anchorable line produced a claim")
	}
	again, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := Encode(a)
	second, _ := Encode(again)
	if a.SHA256 != again.SHA256 || !bytes.Equal(first, second) {
		t.Fatal("gap path is not deterministic")
	}
}

func TestDCPV1146RefusalsNameTheSpan(t *testing.T) {
	t.Parallel()
	_, err := nativeDocsBuild(t, map[string]string{"x.go": "package x\n\nfunc F() string { return \"" + strings.Repeat("y", maxSpanBytes) + "\" }\n"}, ManifestSchemaV2)
	if err == nil || !strings.Contains(err.Error(), "vacuous or oversized anchor at docs/x.go:3-3") {
		t.Fatalf("oversized anchor refusal is not located: %v", err)
	}
	var page strings.Builder
	for i := range MaxRecords + 1 {
		fmt.Fprintf(&page, "Paragraph %d.\n\n", i)
	}
	_, err = nativeDocsBuild(t, map[string]string{"page.md": page.String()}, "")
	if err == nil || !strings.Contains(err.Error(), "native paragraph bound exceeded: more than 4096 records at docs/page.md:") ||
		!strings.Contains(err.Error(), ManifestSchemaV2+" manifest raises the bound") {
		t.Fatalf("record bound refusal is not located: %v", err)
	}
	var decls strings.Builder
	decls.WriteString("package x\n")
	for i := range MaxRecords {
		fmt.Fprintf(&decls, "func F%d() {}\n", i)
	}
	_, err = nativeDocsBuild(t, map[string]string{"x.go": decls.String()}, "")
	if err == nil || !strings.Contains(err.Error(), "native record bound exceeded: more than 4096 records at docs/x.go:") ||
		!strings.Contains(err.Error(), ManifestSchemaV2+" manifest raises the bound") {
		t.Fatalf("native record bound refusal is not located: %v", err)
	}
}
