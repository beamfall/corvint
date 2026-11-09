// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-3334: only proven testing.T metadata leaves the reader index. A
// name-derived path, escaped handle, custom receiver, and nested read remain.
func TestTestMetadataReaders_AFPV03334(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod": "module example.test/m\n",
		"diagnostic/d_test.go": `package diagnostic
import "testing"
func TestX(t *testing.T) { t.Fatal("script executed") }
`,
		"name/n_test.go": `package name
import "testing"
func TestX(t *testing.T) { t.Run("script",func(t *testing.T){t.Log("nothing");t.Helper();t.Parallel()}) }
`,
		"namedpath/n_test.go": `package namedpath
import ("testing";"os")
func TestX(t *testing.T) { t.Run("script",func(t *testing.T){os.ReadFile(t.Name()+"/data.tsv")}) }
`,
		"escaped/e_test.go": `package escaped
import "testing"
func helper(t *testing.T) {}
func TestX(t *testing.T) { t.Run("script",func(t *testing.T){helper(t)}) }
`,
		"custom/c_test.go": `package custom
import "testing"
type reader struct{}
func (reader) Fatal(string) {}
func TestX(t *testing.T) { { t := reader{}; t.Fatal("script") } }
`,
		"nested/n_test.go": `package nested
import ("testing";"os")
func TestX(t *testing.T) { t.Fatal(os.ReadFile("script/data.tsv")) }
`,
		"alias/a_test.go": `package alias
import tt "testing"
func TestX(t *tt.T) { t.Logf("script %d",1) }
`,
		"real/r_test.go": `package real
import ("testing";"os")
func TestX(t *testing.T) { t.Fatal("script"); os.ReadFile("script/data.tsv") }
`,
	})
	g, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	p := affected.Select(g, []string{"script/data.tsv"})
	for _, name := range []string{"diagnostic", "name", "alias"} {
		if slices.ContainsFunc(p.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/m/"+name }) {
			t.Errorf("metadata selected %s", name)
		}
	}
	for _, name := range []string{"namedpath", "escaped", "custom", "nested", "real"} {
		if !slices.ContainsFunc(p.Selected, func(s affected.Selection) bool { return s.UnitID == "go:example.test/m/"+name }) {
			t.Errorf("lost possible reader %s", name)
		}
	}
}
