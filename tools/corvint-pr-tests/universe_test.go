package main

import (
	"strings"
	"testing"
)

// AFP-V0-9012: the narrow walk names exactly the package directories ./...
// skips, and nothing Go would itself exclude.
func TestHiddenPackageDirs_AFPV09012(t *testing.T) {
	got := hiddenPackageDirs([]string{
		"go.mod", "a/a.go", "root.go",
		".github/cishards/shards.go", ".github/cishards/shards_test.go",
		".github/cishards/cmd/main.go",
		"_tools/gen.go", "a/.helpers/h_test.go",
		".hidden/_ignored.go", ".hidden/.ignored.go",
		".github/testdata/fixture.go", ".github/vendor/v/v.go",
		".nested/go.mod", ".nested/inner/x.go", "x/go.mod", "x/.y/z.go",
		"../outside/.h/o.go", ".hidden/README.md",
	})
	want := []string{"./.github/cishards", "./.github/cishards/cmd", "./_tools", "./a/.helpers"}
	if !equal(got, want) {
		t.Fatalf("hidden packages = %v, want %v", got, want)
	}
	for args, ok := range map[string]bool{
		"./...":                                 true,
		"./... ./.github/cishards ./a/.helpers": true,
		"./... ./a/.helpers ./.github/cishards": false,
		"./... ./a/.helpers ./a/.helpers":       false,
		"./... ./a":                             false,
		"./... ./.x/testdata":                   false,
		"./a/.helpers":                          false,
		"":                                      false,
	} {
		if canonicalFullArgs(strings.Fields(args)) != ok {
			t.Fatalf("canonicalFullArgs(%q) != %v", args, ok)
		}
	}
	if err := fullReceiptArgs([]string{"./..."}, nil); err == nil {
		t.Fatal("receipt admitted without the target tree")
	}
}
