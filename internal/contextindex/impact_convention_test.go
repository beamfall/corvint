package contextindex

import (
	"context"
	"testing"
)

func TestImpactDistinguishesTestConventionFromProvenImporter(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/fixture\n\ngo 1.27.0\n",
		"pkg/foo.go":      "package pkg\n\nfunc Foo() {}\n",
		"pkg/foo_test.go": "package pkg\n\nfunc TestUnrelated() {}\n",
		"cmd/use/main.go": "package main\n\nimport \"example.test/fixture/pkg\"\n\nfunc main() { pkg.Foo() }\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"pkg/foo.go"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}

	authorities := make(map[string]string)
	for _, result := range mapsFromAny(receipt["results"]) {
		for _, item := range mapsFromAny(result["evidence"]) {
			authorities[stringValue(result["kind"])+":"+stringValue(result["id"])] = stringValue(item["authority"])
		}
	}
	if got := authorities["test:pkg/foo_test.go"]; got != "test-convention" {
		t.Fatalf("foo_test.go authority = %q, want test-convention", got)
	}
	if got := authorities["reverse-import:cmd/use/main.go"]; got != "syntax" {
		t.Fatalf("proven importer authority = %q, want syntax", got)
	}
}

func TestImpactReservesConventionTestsAheadOfBroadModuleRootImporters(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                   "module example.test/fixture\n\ngo 1.27.0\n",
		"foo.go":                   "package fixture\n\nfunc Foo() {}\n",
		"foo_test.go":              "package fixture\n\nfunc TestFoo() { Foo() }\n",
		"other_test.go":            "package fixture\n\nfunc TestOther() {}\n",
		"cmd/broad/main.go":        "package main\n\nimport fixture \"example.test/fixture\"\n\nfunc main() { fixture.Other() }\n",
		"cmd/file-named/main.go":   "package main\n\nimport fixture \"example.test/fixture\"\n\n// foo.go remains relevant here.\nfunc main() { fixture.Other() }\n",
		"cmd/symbol-named/main.go": "package main\n\nimport fixture \"example.test/fixture\"\n\nfunc main() { fixture.Foo() }\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"foo.go"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	got := resultKeys(receipt)
	want := []string{
		"path:foo.go",
		"test:foo_test.go",
		"reverse-import:cmd/file-named/main.go",
		"reverse-import:cmd/symbol-named/main.go",
		"test:other_test.go",
	}
	if !stringSlicesEqual(got, want) {
		t.Fatalf("impact order = %v, want %v", got, want)
	}
}
