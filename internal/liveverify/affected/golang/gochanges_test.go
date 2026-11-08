// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3320: parse both sides, compare scanner tokens including automatic
// semicolons, and protect every directive kind and cgo preamble.
func TestGoChangeRecords_AFP_V0_3320(t *testing.T) {
	const plain = "package p\n\nfunc F() int { return 1 }\n"
	graph, err := affected.BuildFS(fstest.MapFS{
		"go.mod":            {Data: []byte("module example.test/g\n")},
		"p.go":              {Data: []byte(plain)},
		"p_test.go":         {Data: []byte("package p\n")},
		"user/user_test.go": {Data: []byte("package user\nimport _ \"example.test/g\"\n")},
	}, New())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, before, after string
		commentOnly         bool
	}{
		{"comment", plain, plain + "// more context\n", true},
		{"whitespace", plain, "package  p\n\n\nfunc  F() int {  return 1 }\n", true},
		{"code", plain, strings.Replace(plain, "return 1", "return 2", 1), false},
		{"parse head", plain, "package p\nfunc {", false},
		{"parse base", "package p\nfunc {", plain, false},
		{"semicolon", "package p\nfunc F() { f() /* one line */ ; }\n", "package p\nfunc F() { f() /* two\nlines */ ; }\n", false},
		{"raw string", "package p\nvar X = `one two`\n", "package p\nvar X = `one  two`\n", false},
		{"go build", "//go:build linux\n\n" + plain, "//go:build darwin\n\n" + plain, false},
		{"legacy build", "// +build linux\n\n" + plain, "// +build darwin\n\n" + plain, false},
		{"tab build", "//\t+build linux\n\n" + plain, "//\t+build darwin\n\n" + plain, false},
		{"legacy build separation", "// +build linux\n\n" + plain, "// +build linux\n// ordinary\n" + plain, false},
		{"legacy build prefix", "// +build linux\n\n" + plain, "/* ordinary */\n// +build linux\n\n" + plain, false},
		{"embed", plain + "//go:embed one\nvar X string\n", plain + "//go:embed two\nvar X string\n", false},
		{"generate", plain + "//go:generate echo one\n", plain + "//go:generate echo two\n", false},
		{"linkname", plain + "//go:linkname F other.F\n", plain + "//go:linkname F other.G\n", false},
		{"line", plain + "//line one.go:1\n", plain + "//line two.go:1\n", false},
		{"block line", plain + "/*line one.go:1*/\n", plain + "/*line two.go:1*/\n", false},
		{"export", "package p\n//export F\nfunc F() {}\n", "package p\n//export G\nfunc F() {}\n", false},
		{"cgo", "package p\n/* int x = 1; */\nimport \"C\"\n", "package p\n/* int x = 2; */\nimport \"C\"\n", false},
		{"cgo grouped", "package p\nimport (\n// int x = 1;\n\"C\"\n)\n", "package p\nimport (\n// int x = 2;\n\"C\"\n)\n", false},
		{"cgo grouped declaration", "package p\n// int x = 1;\nimport (\n\"C\"\n)\n", "package p\n// int x = 2;\nimport (\n\"C\"\n)\n", false},
		{"cgo detached", "package p\n// int x;\nimport \"C\"\n", "package p\n// int x;\n\nimport \"C\"\n", false},
		{"directive removed", plain + "//go:generate echo one\n", plain, false},
		{"directive moved", "package p\n//go:noinline\nfunc F() int { return 1 }\n", plain + "//go:noinline\n", false},
		{"directive unchanged", "//go:build linux\n\n" + plain, "//go:build linux\n\n" + plain + "// end\n", true},
		{"cgo unchanged", "package p\n/* int x; */\nimport \"C\"\n", "package p\n/* int x; */\nimport \"C\"\n// end\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !strings.HasPrefix(test.name, "parse ") {
				for _, content := range []string{test.before, test.after} {
					if _, err := parser.ParseFile(token.NewFileSet(), "p.go", content, parser.ParseComments); err != nil {
						t.Fatalf("fixture must parse before classification: %v", err)
					}
				}
			}
			base := fstest.MapFS{"p.go": {Data: []byte(test.before)}}
			head := fstest.MapFS{"p.go": {Data: []byte(test.after)}}
			got := GoChangeRecords(base, head, []string{"p.go"})
			if (len(got) == 1) != test.commentOnly || len(got) == 1 && got[0].Kind != affected.GoCommentOnly {
				t.Fatalf("classification = %+v, commentOnly=%v", got, test.commentOnly)
			}
			plan := affected.Select(graph.WithGoChanges(got), []string{"p.go"})
			wantSelected := 2
			if test.commentOnly {
				wantSelected = 1
			}
			if len(plan.Selected) != wantSelected {
				t.Fatalf("classification did not preserve fail-closed propagation: %+v", plan.Selected)
			}
		})
	}
}

// AFP-V0-3320: missing endpoints, rename and read errors cannot narrow.
func TestGoChangesFailClosed_AFP_V0_3320(t *testing.T) {
	file := &fstest.MapFile{Data: []byte("package p\n")}
	base := fstest.MapFS{"p.go": file}
	if len(GoChangeRecords(base, base, []string{"p.go"})) != 1 {
		t.Fatal("fixture is not admitted before fault injection")
	}
	for _, test := range []struct {
		name       string
		base, head fs.FS
		dirty      []string
	}{
		{"unknown base", nil, base, []string{"p.go"}},
		{"unknown head", base, nil, []string{"p.go"}},
		{"addition", fstest.MapFS{}, base, []string{"p.go"}},
		{"deletion", base, fstest.MapFS{}, []string{"p.go"}},
		{"rename", base, fstest.MapFS{"renamed.go": file}, []string{"p.go", "renamed.go"}},
		{"read error", base, fstest.MapFS{"p.go": {Mode: fs.ModeDir}}, []string{"p.go"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := GoChangeRecords(test.base, test.head, test.dirty); len(got) != 0 {
				t.Fatalf("failure classified as safe: %+v", got)
			}
		})
	}
}

// AFP-V0-3322: a test file in a comment-inspecting package is not narrowed;
// syntax-only introspection and ordinary tests remain eligible.
func TestCommentInspectingTests_AFP_V0_3322(t *testing.T) {
	for _, test := range []struct {
		name, helper string
		want         bool
	}{
		{"ordinary", "package p\n", true},
		{"doc", "package p\nimport \"go/doc\"\n", false},
		{"ast comments", "package p\nimport (\"go/ast\"; p \"go/parser\")\nvar mode = p.ParseComments\n", false},
		{"dot comments", "package p\nimport (\"go/ast\"; . \"go/parser\")\nvar mode = ParseComments\n", false},
		{"ast syntax", "package p\nimport \"go/ast\"\n", true},
		{"broken sibling", "package p\nfunc {", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := fstest.MapFS{"p_test.go": {Data: []byte("package p\n")}, "helper.go": {Data: []byte(test.helper)}}
			head := fstest.MapFS{"p_test.go": {Data: []byte("package p\n// docs\n")}, "helper.go": base["helper.go"]}
			if got := GoChangeRecords(base, head, []string{"p_test.go"}); (len(got) == 1) != test.want {
				t.Fatalf("classification = %+v, want safe=%v", got, test.want)
			}
		})
	}
}
