// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"runtime"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AZU-V1-960: a URL-style source match must not erase the literal CommonJS read.
func TestCommonJSLiteralPaths_AZU_V1_960(t *testing.T) {
	for _, suffix := range []string{"#one.json", "?one.json"} {
		for _, esm := range []bool{false, true} {
			name := "require" + suffix
			if esm {
				name = "import" + suffix
			}
			t.Run(name, func(t *testing.T) {
				if runtime.GOOS == "windows" && suffix[0] == '?' {
					t.Skip("Windows filenames cannot contain question marks")
				}
				root := t.TempDir()
				literal := "settings" + suffix
				reader := "read.test.cjs"
				body := "const {test} = require('node:test');\n" +
					"const data = require('./" + literal + "');\n"
				if esm {
					reader = "read.test.mjs"
					body = "import {test} from 'node:test';\n" +
						"import data from './" + literal + "';\n"
				}
				write(t, root, reader, body+"test('read', () => data.value);\n")
				write(t, root, "settings.cjs", "module.exports = {value: 2};\n")
				write(t, root, literal, `{"value":1}`)
				result, err := New().Units(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				unit := unitByID(result, unitID(runnerNode, reader))
				if !slices.Contains(unit.Imports, unitID("", "settings.cjs")) {
					t.Fatalf("fixture must resolve its source alternative: %+v", unit)
				}
				if unit.PathTokensBounded {
					t.Fatalf("ordinary literal path must retain a proven read set: %+v", unit)
				}
				graph, err := affected.Build(context.Background(), root, New())
				if err != nil {
					t.Fatal(err)
				}
				plan := affected.Select(graph, []string{literal})
				if got := slices.Contains(plan.SelectedTests(), reader); got != !esm {
					t.Fatalf("literal reader selected=%v, want %v: %+v", got, !esm, plan)
				}
				plan = affected.Select(graph, []string{"settings.cjs"})
				if !slices.Contains(plan.SelectedTests(), reader) {
					t.Fatalf("source interpretation lost its reader: %+v", plan)
				}
			})
		}
	}
}

// AZU-V1-960: a source alternative cannot certify unsupported literal resolution.
func TestCommonJSUnknownLiteralPath_AZU_V1_960(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.cjs", "module.exports = {};\n")
	write(t, root, "read.test.cjs", "const {test} = require('node:test');\n"+
		"const data = require('./settings#%future');\ntest('read', () => data.value);\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"unrelated.json"})
	want := "typescript:path-token-bound:" + unitID(runnerNode, "read.test.cjs")
	if !slices.Contains(frontierDetails(plan), want) {
		t.Fatalf("unsupported literal form lost its frontier: %+v", plan)
	}
}
