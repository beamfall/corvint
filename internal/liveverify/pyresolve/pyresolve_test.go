package pyresolve

import (
	"strings"
	"testing"
	"time"
)

func TestImportsAtLine(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		imported string
		line     int
		want     bool
	}{
		{
			name:     "plain import",
			source:   "import os\n",
			imported: "os",
			line:     1,
			want:     true,
		},
		{
			name:     "dotted import",
			source:   "import a.b.c\n",
			imported: "a.b.c",
			line:     1,
			want:     true,
		},
		{
			name:     "aliased import",
			source:   "import a.b.c as x\n",
			imported: "a.b.c",
			line:     1,
			want:     true,
		},
		{
			name:     "comma list",
			source:   "import x, a.b.c\n",
			imported: "a.b.c",
			line:     1,
			want:     true,
		},
		{
			name:     "comma list wrong entry",
			source:   "import x, a.b.c\n",
			imported: "x.y",
			line:     1,
			want:     false,
		},
		{
			name:     "from-import single member",
			source:   "from a.b import c\n",
			imported: "a.b.c",
			line:     1,
			want:     true,
		},
		{
			name:     "from-import package form",
			source:   "from a.b import c\n",
			imported: "a.b",
			line:     1,
			want:     true,
		},
		{
			name: "from-import parenthesized multi-line cites the from line",
			source: "from a.b import (\n" +
				"    c,\n" +
				"    d,\n" +
				")\n",
			imported: "a.b.d",
			line:     1,
			want:     true,
		},
		{
			name:     "relative import does not match absolute spelling",
			source:   "from . import c\n",
			imported: "c",
			line:     1,
			want:     false,
		},
		{
			name:     "relative import matches its own literal spelling",
			source:   "from .a import b\n",
			imported: ".a.b",
			line:     1,
			want:     true,
		},
		{
			name:     "name inside a comment on the cited line",
			source:   "import os  # a.b.c\n",
			imported: "a.b.c",
			line:     1,
			want:     false,
		},
		{
			name: "name inside a triple-quoted string spanning the cited line",
			source: "x = \"\"\"\n" +
				"import a.b.c\n" +
				"\"\"\"\n",
			imported: "a.b.c",
			line:     2,
			want:     false,
		},
		{
			name:     "longer dotted name is not a match",
			source:   "import a.b.cd\n",
			imported: "a.b.c",
			line:     1,
			want:     false,
		},
		{
			name:     "wrong line",
			source:   "x = 1\nimport a.b.c\n",
			imported: "a.b.c",
			line:     1,
			want:     false,
		},
		{
			name:     "semicolon-separated statements",
			source:   "import os; import a.b\n",
			imported: "a.b",
			line:     1,
			want:     true,
		},
		{
			name:     "semicolon-separated statements first entry",
			source:   "import os; import a.b\n",
			imported: "os",
			line:     1,
			want:     true,
		},
		{
			name:     "backslash continuation",
			source:   "from a.b import \\\n    c\n",
			imported: "a.b.c",
			line:     1,
			want:     true,
		},
		{
			name:     "unterminated string returns false",
			source:   "x = \"abc\nimport os\n",
			imported: "os",
			line:     2,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImportsAtLine([]byte(tt.source), tt.imported, tt.line)
			if got != tt.want {
				t.Errorf("ImportsAtLine(%q, %q, %d) = %v, want %v", tt.source, tt.imported, tt.line, got, tt.want)
			}
		})
	}
}

func TestImportsEmptySource(t *testing.T) {
	imports, err := Imports([]byte(""))
	if err != nil {
		t.Fatalf("Imports(empty) returned error: %v", err)
	}
	if len(imports) != 0 {
		t.Fatalf("Imports(empty) = %v, want none", imports)
	}
}

func TestImportsUnterminatedStringErrors(t *testing.T) {
	_, err := Imports([]byte("x = \"abc\n"))
	if err == nil {
		t.Fatalf("Imports(unterminated string) returned no error")
	}
}

func TestImportsDeterministic(t *testing.T) {
	source := []byte("import os\nfrom a.b import c, d\nfrom . import e\n")
	first, err := Imports(source)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}
	second, err := Imports(source)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("non-deterministic import counts: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Line != second[i].Line || strings.Join(first[i].Names, ",") != strings.Join(second[i].Names, ",") {
			t.Fatalf("non-deterministic import at %d: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestImportsShapes(t *testing.T) {
	source := []byte("import os\nfrom a.b import c, d\nfrom . import e\nfrom .x import y\nfrom pkg import *\n")
	imports, err := Imports(source)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}
	want := []Import{
		{Line: 1, Names: []string{"os"}},
		{Line: 2, Names: []string{"a.b.c", "a.b.d", "a.b"}},
		{Line: 3, Names: []string{".e", "."}},
		{Line: 4, Names: []string{".x.y", ".x"}},
		{Line: 5, Names: []string{"pkg"}},
	}
	if len(imports) != len(want) {
		t.Fatalf("Imports() = %+v, want %+v", imports, want)
	}
	for i := range want {
		if imports[i].Line != want[i].Line || strings.Join(imports[i].Names, ",") != strings.Join(want[i].Names, ",") {
			t.Fatalf("Imports()[%d] = %+v, want %+v", i, imports[i], want[i])
		}
	}
}

func TestImportsLargeSourcePerformance(t *testing.T) {
	var builder strings.Builder
	line := "from pkg.sub import member\n"
	for builder.Len() < 1<<20 {
		builder.WriteString(line)
	}
	source := []byte(builder.String())

	start := time.Now()
	imports, err := Imports(source)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Imports(1MiB): %v", err)
	}
	if len(imports) == 0 {
		t.Fatalf("Imports(1MiB) found no imports")
	}
	// A complexity guard, not a latency budget. What it exists to catch is a
	// superlinear regression in the statement walk, which on a 1 MiB input costs
	// seconds and not milliseconds, so the ceiling belongs where that regression is
	// unmissable and a loaded host is not. Measured on this host, 1 MiB of import
	// lines cost 13.3 ms median and 32.0 ms max at load 13 without the detector,
	// and 212.6 ms median at load 13 under it, rising to an observed 507.3 ms
	// failure at load 33. Gate sweeps here reach load 68, where the race arm
	// extrapolates to roughly 1 s, so the previous 100 ms and its 5x race
	// multiplier measured host load rather than the walk's complexity.
	maxElapsed := time.Second
	if raceEnabled {
		maxElapsed = 3 * time.Second
	}
	if elapsed > maxElapsed {
		t.Fatalf("Imports(1MiB) took %s, want <%s", elapsed, maxElapsed)
	}
}

func TestResolveRelative(t *testing.T) {
	tests := []struct {
		name             string
		importingPackage string
		dots             int
		module           string
		want             string
	}{
		{"current package, no module", "a.b", 1, "", "a.b"},
		{"current package, with module", "a.b", 1, "c", "a.b.c"},
		{"parent package", "a.b", 2, "c", "a.c"},
		{"climb past root returns empty", "a", 3, "c", ""},
		{"zero dots is not relative", "a.b", 0, "c", ""},
		{"root package, no module", "", 1, "", ""},
		{"root package, with module", "", 1, "x", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRelative(tt.importingPackage, tt.dots, tt.module)
			if got != tt.want {
				t.Errorf("ResolveRelative(%q, %d, %q) = %q, want %q", tt.importingPackage, tt.dots, tt.module, got, tt.want)
			}
		})
	}
}
