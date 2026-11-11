// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"reflect"
	"strings"
	"testing"
)

// ruleTree is a repository with one RSpec and one Vitest instance.
func ruleTree() MapTree {
	return MapTree{
		".rspec":                     "--require spec_helper\n",
		"Gemfile.lock":               "GEM\n",
		"spec/user_spec.rb":          "describe 'user' do\n  it 'works' do\n  end\nend\n",
		"spec/order_spec.rb":         "describe 'order' do\nend\n",
		"spec/spec_helper.rb":        "",
		"web/vitest.config.ts":       "export default {}\n",
		"web/package-lock.json":      "{}\n",
		"web/src/sum.test.ts":        "test('adds', () => {})\n",
		"web/src/sum.ts":             "export const sum = 1\n",
		"lib/user.rb":                "class User; end\n",
		"web/node_modules/x.test.ts": "test('vendored', () => {})\n",
	}
}

// recordOf is the inventory record a refresh of tree would write for
// instance, with tests naming each candidate file at file level.
func recordOf(t *testing.T, tree MapTree, instance Declared) Instance {
	t.Helper()
	files, _ := tree.Files()
	identity, _ := CurrentLister(instance.Runner)
	record := Instance{
		Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config,
		ConfigDigest: files[instance.Config], Lockfile: instance.Lockfile, Lister: Lister{Name: identity.Name, Version: identity.Version, Kind: identity.Kind},
		Commit: fixtureCommit, Files: []File{}, Tests: []Test{},
	}
	if instance.Lockfile != "" {
		record.LockfileDigest = files[instance.Lockfile]
	}
	if identity.Kind == KindExecuting {
		record.Lister.Command = []string{"runner", "list"}
	}
	declared := Detect(files, tree.Read)
	for _, name := range Candidates(files, instance, declared) {
		record.Files = append(record.Files, File{Path: name, Blob: files[name]})
		record.Tests = append(record.Tests, Test{ID: instance.Runner + ":" + ownerOf(instance.Runner, name), File: name, Reason: "test-id-unmapped"})
	}
	return record
}

