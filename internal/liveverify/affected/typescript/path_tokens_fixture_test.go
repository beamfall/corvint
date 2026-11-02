package typescript

import (
	"slices"
	"testing"
)

// TestFixtureMarkedLiteralsNameNoRead_TJAA830: a token only literals on a
// //corvint:fixture-path line hold is dropped; an unmarked twin keeps it, and
// the marker text inside a literal or followed by code marks nothing (V1-4258).
func TestFixtureMarkedLiteralsNameNoRead_TJAA830(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want []string
	}{
		{"marked", "writeFileSync(join(repo,'README.md'),'x') //corvint:fixture-path\nread('docs/a.md')\n", []string{"docs/a.md"}},
		{"marked end line", "const p = `\nfixture/README.md` //corvint:fixture-path\n", nil},
		{"unmarked twin", "write('README.md') //corvint:fixture-path\nread('README.md')\n", []string{"README.md"}},
		{"source module", "const asset = 'README.md' //corvint:fixture-path\n", nil},
		{"marker in a literal", "write('README.md', '//corvint:fixture-path')\n", []string{"//corvint", "README.md"}},
		{"marker with trailing code", "write('README.md') //corvint:fixture-path please\n", []string{"README.md"}},
		{"no marker", "write('README.md')\n", []string{"README.md"}},
	} {
		got, bounded := pathTokens(test.body)
		if bounded || !slices.Equal(got, test.want) {
			t.Errorf("%s: tokens %q bounded %v, want %q", test.name, got, bounded, test.want)
		}
	}
}
