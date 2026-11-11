// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"context"
	"reflect"
	"testing"
)

// refreshTwice builds the inventory of root twice on an unchanged tree and
// requires identical bytes (task 4 proof), returning the decoded first.
func refreshTwice(t *testing.T, git Git, exec Exec) Inventory {
	t.Helper()
	first, err := Build(context.Background(), Options{Git: git, Exec: exec})
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := Encode(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), Options{Git: git, Exec: exec})
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := Encode(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("refresh twice wrote different bytes:\n%s\nvs\n%s", firstBytes, secondBytes)
	}
	decoded, err := Decode(firstBytes)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func instanceByKey(t *testing.T, inventory Inventory, key string) Instance {
	t.Helper()
	for _, instance := range inventory.Instances {
		if instance.Key == key {
			return instance
		}
	}
	t.Fatalf("no instance %s in %+v (failures %+v)", key, inventory.Instances, inventory.Failures)
	return Instance{}
}

func testsOf(instance Instance) []Test {
	out := make([]Test, len(instance.Tests))
	for index, test := range instance.Tests {
		test.Extra = nil
		out[index] = test
	}
	return out
}

// requireBinding checks TIV-V1-005 on one record: the commit, config and
// lockfile digests and a blob per listed file, all equal to Git's.
func requireBinding(t *testing.T, root string, instance Instance, wantFiles ...string) {
	t.Helper()
	if instance.Commit != runGit(t, root, "rev-parse", "HEAD") {
		t.Errorf("%s commit %s", instance.Key, instance.Commit)
	}
	if instance.ConfigDigest != runGit(t, root, "rev-parse", "HEAD:"+instance.Config) {
		t.Errorf("%s config digest %s", instance.Key, instance.ConfigDigest)
	}
	if instance.Lockfile != "" && instance.LockfileDigest != runGit(t, root, "rev-parse", "HEAD:"+instance.Lockfile) {
		t.Errorf("%s lockfile digest %s", instance.Key, instance.LockfileDigest)
	}
	var got []string
	for _, file := range instance.Files {
		got = append(got, file.Path)
		if file.Blob != runGit(t, root, "rev-parse", "HEAD:"+file.Path) {
			t.Errorf("%s file %s blob %s", instance.Key, file.Path, file.Blob)
		}
	}
	if !reflect.DeepEqual(got, wantFiles) {
		t.Errorf("%s files %v, want %v", instance.Key, got, wantFiles)
	}
	identity, _ := CurrentLister(instance.Runner)
	if instance.Lister.Name != identity.Name || instance.Lister.Version != identity.Version || instance.Lister.Kind != identity.Kind {
		t.Errorf("%s lister %+v, want %+v", instance.Key, instance.Lister, identity)
	}
}

// TIV-V1-004, TIV-V1-005: the static Go lister names every top-level Test,
// Fuzz and runnable Example function per package with go/ast, keeps nested
// modules apart, marks a name declared twice in one package duplicate, and
// names an unparseable file by its package with test-id-unmapped.
func TestTIVV1004GoStaticLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		"go.mod":       "module example.com/m\n",
		"go.sum":       "",
		"calc/calc.go": "package calc\n",
		"calc/calc_test.go": `package calc

import tst "testing"

func TestAdd(t *tst.T)         {}
func TestAdd_negative(t *tst.T) {}
func Testlower(t *tst.T)       {}
func TestHelper(n int)         {}
func BenchmarkAdd(b *tst.B)    {}
func FuzzParse(f *tst.F)       {}

func ExampleAdd() {
	// Output: 1
}

func ExampleQuiet() {}
`,
		"calc/ext_test.go":        "package calc_test\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n",
		"calc/broken_test.go":     "package calc\n\nfunc (\n",
		"calc/testdata/x_test.go": "package x\n\nimport \"testing\"\n\nfunc TestFixture(t *testing.T) {}\n",
		"sub/go.mod":              "module example.com/sub\n",
		"sub/s_test.go":           "package sub\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {}\n",
	})
	inventory := refreshTwice(t, git, nil)
	main := instanceByKey(t, inventory, "go@.")
	want := []Test{
		{ID: "go:calc", File: "calc/broken_test.go", Reason: "test-id-unmapped"},
		{ID: "go:calc::ExampleAdd", File: "calc/calc_test.go"},
		{ID: "go:calc::FuzzParse", File: "calc/calc_test.go"},
		{ID: "go:calc::TestAdd", File: "calc/calc_test.go", Duplicate: true},
		{ID: "go:calc::TestAdd::#2", File: "calc/ext_test.go", Duplicate: true},
		{ID: "go:calc::TestAdd_negative", File: "calc/calc_test.go"},
	}
	if got := testsOf(main); !reflect.DeepEqual(got, want) {
		t.Fatalf("go@. tests\n%+v\nwant\n%+v", got, want)
	}
	requireBinding(t, root, main, "calc/broken_test.go", "calc/calc_test.go", "calc/ext_test.go")
	if main.Lockfile != "go.sum" || main.Lister.Command != nil {
		t.Fatalf("go@. lockfile %q command %v", main.Lockfile, main.Lister.Command)
	}
	sub := instanceByKey(t, inventory, "go@sub")
	if got := testsOf(sub); !reflect.DeepEqual(got, []Test{{ID: "go:sub::TestSub", File: "sub/s_test.go"}}) {
		t.Fatalf("go@sub tests %+v", got)
	}
}

