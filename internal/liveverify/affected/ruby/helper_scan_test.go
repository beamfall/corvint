package ruby

import (
	"context"
	"io/fs"
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// countingTree counts every open of each path.
type countingTree struct {
	immutableTree
	mu    *sync.Mutex
	opens map[string]int
}

func (tree countingTree) Open(name string) (fs.File, error) {
	tree.mu.Lock()
	tree.opens[name]++
	tree.mu.Unlock()
	return tree.immutableTree.Open(name)
}

// AFP-V0-394: every spec loads the same helpers, and one observation reads
// each file exactly once, a helper included, however many specs load it,
// where it used to read a helper once as a source and again per loading spec.
// The disk observation is the immutable one.
func TestAFPV0394HelpersAreScannedOncePerObservation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Gemfile", "gem 'rspec'\n")
	write(t, root, "spec/spec_helper.rb", "require 'support/matchers'\nRSpec.configure do |config|\nend\n")
	write(t, root, "spec/support/matchers.rb", "module Matchers\nend\n")
	write(t, root, "spec/rails_helper.rb", "require 'spec_helper'\n")
	specs := 40
	for index := range specs {
		name := "widget" + strconv.Itoa(index)
		write(t, root, "lib/"+name+".rb", "class Widget"+strconv.Itoa(index)+"\nend\n")
		write(t, root, "spec/"+name+"_spec.rb", "require 'rails_helper'\nrequire '"+name+"'\n\nRSpec.describe Widget"+strconv.Itoa(index)+" do\n  it 'works' do\n  end\nend\n")
	}
	tree := countingTree{immutableTree: newImmutableTree(t, root), mu: &sync.Mutex{}, opens: map[string]int{}}
	serial, err := New().UnitsSource(affected.FSSource(tree))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spec/spec_helper.rb", "spec/rails_helper.rb", "spec/support/matchers.rb", "lib/widget7.rb", "spec/widget7_spec.rb"} {
		if opens := tree.opens[name]; opens != 1 {
			t.Fatalf("%s opened %d times for %d specs; want exactly 1", name, opens, specs)
		}
	}
	disk, err := New().UnitsSource(affected.DiskSourceContext(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(serial, disk) {
		t.Fatalf("disk observation differs from the immutable one:\n%#v\n%#v", disk, serial)
	}
	if kinds := frameworksOf(serial); kinds["ruby:rspec:spec/widget7_spec"] != true {
		t.Fatalf("specs must stay RSpec units through the memoized helper scan: %v", kinds)
	}
}

func frameworksOf(result affected.Result) map[string]bool {
	ids := map[string]bool{}
	for _, unit := range result.Units {
		ids[unit.ID] = true
	}
	return ids
}
