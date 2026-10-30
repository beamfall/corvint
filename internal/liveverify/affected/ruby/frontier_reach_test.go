// SPDX-License-Identifier: Apache-2.0
package ruby

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-511: a dynamic load binds a Ruby feature, so a dirty path no plugin owns drops it; a
// test with no visible edge may depend on any change, so the same path keeps it; a dirty
// Rakefile, which the plugin reads for load roots, keeps every entry.
func TestFrontierReachHoldsForUnownedPaths_AFPV0511(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "lib/exporter.rb", "class Exporter\n  def call\n    require \"plugins/#{name}\"\n  end\nend\n")
	write(t, root, "spec/exporter_spec.rb", "require \"exporter\"\n\nRSpec.describe Exporter do\n  it \"calls\" do\n    expect(Exporter.new.call).to be_nil\n  end\nend\n")
	write(t, root, "spec/orphan_spec.rb", "RSpec.describe \"orphan\" do\n  it \"runs\" do\n    expect(1).to eq(1)\n  end\nend\n")
	before, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	dynamic := FrontierDynamicLoad + ":lib/exporter.rb:3"
	implicit := FrontierImplicitTestDependency + ":spec/orphan_spec.rb"
	if fmt.Sprint(before.Frontier) != fmt.Sprint([]string{dynamic, implicit}) {
		t.Fatalf("plugin frontier = %v", before.Frontier)
	}
	write(t, root, "Makefile", "all:\n\tbundle exec rspec\n")
	write(t, root, "pkg/testdata/cases.json", `[{"name": "a"}]`)
	after, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unowned paths changed the observation:\n%+v\n%+v", before, after)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  []string
	}{
		{"Makefile", []string{implicit}},
		{"pkg/testdata/cases.json", []string{implicit}},
		{"Rakefile", []string{dynamic, implicit}},
		{"lib/exporter.rb", []string{dynamic, implicit}},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		got := []string{}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownLanguageFrontier {
				got = append(got, unknown.Detail)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("dirty %s: frontier %v, want %v", tc.dirty, got, tc.want)
		}
	}
}
