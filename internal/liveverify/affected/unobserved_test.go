package affected

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestUnobservedDirectoriesAreValidatedAndDigested_AFPV0191 pins the seam
// contract of Result.Unobserved: entries are canonical relative directories in
// strictly ascending order, they are part of the graph digest, and a graph
// without them keeps the digest it had before the field existed.
func TestUnobservedDirectoriesAreValidatedAndDigested_AFPV0191(t *testing.T) {
	for name, directories := range map[string][]string{
		"absolute":  {"/nested"},
		"dot":       {"."},
		"unsorted":  {"b", "a"},
		"duplicate": {"a", "a"},
		"empty":     {""},
		"dot-dot":   {"a/../b"},
	} {
		language := chain()
		language.unobserved = directories
		if _, err := Build(context.Background(), t.TempDir(), language); !errors.Is(err, ErrInvalidLanguage) {
			t.Errorf("%s: err = %v, want ErrInvalidLanguage", name, err)
		}
	}
	plain, err := Build(context.Background(), t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	body, err := plain.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || bytes.Contains(body, []byte(`"unobserved"`)) {
		t.Fatalf("a graph without unobserved directories carries the member: %s", body)
	}
	language := chain()
	language.unobserved = []string{"nested"}
	marked, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Digest() == plain.Digest() {
		t.Fatal("unobserved directories are not part of the digest")
	}
	plan := Select(marked, []string{"nested/new.x", "other/new.x"})
	want := []Unknown{
		{Reason: UnknownLanguageFrontier, Detail: "x:unobserved-directory:nested:nested/new.x"},
		{Reason: UnknownUnindexedSourcePath, Detail: "other/new.x"},
	}
	if !reflect.DeepEqual(plan.Unknown, want) || plan.Scope != ScopeUnknown {
		t.Fatalf("scope = %s unknown = %+v, want UNKNOWN with %+v", plan.Scope, plan.Unknown, want)
	}
}

// TestRunnableDirectoriesAreValidatedAndDigested_AFPV0650 pins the seam
// contract of Result.Runnable: entries are strictly ascending members of the
// same plugin's Unobserved list, they are part of the graph digest, and a
// dirty path in one is the unobserved-module frontier while a deeper
// unobserved directory that is not runnable keeps its own unobserved-directory
// frontier.
func TestRunnableDirectoriesAreValidatedAndDigested_AFPV0650(t *testing.T) {
	for name, runnable := range map[string][]string{
		"not unobserved": {"other"},
		"unsorted":       {"nested/deep", "nested"},
		"duplicate":      {"nested", "nested"},
	} {
		language := chain()
		language.unobserved = []string{"nested", "nested/deep"}
		language.runnable = runnable
		if _, err := Build(context.Background(), t.TempDir(), language); !errors.Is(err, ErrInvalidLanguage) {
			t.Errorf("%s: err = %v, want ErrInvalidLanguage", name, err)
		}
	}
	language := chain()
	language.unobserved = []string{"nested", "nested/deep"}
	plain, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	body, err := plain.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte(`"runnable"`)) {
		t.Fatalf("a graph without runnable directories carries the member: %s", body)
	}
	language.runnable = []string{"nested"}
	marked, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Digest() == plain.Digest() {
		t.Fatal("runnable directories are not part of the digest")
	}
	plan := Select(marked, []string{"nested/new.x", "nested/deep/new.x"})
	want := []Unknown{
		{Reason: UnknownLanguageFrontier, Detail: "x:unobserved-directory:nested/deep:nested/deep/new.x"},
		{Reason: UnknownLanguageFrontier, Detail: "x:unobserved-module:nested:nested/new.x"},
	}
	if !reflect.DeepEqual(plan.Unknown, want) || plan.Scope != ScopeUnknown {
		t.Fatalf("scope = %s unknown = %+v, want UNKNOWN with %+v", plan.Scope, plan.Unknown, want)
	}
}
