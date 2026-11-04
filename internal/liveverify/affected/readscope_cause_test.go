// SPDX-License-Identifier: Apache-2.0
package affected

import "testing"

// TSP-V0-1062: the explanation pairs each entry with the declaration that holds it; a changed Go
// declaration is named as itself even for a shell unit it selects.
func TestReadScopeCausePairsDeclarationAndEntry_TSPV01062(t *testing.T) {
	for _, c := range []struct {
		id, directory, dirty string
		scope                []string
		want                 ReadScopeCause
	}{
		{"go:m/reader", "reader", "a/new_test.go", []string{"a/"}, ReadScopeCause{ReadScopesPath, "a/"}},
		{"go:m/reader", "reader", "reader/x_test.go", []string{"a/"}, ReadScopeCause{ReadScopesPath, "reader/"}},
		{"shell:script/x_test.sh", "script", "internal/a.go", []string{"internal/"}, ReadScopeCause{ShellReadScopesPath, "internal/"}},
		{"shell:script/x_test.sh", "script", ReadScopesPath, []string{"internal/"}, ReadScopeCause{ReadScopesPath, ReadScopesPath}},
	} {
		if got := readScopeCause(c.id, c.directory, c.scope, c.dirty); *got != c.want {
			t.Errorf("readScopeCause(%s, %s) = %+v, want %+v", c.id, c.dirty, *got, c.want)
		}
	}
}
