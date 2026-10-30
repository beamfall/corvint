package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// adversarialTermArtifact holds record text whose tokenization depends on camel-case boundaries,
// underscores, case folding, non-ASCII and invalid UTF-8 next to the ID/text join, aliases,
// symbols, shared paths and relations, plus a claim about no subject.
func adversarialTermArtifact() *Artifact {
	anchor := func(path, symbol string) Evidence {
		return Evidence{Anchors: []Anchor{{Path: path, Symbol: symbol}, {Path: path}}}
	}
	return &Artifact{
		Subjects: []Subject{
			{ID: "native:symbol:HTTPServer", Name: "ParseHTTPRequest_v2", AlternateIDs: []string{"alias:a", "alias:a"}, Evidence: anchor("src/a.go", "HTTPServer")},
			{ID: "subjectZ", Name: "İstanbul Ünïcode café", Evidence: anchor("src/b.go", "")},
			{ID: "s3", Name: "bad\xffbytes endsUpper", Evidence: anchor("src/a.go", "")},
		},
		Claims: []Claim{
			{ID: "claimEndsLower", Subject: "native:symbol:HTTPServer", Text: "Upper starts here and_uses-many_separators classes amplify", Evidence: anchor("src/a.go", "x")},
			{ID: "claim2X", Subject: "subjectZ", Text: "\xffStray continuation İ dotted", Evidence: anchor("src/b.go", "")},
			{ID: "orphanClaim", Subject: "no-such-subject", Text: "orphan words never reach subject terms", Evidence: anchor("src/c.go", "")},
			{ID: "c4", Subject: "s3", Text: "ABCDef ghiJKL 123abc", Evidence: anchor("src/a.go", "")},
		},
		Relations: []Relation{
			{ID: "r1", From: "native:symbol:HTTPServer", To: "subjectZ", Evidence: anchor("src/a.go", "")},
			{ID: "r2", From: "s3", To: "s3", Evidence: anchor("src/d.go", "")},
		},
	}
}

// TestDCRV1419FastIndexMatchesReference: the concurrent derivation stores exactly the reference
// derivation's index for corpora of every record family, including text whose term sets depend on
// the ID/text join, and for a corpus large enough to span many worker blocks.
func TestDCRV1419FastIndexMatchesReference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, built, _ := builtFixture(t)
	root, m := adoptionFixture(t, nil)
	adoption, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	padded, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	padIndexCapacity(padded, 3000, 9000, 2000)
	for name, a := range map[string]*Artifact{"built": built, "adoption": adoption, "padded": padded, "adversarial": adversarialTermArtifact()} {
		reference, err := buildQueryIndexReference(ctx, a)
		if err != nil {
			t.Fatalf("%s: reference: %v", name, err)
		}
		fast, ok, err := buildQueryIndexFast(ctx, a)
		if err != nil || !ok {
			t.Fatalf("%s: fast path declined: ok=%v %v", name, ok, err)
		}
		if !reflect.DeepEqual(fast, reference) {
			t.Fatalf("%s: fast index differs from the reference", name)
		}
	}
}

