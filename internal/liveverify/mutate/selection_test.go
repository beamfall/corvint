// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestRSPV0033GoSelectionOperatorsReuseLanguagePiece(t *testing.T) {
	raw := []byte("package p\nconst N = 2\nfunc F(n int) int { if n > 0 { return 1 }; return 0 }\n")
	population, err := SelectionPopulation(raw, 100)
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]bool{}
	operators := map[string]bool{}
	for _, op := range SelectionOperators() {
		operators[op] = true
	}
	for _, m := range population {
		classes[m.Class] = true
		if !operators[m.Operator] || bytes.Equal(raw, m.Source) || m.Start < 0 || m.End <= m.Start {
			t.Fatalf("%+v", m)
		}
	}
	if !classes["body-edit"] || !classes["declaration-edit"] {
		t.Fatal(classes)
	}
}
func TestRSPV0033GoSelectionMutationRejectsMalformedSource(t *testing.T) {
	if p, err := SelectionPopulation([]byte("package p\nfunc {"), 10); err == nil || p != nil {
		t.Fatal("malformed population accepted")
	}
}
func TestRSPV0035GoSelectionPopulationIsDeterministicAndComplete(t *testing.T) {
	raw := []byte("package p\nconst N=2\nfunc F() int { return 1 }\n")
	a, err := SelectionPopulation(raw, 100)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SelectionPopulation(raw, 100)
	if err != nil || !reflect.DeepEqual(a, b) || len(a) != 2 {
		t.Fatalf("%+v %+v %v", a, b, err)
	}
}
func TestRSPV0035GoSelectionPopulationCapNeverReturnsPrefix(t *testing.T) {
	raw := []byte("package p\nconst N=2\nfunc F() int { return 1 }\n")
	for _, cap := range []int{0, 1, 4097} {
		if p, err := SelectionPopulation(raw, cap); err == nil || p != nil {
			t.Fatalf("cap=%d admitted prefix", cap)
		}
	}
	if p, err := SelectionPopulation([]byte(strings.Repeat(" ", 1<<20+1)), 10); err == nil || p != nil {
		t.Fatal("source cap ignored")
	}
}

func TestRSPV0035GoUnparseableMutantsRefuseWholePopulation(t *testing.T) {
	raw := []byte("package p\nfunc F() int { return 1 }\nfunc G() int { return 2 }\nfunc H() int { return 3 }\n")
	calls := 0
	population, err := selectionPopulation(raw, 100, func(source []byte, index int) ([]byte, bool) {
		calls++
		if index == 0 || index == 2 {
			return nil, false
		}
		return renderMutant(source, index)
	})
	if population != nil || err == nil || calls != 3 {
		t.Fatalf("partial population or missing attempts: %+v %v calls=%d", population, err, calls)
	}
	for _, reason := range []string{"go-mutation-unparseable:replace-literal:2:", "go-mutation-unparseable:replace-literal:4:"} {
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("unparseable candidate not accounted for: %s", err)
		}
	}
	if strings.Count(err.Error(), "go-mutation-unparseable:") != 2 {
		t.Fatalf("failure count: %s", err)
	}
}
