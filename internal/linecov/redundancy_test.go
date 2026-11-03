package linecov

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// redundancyFixture is package p's test file. TestBig and TestDup are the
// same test under two names; TestSameDiff executes the same blocks but asserts
// something else, which coverage alone cannot tell apart (LPCV-V0-964).
const redundancyFixture = `package p

import "testing"

// TestBig covers LPCV-V0-964.
func TestBig(t *testing.T) {
	if Do() != 1 {
		t.Fatal("one")
	}
}

func TestDup(t *testing.T) {
	if Do() != 1 {
		t.Fatal("one")
	}
}

func TestSameDiff(t *testing.T) {
	if Do() == 2 {
		t.Fatal("not two")
	}
}

func TestSmall(t *testing.T)      {}
func TestLow(t *testing.T)        {}
func TestUnion(t *testing.T)      {}
func TestUnique(t *testing.T)     {}
func TestNone(t *testing.T)       {}
func TestFailsAlone(t *testing.T) {}
`

func TestRedundancy(t *testing.T) {
	blocks := map[string][2]int{"A": {3, 4}, "B": {5, 6}, "C": {7, 8}, "D": {9, 10}, "E": {11, 12}, "F": {13, 14}, "G": {15, 16}, "H": {17, 18}}
	profile := func(ran string) []byte {
		var b strings.Builder
		b.WriteString("mode: atomic\n")
		for _, k := range "ABCDEFGH" {
			n := 0
			if strings.ContainsRune(ran, k) {
				n = 1
			}
			l := blocks[string(k)]
			fmt.Fprintf(&b, "example.com/m/p/p.go:%d.1,%d.2 1 %d\n", l[0], l[1], n)
		}
		return []byte(b.String())
	}
	runs := map[string]string{
		"TestBig": "ABC", "TestDup": "ABC", "TestSameDiff": "ABC", "TestSmall": "AB",
		"TestLow": "AG", "TestUnion": "AD", "TestUnique": "DEF", "TestNone": "",
		InOrder: "ABCDEFGH", // H only in order: an order-sensitive record
	}
	var records []Record
	for name, ran := range runs {
		records = append(records, Record{Package: "p", Name: name, Profile: profile(ran), Elapsed: time.Second})
	}
	ix, err := Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, records)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "p_test.go"), []byte(redundancyFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := TestSources(root, "p")
	if err != nil {
		t.Fatal(err)
	}
	declared, err := HeadTests(root, []string{"p"})
	if err != nil {
		t.Fatal(err)
	}
	got := ix.Redundancy(declared, map[string]map[string]Source{"p": src}, 1)
	req := []string{"LPCV-V0-964"}
	want := []PackageReport{{
		Package: "p", Tests: 8, InOrder: true, Unindexed: []string{"TestFailsAlone"},
		Candidates: []Candidate{
			{Package: "p", Name: "TestBig", Kind: Equal, Blocks: 3, By: []string{"TestDup", "TestSameDiff"}, SameBody: []string{"TestDup"}, Requirements: req, Elapsed: time.Second},
			{Package: "p", Name: "TestDup", Kind: Equal, Blocks: 3, By: []string{"TestBig", "TestSameDiff"}, SameBody: []string{"TestBig"}, Elapsed: time.Second},
			{Package: "p", Name: "TestLow", Kind: LowUnique, Blocks: 2, Unique: 1, Elapsed: time.Second},
			{Package: "p", Name: "TestNone", Kind: NoBlocks, Elapsed: time.Second},
			{Package: "p", Name: "TestSameDiff", Kind: Equal, Blocks: 3, By: []string{"TestBig", "TestDup"}, Elapsed: time.Second},
			{Package: "p", Name: "TestSmall", Kind: Subset, Blocks: 2, By: []string{"TestBig"}, Elapsed: time.Second},
			{Package: "p", Name: "TestUnion", Kind: Union, Blocks: 2, Elapsed: time.Second},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report:\n got %+v\nwant %+v", got, want)
	}
}