// TestDCRV1419FastIndexDeclinesEveryRefusal: whenever the reference derivation refuses, the
// concurrent derivation declines, so BuildQueryIndex returns the reference refusal unchanged.
func TestDCRV1419FastIndexDeclinesEveryRefusal(t *testing.T) {
	ctx := context.Background()
	edit := func(change func(*Artifact)) *Artifact {
		a := adversarialTermArtifact()
		change(a)
		return a
	}
	cases := map[string]*Artifact{
		"duplicate identity": edit(func(a *Artifact) { a.Claims[1].ID = a.Claims[0].ID }),
		"empty identity":     edit(func(a *Artifact) { a.Relations[0].ID = "" }),
		"identity collision": edit(func(a *Artifact) { a.Relations[0].ID = a.Subjects[0].ID }),
		"long term":          edit(func(a *Artifact) { a.Claims[2].Text = "x" + strings.Repeat("y", 70000) }),
		"long alias":         edit(func(a *Artifact) { a.Subjects[1].AlternateIDs = []string{strings.Repeat("a", 70000)} }),
		"long path":          edit(func(a *Artifact) { a.Claims[3].Evidence.Anchors[0].Path = strings.Repeat("p", 70000) }),
	}
	for name, a := range cases {
		_, ok, err := buildQueryIndexFast(ctx, a)
		_, want := buildQueryIndexReference(ctx, a)
		_, got := BuildQueryIndex(ctx, a)
		if ok || err != nil || want == nil || got == nil || got.Error() != want.Error() {
			t.Errorf("%s: fast ok=%v err=%v; BuildQueryIndex %v, reference %v", name, ok, err, got, want)
		}
	}
	a := adversarialTermArtifact()
	if _, err := buildQueryIndexReference(ctx, a); err != nil {
		t.Fatal(err)
	}
	// The reference refusal names the full observed totals (DCP-V1-194).
	observed := func(postings, keyBytes int) int {
		defer SetIndexBoundsForTest(postings, keyBytes)()
		_, err := buildQueryIndexReference(ctx, a)
		var n, bound int
		if _, scan := fmt.Sscanf(strings.TrimPrefix(strings.TrimPrefix(capacityMessage(t, err), "indexed postings "), "indexed unique key bytes "), "%d exceeds the limit of %d", &n, &bound); scan != nil {
			t.Fatalf("capacity message %v", err)
		}
		return n
	}
	postings, keyBytes := observed(1, MaxIndexKeyBytes), observed(MaxIndexPostings, 1)
	for name, bounds := range map[string][2]int{"postings": {postings - 1, MaxIndexKeyBytes}, "key bytes": {MaxIndexPostings, keyBytes - 1}, "both at the bound": {postings, keyBytes}} {
		restore := SetIndexBoundsForTest(bounds[0], bounds[1])
		_, ok, err := buildQueryIndexFast(ctx, a)
		_, want := buildQueryIndexReference(ctx, a)
		_, got := BuildQueryIndex(ctx, a)
		restore()
		if name == "both at the bound" {
			if !ok || err != nil || got != nil || want != nil {
				t.Errorf("%s: fast ok=%v err=%v; BuildQueryIndex %v, reference %v", name, ok, err, got, want)
			}
			continue
		}
		if ok || err != nil || want == nil || got == nil || got.Error() != want.Error() {
			t.Errorf("%s: fast ok=%v err=%v; BuildQueryIndex %v, reference %v", name, ok, err, got, want)
		}
	}
}

// TestDCRV1419BoundaryScanFindsStringContent: the boundary scan skips strings holding escaped
// quotes, backslashes, brackets and element separators without splitting them, and refuses an
// unterminated string, an escaped member name that is a duplicate, unbalanced element brackets
// and trailing bytes, as the closed decoder does.
func TestDCRV1419BoundaryScanFindsStringContent(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(c *Artifact) {
		c.Claims = append([]Claim{}, c.Claims...)
		c.Claims[0].Text = `quote " backslash \ pair \\" brackets ]},{"id":"x"} [ { end\`
		c.Claims[1].Text = `\"],"relations":[` + "\u2028\x7f"
	})
	decoded, err := decodeArtifactParallel(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want, err := ParseArtifact(data); err != nil || !reflect.DeepEqual(decoded, want) {
		t.Fatalf("boundary scan split string content: %v", err)
	}
	cases := map[string][]byte{
		"unterminated string": data[:bytes.LastIndex(data, []byte(`"`))],
		"escaped duplicate":   bytes.Replace(data, []byte(`{"schema"`), []byte(`{"tree":"t","sch\u0065ma"`), 1),
		"unbalanced element":  bytes.Replace(data, []byte(`"start_line":1`), []byte(`"start_line":[1`), 1),
		"trailing bytes":      append(bytes.Clone(data), 'x'),
		"stray comma":         bytes.Replace(data, []byte(`"claims":[`), []byte(`"claims":[,`), 1),
	}
	for name, bad := range cases {
		_, closed := ParseArtifact(bad)
		if _, err := decodeArtifactParallel(bad, nil); err == nil || closed == nil {
			t.Errorf("%s: parallel=%v closed=%v; both must refuse", name, err, closed)
		}
	}
}