// TIV-V1-004: the static RSpec lister names each spec file at file level
// (RBR-V1-001).
func TestTIVV1004RSpecStaticLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".rspec":                    "--require spec_helper\n",
		"Gemfile.lock":              "GEM\n",
		"spec/spec_helper.rb":       "",
		"spec/user_spec.rb":         "RSpec.describe 'user' do\n  it('works') {}\nend\n",
		"spec/models/order_spec.rb": "RSpec.describe 'order' do\nend\n",
		"lib/user.rb":               "class User; end\n",
	})
	inventory := refreshTwice(t, git, nil)
	rspec := instanceByKey(t, inventory, "rspec@.")
	want := []Test{
		{ID: "rspec:spec/models/order_spec.rb", File: "spec/models/order_spec.rb", Reason: "test-id-unmapped"},
		{ID: "rspec:spec/user_spec.rb", File: "spec/user_spec.rb", Reason: "test-id-unmapped"},
	}
	if got := testsOf(rspec); !reflect.DeepEqual(got, want) {
		t.Fatalf("rspec tests %+v", got)
	}
	requireBinding(t, root, rspec, "spec/models/order_spec.rb", "spec/user_spec.rb")
}

// TIV-V1-004: the static Minitest lister names `def test_` methods of a
// class deriving from Minitest::Test, and a file it cannot read with
// certainty at file level.
func TestTIVV1004MinitestStaticLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		"Gemfile.lock":        "GEM\n",
		"test/test_helper.rb": "require 'minitest/autorun'\n",
		"test/calc_test.rb": `require_relative 'test_helper'

class CalcTest < Minitest::Test
  def setup; end

  def test_add
    assert true
  end

  def test_zero?
  end
end
`,
		"test/dsl_test.rb":    "class DslTest < ActiveSupport::TestCase\n  test \"works\" do\n  end\nend\n",
		"test/shared_test.rb": "class SharedTest < Minitest::Test\n  include Shared\n  def test_own; end\nend\n",
	})
	inventory := refreshTwice(t, git, nil)
	minitest := instanceByKey(t, inventory, "minitest@.")
	want := []Test{
		{ID: "minitest:test/calc_test.rb::CalcTest::test_add", File: "test/calc_test.rb"},
		{ID: "minitest:test/calc_test.rb::CalcTest::test_zero?", File: "test/calc_test.rb"},
		{ID: "minitest:test/dsl_test.rb", File: "test/dsl_test.rb", Reason: "test-id-unmapped"},
		{ID: "minitest:test/shared_test.rb", File: "test/shared_test.rb", Reason: "test-id-unmapped"},
	}
	if got := testsOf(minitest); !reflect.DeepEqual(got, want) {
		t.Fatalf("minitest tests\n%+v\nwant\n%+v", got, want)
	}
	requireBinding(t, root, minitest, "test/calc_test.rb", "test/dsl_test.rb", "test/shared_test.rb")
}
