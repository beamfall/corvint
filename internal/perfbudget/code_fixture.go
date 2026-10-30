package perfbudget

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// The code repository's fixed reads (PRB-V0-058): the Go and TypeScript paths the affected
// scenarios plan over and the task the context query and lifecycle hook answer.
const (
	codeGoPath      = "pkg/p0002/p0002.go"
	codeTSPath      = "web/src/m00002.ts"
	codeContextTask = "change Word0002 in pkg/p0002 so the invoice ledger word is reported"
)

// generateCode writes the Go and TypeScript code repository (PRB-V0-058). Go package pN imports
// pN/2 and TypeScript module mN imports mN/2, so each import graph is a binary tree whose second
// node reaches about half the tree. The root package and module also import the adversarial
// sources: one Go and one TypeScript declaration, each a single line of s.LongLineBytes bytes.
func generateCode(ctx context.Context, root string, s Scale) error {
	files := map[string]string{
		"go.mod":        "module example.com/codeapp\n\ngo 1.22\n",
		"package.json":  "{\n  \"name\": \"codeapp-web\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"scripts\": {\"test\": \"vitest run\"},\n  \"devDependencies\": {\"typescript\": \"5.6.3\", \"vitest\": \"2.1.8\"}\n}\n",
		"tsconfig.json": "{\n  \"compilerOptions\": {\"target\": \"ES2022\", \"module\": \"ESNext\", \"moduleResolution\": \"Bundler\", \"strict\": true},\n  \"include\": [\"web/src\"]\n}\n",
		"pkg/table/table.go": "// Package table holds the adversarial single-line declarations.\npackage table\n\n" +
			"var Table = []string{" + longList(s.LongLineBytes, func(i int) string { return fmt.Sprintf("%q", word(i)+fmt.Sprint(i)) }) + "}\n\n" +
			"func Pick(" + longList(s.LongLineBytes/4, func(i int) string { return fmt.Sprintf("a%d int", i) }) + ") int { return a0 }\n",
		"web/src/table.ts": "export const TABLE: string[] = [" + longList(s.LongLineBytes, func(i int) string { return fmt.Sprintf("%q", word(i)+fmt.Sprint(i)) }) + "];\n",
	}
	for n := 1; n <= s.GoPackages; n++ {
		dir := fmt.Sprintf("pkg/p%04d/", n)
		parent, call := "example.com/codeapp/pkg/table", "table.Table[0]"
		if n > 1 {
			parent, call = fmt.Sprintf("example.com/codeapp/pkg/p%04d", n/2), fmt.Sprintf("p%04d.Word%04d()", n/2, n/2)
		}
		files[dir+fmt.Sprintf("p%04d.go", n)] = fmt.Sprintf("// Package p%04d is generated perf-budget fixture code.\npackage p%04d\n\nimport %q\n\n// Word%04d reports the %s word.\nfunc Word%04d() string { return %s + %q }\n\n// Record%04d is one %s record.\ntype Record%04d struct {\n\tName  string\n\tCount int\n}\n\n// Describe names the record.\nfunc (r Record%04d) Describe() string { return r.Name + Word%04d() }\n",
			n, n, parent, n, word(n), n, call, word(n), n, word(n/3), n, n, n)
		files[dir+fmt.Sprintf("p%04d_test.go", n)] = fmt.Sprintf("package p%04d\n\nimport \"testing\"\n\nfunc TestWord%04d(t *testing.T) {\n\tif Word%04d() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n", n, n, n)
	}
	for n := 1; n <= s.TSModules; n++ {
		parent, call := "./table", "TABLE[0]"
		name := "TABLE"
		if n > 1 {
			parent, call, name = fmt.Sprintf("./m%05d", n/2), fmt.Sprintf("word%05d()", n/2), fmt.Sprintf("word%05d", n/2)
		}
		files[fmt.Sprintf("web/src/m%05d.ts", n)] = fmt.Sprintf("import { %s } from %q;\n\n// word%05d reports the %s word.\nexport function word%05d(): string {\n  return %s + %q;\n}\n\nexport interface Shape%05d {\n  name: string;\n  count: number;\n}\n", name, parent, n, word(n), n, call, word(n), n)
		if n%4 == 2 {
			files[fmt.Sprintf("web/src/m%05d.test.ts", n)] = fmt.Sprintf("import { describe, expect, it } from \"vitest\";\nimport { word%05d } from \"./m%05d\";\n\ndescribe(\"word%05d\", () => {\n  it(\"answers\", () => {\n    expect(word%05d()).not.toBe(\"\");\n  });\n});\n", n, n, n, n)
		}
	}
	for path, content := range files {
		if err := writeFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content)); err != nil {
			return err
		}
	}
	return commitAll(ctx, root, "code")
}

// longList joins item(0), item(1), ... with ", " until the list is at least size bytes long.
func longList(size int, item func(int) string) string {
	var b strings.Builder
	for i := 0; b.Len() < size; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(item(i))
	}
	return b.String()
}