func encodedInventory(t *testing.T, tree MapTree, edit func(*Inventory)) string {
	t.Helper()
	files, _ := tree.Files()
	inventory := Inventory{Profile: Profile, Instances: []Instance{}, Failures: []Failure{}}
	for _, instance := range Detect(files, tree.Read) {
		inventory.Instances = append(inventory.Instances, recordOf(t, tree, instance))
	}
	if edit != nil {
		edit(&inventory)
	}
	data, err := Encode(inventory)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTIVV1003DetectInstances(t *testing.T) {
	files, _ := ruleTree().Files()
	got := Detect(files, ruleTree().Read)
	want := []Declared{
		{Key: "rspec@.", Runner: "rspec", Root: ".", Config: ".rspec", Lockfile: "Gemfile.lock"},
		{Key: "vitest@web", Runner: "vitest", Root: "web", Config: "web/vitest.config.ts", Lockfile: "web/package-lock.json"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Detect = %+v\nwant %+v", got, want)
	}
	if c := Candidates(files, want[1], got); !reflect.DeepEqual(c, []string{"web/src/sum.test.ts"}) {
		t.Fatalf("vitest candidates %v (node_modules must be skipped)", c)
	}
	// A pyproject.toml declares pytest only with its pytest section.
	py := MapTree{"pyproject.toml": "[project]\nname='x'\n", "svc/pyproject.toml": "[tool.pytest.ini_options]\n", "svc/uv.lock": ""}
	pyFiles, _ := py.Files()
	if got := Detect(pyFiles, py.Read); len(got) != 1 || got[0].Key != "pytest@svc" || got[0].Lockfile != "svc/uv.lock" {
		t.Fatalf("pytest detection %+v", got)
	}
}

// TIV-V1-006 to TIV-V1-009: one row per cause, each with its reason string.
func TestTIVV1006To009ReaderRules(t *testing.T) {
	base := ruleTree()
	fresh := encodedInventory(t, base, nil)
	type want struct {
		state, fileReason string
		rspec, vitest     InstanceVerdict
		undeclared        []string
	}
	planned := func(key, runner, root, config string, tests int, unassigned ...string) InstanceVerdict {
		return InstanceVerdict{Key: key, Runner: runner, Root: root, Config: config, Tests: tests, Unassigned: unassigned}
	}
	full := func(key, runner, root, config, reason string) InstanceVerdict {
		return InstanceVerdict{Key: key, Runner: runner, Root: root, Config: config, Full: true, Reason: reason}
	}
	rspecFresh := planned("rspec@.", "rspec", ".", ".rspec", 2)
	vitestFresh := planned("vitest@web", "vitest", "web", "web/vitest.config.ts", 1)
	cases := []struct {
		name string
		tree func() MapTree
		want want
	}{
		{"fresh", func() MapTree { tr := ruleTree(); tr[Path] = fresh; return tr }, want{StateRead, "", rspecFresh, vitestFresh, nil}},
		{"absent file", func() MapTree { return ruleTree() }, want{StateAbsent, ReasonAbsent,
			full("rspec@.", "rspec", ".", ".rspec", ReasonAbsent), full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonAbsent), nil}},
		{"absent record", func() MapTree {
			tr := ruleTree()
			tr[Path] = encodedInventory(t, base, func(inv *Inventory) { inv.Instances = inv.Instances[:1] })
			return tr
		}, want{StateRead, "", rspecFresh, full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonAbsent), nil}},
		{"absent after lister failure", func() MapTree {
			tr := ruleTree()
			tr[Path] = encodedInventory(t, base, func(inv *Inventory) {
				inv.Instances = inv.Instances[:1]
				inv.Failures = []Failure{{Key: "vitest@web", Runner: "vitest", Root: "web", Config: "web/vitest.config.ts",
					Lister: Lister{Name: "vitest-list", Version: "1", Kind: KindExecuting, Command: []string{"vitest", "list"}}, Error: "exit status 1"}}
			})
			return tr
		}, want{StateRead, "", rspecFresh, full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonAbsent), nil}},
		{"stale config", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			tr["web/vitest.config.ts"] = "export default { test: {} }\n"
			return tr
		}, want{StateRead, "", rspecFresh, full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonStale), nil}},
		{"stale lockfile", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			tr["Gemfile.lock"] = "GEM\n  rspec (4.0)\n"
			return tr
		}, want{StateRead, "", full("rspec@.", "rspec", ".", ".rspec", ReasonStale), vitestFresh, nil}},
		{"stale lockfile appeared", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			tr["web/yarn.lock"] = ""
			delete(tr, "web/package-lock.json")
			return tr
		}, want{StateRead, "", rspecFresh, full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonStale), nil}},
		{"stale lister version", func() MapTree {
			tr := ruleTree()
			tr[Path] = encodedInventory(t, base, func(inv *Inventory) { inv.Instances[0].Lister.Version = "0" })
			return tr
		}, want{StateRead, "", full("rspec@.", "rspec", ".", ".rspec", ReasonStale), vitestFresh, nil}},
		{"stale config moved", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			delete(tr, "web/vitest.config.ts")
			tr["web/vitest.config.mts"] = "export default {}\n"
			return tr
		}, want{StateRead, "", rspecFresh, full("vitest@web", "vitest", "web", "web/vitest.config.mts", ReasonStale), nil}},
		{"unassigned changed file", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			tr["spec/user_spec.rb"] += "describe 'more' do\n  it 'also' do\n  end\nend\n"
			return tr
		}, want{StateRead, "", planned("rspec@.", "rspec", ".", ".rspec", 1, "spec/user_spec.rb"), vitestFresh, nil}},
		{"unassigned new file", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			tr["web/src/mul.test.ts"] = "test('mul', () => {})\n"
			return tr
		}, want{StateRead, "", rspecFresh, planned("vitest@web", "vitest", "web", "web/vitest.config.ts", 1, "web/src/mul.test.ts"), nil}},
		{"invalid truncated", func() MapTree { tr := ruleTree(); tr[Path] = fresh[:len(fresh)/3]; return tr }, want{StateInvalid, ReasonInvalid,
			full("rspec@.", "rspec", ".", ".rspec", ReasonInvalid), full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonInvalid), nil}},
		{"invalid profile", func() MapTree {
			tr := ruleTree()
			tr[Path] = strings.Replace(fresh, Profile, "corvint-test-inventory/9", 1)
			return tr
		}, want{StateInvalid, ReasonInvalid,
			full("rspec@.", "rspec", ".", ".rspec", ReasonInvalid), full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonInvalid), nil}},
		{"invalid oversize", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh + strings.Repeat(" ", MaxBytes)
			return tr
		}, want{StateInvalid, ReasonInvalid,
			full("rspec@.", "rspec", ".", ".rspec", ReasonInvalid), full("vitest@web", "vitest", "web", "web/vitest.config.ts", ReasonInvalid), nil}},
		{"undeclared record ignored", func() MapTree {
			tr := ruleTree()
			tr[Path] = fresh
			delete(tr, ".rspec")
			delete(tr, "spec/spec_helper.rb")
			return tr
		}, want{StateRead, "", InstanceVerdict{}, vitestFresh, []string{"rspec@."}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verdict := Judge(c.tree())
			if verdict.State != c.want.state || verdict.Reason != c.want.fileReason {
				t.Fatalf("state %q reason %q detail %q, want %q %q", verdict.State, verdict.Reason, verdict.Detail, c.want.state, c.want.fileReason)
			}
			got := map[string]InstanceVerdict{}
			for _, instance := range verdict.Instances {
				instance.Cause, instance.Removed = "", nil
				got[instance.Runner] = instance
			}
			if c.want.rspec.Key != "" && !reflect.DeepEqual(got["rspec"], c.want.rspec) {
				t.Errorf("rspec %+v\nwant  %+v", got["rspec"], c.want.rspec)
			}
			if !reflect.DeepEqual(got["vitest"], c.want.vitest) {
				t.Errorf("vitest %+v\nwant   %+v", got["vitest"], c.want.vitest)
			}
			if !reflect.DeepEqual(verdict.Undeclared, c.want.undeclared) {
				t.Errorf("undeclared %v, want %v", verdict.Undeclared, c.want.undeclared)
			}
			for _, instance := range verdict.Instances {
				if instance.Full && instance.Reason == "" {
					t.Errorf("full instance %s carries no reason", instance.Key)
				}
			}
		})
	}
}

// TIV-V1-003: a full selection of one runner covers every instance of it.
func TestTIVV1003ForceFullCoversEveryInstance(t *testing.T) {
	tree := MapTree{
		"apps/web/jest.config.js": "", "apps/web/a.test.js": "",
		"apps/admin/jest.config.js": "", "apps/admin/b.test.js": "",
		"go.mod": "module x\n", "x_test.go": "package x\n",
	}
	tree[Path] = encodedInventory(t, tree, nil)
	verdict := Judge(tree)
	verdict.ForceFull("jest")
	fullJest := 0
	for _, instance := range verdict.Instances {
		switch instance.Runner {
		case "jest":
			if !instance.Full || instance.Reason != ReasonSelectionFull {
				t.Errorf("jest instance %s not full: %+v", instance.Key, instance)
			}
			fullJest++
		case "go":
			if instance.Full {
				t.Errorf("go instance forced: %+v", instance)
			}
		}
	}
	if fullJest != 2 {
		t.Fatalf("%d jest instances full, want 2: %+v", fullJest, verdict.Instances)
	}
	if got := verdict.Reasons(); !reflect.DeepEqual(got, []string{ReasonSelectionFull}) {
		t.Fatalf("reasons %v", got)
	}
}
