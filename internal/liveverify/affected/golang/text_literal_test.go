// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2952 (V1-4283): a climbing token a non-test file holds only inside
// line-broken literals is program or document text, not a path it opens.
func TestLineBrokenClimbIsText_AFPV02952(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":           "module example.test/m\n",
		"prose/p.go":       "package prose\nvar Doc = \"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\"\n",
		"template/t.go":    "package template\nimport \"fmt\"\nfunc Ruby(model string) []byte { return fmt.Appendf(nil, \"require_relative \\\"../models/%s\\\"\\n\\nclass X\\nend\\n\", model) }\n",
		"path/p.go":        "package path\nvar Manifest = \"../manifest.json\"\n",
		"script/s.go":      "package script\nvar Script = \"set -e\\ncd ../fixture\\ncat ../manifest.json\\n\"\n",
		"both/b.go":        "package both\nvar Doc = \"See ../manifest.json\\n\"\nvar Path = \"../manifest.json\"\n",
		"tested/t.go":      "package tested\n",
		"tested/t_test.go": "package tested\nvar Doc = \"See ../../manifest.json\\n\"\n",
		"raw/r.go":         "package raw\nvar Doc = `| manifest.json | See ../manifest.json for the digest. |\n\nMore text.\n`\n",
	})
	for _, name := range []string{"prose", "template", "path", "script", "both", "raw"} {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package " + name + "\n"})
	}
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	for _, name := range []string{"prose", "template", "raw"} {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads != "" || unit.LocatesRoot {
			t.Errorf("%s: a line-broken text literal made the package an unbounded reader: %+v", name, unit)
		}
	}
	for _, name := range []string{"path", "script", "both", "tested"} {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads == "" {
			t.Errorf("%s: a climbing path literal lost rule (d): %+v", name, unit)
		}
	}
	// The exempt token stays a rule (c) path token.
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"manifest.json": "{}\n"})
	found := map[string]string{}
	for _, item := range affected.Select(graph, []string{"manifest.json"}).Selected {
		found[item.UnitID] = item.Witness.Kind
	}
	if found["go:example.test/m/prose"] != affected.WitnessPathLiteralReader {
		t.Errorf("prose: the text token no longer names its file: %v", found)
	}
}
