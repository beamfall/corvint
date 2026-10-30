package contextindex

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"
)

func ancestorInstructionFixture(t *testing.T) *Index {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":    "module example.test/ancestors\n\ngo 1.27.0\n",
		"AGENTS.md": "Root policy.\n", "CLAUDE.md": "Root addendum.\n",
		"app/AGENTS.md": "App policy.\n", "app/CLAUDE.md": "App addendum.\n",
		"app/deep/AGENTS.md": "Local policy.\n", "app/deep/CLAUDE.md": "Local addendum.\n",
		"app/other/AGENTS.md": "Sibling policy.\n", "application/AGENTS.md": "Prefix decoy.\n",
		"app/deep/source.go": "package deep\nfunc Work() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestTaskContextAncestorInstructions(t *testing.T) {
	index := ancestorInstructionFixture(t)
	want := []string{"app/deep/AGENTS.md", "app/deep/CLAUDE.md", "app/AGENTS.md", "app/CLAUDE.md", "AGENTS.md", "CLAUDE.md"}
	t.Run("TCP-V0-008 all ancestors and stable directory precedence", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "adjust behavior", "app/deep/source.go", 20)
		if err != nil {
			t.Fatal(err)
		}
		if got := contextRowsByKind(t, packet)["governing"]; !slices.Equal(got, want) {
			t.Fatalf("governing %v, want %v", got, want)
		}
		states, withheld := contextUnexamined(t, contextCoverage(t, packet))
		if states["governing"] != "examined" || contextIntValue(withheld["governing"]) != 0 {
			t.Fatalf("scope %v / %v", states, withheld)
		}
	})
	t.Run("TCP-V0-011 small budgets disclose every missing reservation", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "adjust behavior", "app/deep/source.go", 1)
		if err != nil {
			t.Fatal(err)
		}
		rows := mapsFromAny(packet["results"])
		coverage := contextCoverage(t, packet)
		if len(rows) != 1 || rows[0]["id"] != want[0] {
			t.Fatalf("closest missing: %v", rows)
		}
		missing := contextSelectors(t, coverage, "critical_missing")
		if len(missing) != 5 || coverage["budget_shortage"] != "slots" {
			t.Fatalf("shortage: %v", coverage)
		}
		for _, p := range want[1:] {
			if !slices.Contains(missing, "governing "+p) {
				t.Fatalf("missing selector %s: %v", p, missing)
			}
		}
	})
	t.Run("TCP-V0-008 subject instruction never reserves itself", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "adjust policy", "app/deep/AGENTS.md", 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range mapsFromAny(packet["results"]) {
			if row["id"] == "app/deep/AGENTS.md" {
				t.Fatalf("subject returned: %v", row)
			}
		}
		if got := contextRowsByKind(t, packet)["governing"]; !slices.Equal(got, want[1:]) {
			t.Fatalf("governing: %v", got)
		}
	})
	t.Run("TCP-V0-008 subjectless precedence is unchanged", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "adjust behavior", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if got := contextRowsByKind(t, packet)["governing"]; !slices.Equal(got, []string{"AGENTS.md"}) {
			t.Fatalf("subjectless: %v", got)
		}
	})
	t.Run("TCP-V0-011 unread ancestor remains an explicit gap", func(t *testing.T) {
		compiler := newTaskContextCompiler(index, "adjust behavior", "app/deep/source.go")
		local := *index
		local.Sources = map[string]Source{}
		for name, source := range index.Sources {
			local.Sources[name] = source
		}
		delete(local.Sources, want[0])
		local.Exclusions = append(slices.Clone(index.Exclusions), Exclusion{want[0], "secret-screened fixture"})
		compiler.index = &local
		rows := compiler.governingRows()
		compiler.reserved = rows
		if len(rows) != 5 || rows[0].path != want[1] || compiler.relationState[governingRelation] != "capped" || compiler.withheld(governingRelation, "capped") != 1 {
			t.Fatalf("unread scope: %v / %v", rows, compiler.unexamined())
		}
	})
}

func TestTaskContextAncestorInstructionCap(t *testing.T) {
	t.Run("TCP-V0-011 bounded ancestor scan discloses withheld paths", func(t *testing.T) {
		index := &Index{Tracked: map[string]struct{}{}, Sources: map[string]Source{}}
		directory := "."
		for i := 0; i < contextGoverningCap/2+2; i++ {
			for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
				p := path.Join(directory, name)
				index.Tracked[p] = struct{}{}
				index.Sources[p] = Source{Data: []byte("policy\n")}
			}
			directory = path.Join(directory, fmt.Sprintf("d%d", i))
		}
		compiler := newTaskContextCompiler(index, "adjust behavior", path.Join(directory, "source.go"))
		rows := compiler.governingRows()
		compiler.reserved = rows
		if len(rows) != contextGoverningCap || compiler.relationState[governingRelation] != "capped" || compiler.withheld(governingRelation, "capped") != 4 || compiler.budgetShortage() != "slots" {
			t.Fatalf("cap rows=%d scope=%v", len(rows), compiler.unexamined())
		}
	})
}

func TestTaskContextRoutesFromAncestorInstructions(t *testing.T) {
	t.Run("TCP-V0-047 ancestor routes retain source and shared cap", func(t *testing.T) {
		index := routedIndex(t, map[string]string{
			"go.mod":        "module example.test/ancestorroute\n\ngo 1.27.0\n",
			"AGENTS.md":     "Backlog ledger: `root.md` and `extra.md`.\n",
			"app/AGENTS.md": "Backlog ledger: `local.md`.\n",
			"app/source.go": "package app\nfunc Work() {}\n",
			"local.md":      "Local guidance.\n", "root.md": "Root guidance.\n", "extra.md": "Extra guidance.\n",
		})
		packet, err := TaskContext(context.Background(), index, "backlog ledger", "app/source.go", 20)
		if err != nil {
			t.Fatal(err)
		}
		got := routedRows(t, packet)
		if len(got) != 2 || !strings.HasPrefix(got[0], "local.md | ") || !strings.Contains(got[0], "app/AGENTS.md:1") || !strings.HasPrefix(got[1], "root.md | ") || !strings.Contains(got[1], "AGENTS.md:1") {
			t.Fatalf("routes: %v", got)
		}
		compiler := newTaskContextCompiler(index, "backlog ledger", "app/source.go")
		compiler.reserved = compiler.reservedRows()
		states, withheld := contextUnexamined(t, map[string]any{"unexamined": compiler.unexamined()})
		if states[instructionRoutedRelation] != "examined" || contextIntValue(withheld[instructionRoutedRelation]) != 1 {
			t.Fatalf("route scope: %v / %v", states, withheld)
		}
	})
}
