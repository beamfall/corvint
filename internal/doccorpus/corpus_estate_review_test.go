package doccorpus

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestDCIV1007RubyExpressionsAreNoNames: a gemspec name or dependency and a Gemfile gem whose
// argument is an expression or an interpolated string derive no package and no reference; each
// is named in one inventory-evidence-unreadable gap instead, so a literal prefix of an expression
// never becomes a joinable name.
func TestDCIV1007RubyExpressionsAreNoNames(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "svc", map[string]string{
		"lib/acme-lib.gemspec": "Gem::Specification.new do |spec|\n  spec.name = \"acme\" + \"-lib\"\n  spec.add_dependency \"ra\" + \"ck\"\n  spec.add_dependency \"x#{v}\"\n  spec.add_dependency 'thor', '~> 1'\nend\n",
		"Gemfile":              "gem \"acme-\" + suffix\ngem \"a#{b}\"\ngem 'rails', '~> 7' # web\ngem(\"rake\")\n",
	}, []string{"Gemfile", "lib"}, &RepositoryInventory{})
	for _, s := range a.Subjects {
		if s.Kind == "package" {
			t.Fatalf("an expression name should derive no gem: %+v", s)
		}
	}
	names := []string{}
	for _, c := range a.Claims {
		if strings.Contains(c.ID, ":reference:") {
			names = append(names, c.ID[strings.LastIndex(c.ID, ":")+1:])
		}
	}
	slices.Sort(names)
	if strings.Join(names, ",") != "rails,rake,thor" {
		t.Fatalf("only literal names should be references: %v", names)
	}
	reasons := gapReasons(t, a, "inventory-evidence-unreadable")
	joined := strings.Join(reasons, "\n")
	if len(reasons) != 2 || !strings.Contains(joined, "gemspec name at line 2 is an expression") || !strings.Contains(joined, "gemspec dependency at line 3, 4 is an expression") || !strings.Contains(joined, "Gemfile gem at line 1, 2 is an expression") {
		t.Fatalf("each expression should be named as unreadable evidence: %v", reasons)
	}
}

// TestDCIV1003EstateGapsAreMatchedNotSplit: a reference gap is matched to its claim by subject,
// claim ID and text, so an admitted path holding ": consumes " keeps its reference, and a gap
// naming no reference claim is an unknown entry instead of being dropped.
func TestDCIV1003EstateGapsAreMatchedNotSplit(t *testing.T) {
	t.Parallel()
	id := "native:reference:a: consumes b/Dockerfile:1:image:x"
	text := "consumes container image x at a: consumes b/Dockerfile:1"
	a := &Artifact{
		Claims: []Claim{{ID: id, Subject: "owner", Text: text}},
		Gaps: []Gap{
			{"owner", "outside-corpus-reference", id + ": " + text + "; no covered repository declares x"},
			{"owner", "first-party-reference-outside-corpus", "native:reference:gone: consumes npm package y at gone; under a prefix"},
		},
	}
	outside, unknown := estateOutside("c/corpus.json", a)
	if len(outside) != 1 || outside[id] != "ungraded" {
		t.Fatalf("the reference whose path holds the separator should be kept: %v", outside)
	}
	if len(unknown) != 1 || !strings.Contains(unknown[0], "first-party-reference-outside-corpus gap on owner names no reference claim") {
		t.Fatalf("an unmatched gap should be an unknown entry: %v", unknown)
	}
}

// TestDCIV1001EstateTargetBudgetAndStrictFile: the receipt's join targets are budgeted before any
// reference is expanded, and an exceeded budget refuses naming the reference; the estate file is
// one whole document with no trailing data or repeated member.
func TestDCIV1001EstateTargetBudgetAndStrictFile(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	for _, lib := range []string{"lib1", "lib2"} {
		inventoryRepo(t, parent, lib, map[string]string{"acme.gemspec": "Gem::Specification.new do |s|\n  s.name = \"acme\"\nend\n"}, []string{"acme.gemspec"}, &RepositoryInventory{})
	}
	inventoryRepo(t, parent, "app", map[string]string{"Gemfile": "gem \"acme\"\n"}, []string{"Gemfile"}, &RepositoryInventory{})
	in := EstateInput{Schema: EstateSchema, Corpora: []EstateEntry{{Root: "app", Artifact: "corpus.json"}, {Root: "lib1", Artifact: "corpus.json"}, {Root: "lib2", Artifact: "corpus.json"}}}
	if r, err := estate(context.Background(), parent, in, 2); err != nil || findEstateReference(t, r, "gem", "acme").State != "ambiguous" {
		t.Fatalf("two targets fit a budget of two: %v", err)
	}
	_, err := estate(context.Background(), parent, in, 1)
	var e *Error
	if !errors.As(err, &e) || e.Code != CapacityCode || !strings.HasPrefix(e.Message, "estate corpus app/corpus.json reference native:reference:Gemfile:1:gem:acme: estate reference target count 2 exceeds the limit of 1") {
		t.Fatalf("an exceeded target budget should refuse naming the reference: %v", err)
	}
	for _, bad := range []string{
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":".","artifact":"a.json"}]} {}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":".","artifact":"a.json"}]} x`,
		`{"schema":"corvint-corpus-estate/1","schema":"corvint-corpus-estate/1","corpora":[{"root":".","artifact":"a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":".","artifact":"a.json"}],"corpora":[{"root":"b","artifact":"a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":".","root":"b","artifact":"a.json"}]}`,
	} {
		if _, err := ParseEstate([]byte(bad)); err == nil {
			t.Fatalf("estate %s should be refused", bad)
		}
	}
}
