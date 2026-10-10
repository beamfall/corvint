// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// V1-1094: the reuse step, the advisory drift report and the qualification campaign name the
// go-product-shard count separately. A count that drifts from the matrix makes every main push
// FULL (no record name matches), every drift report abstain, and a qualification bind another
// partition, all silently; so each must equal the matrix length.
func TestAFPV0024WorkflowShardCountsAgree(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ci := read("../../.github/workflows/ci.yml")
	qualification := read("../../.github/workflows/pr-tests-qualification.yml")
	one := func(name, text, pattern string) string {
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%s: %d matches of %q, want 1", name, len(m), pattern)
		}
		return m[0][1]
	}
	matrix := strings.Split(one("matrix", ci, `(?m)^        shard: \[([0-9, ]+)\]$`), ", ")
	for i, s := range matrix {
		if s != strconv.Itoa(i) {
			t.Fatalf("matrix = %v, want 0..N-1", matrix)
		}
	}
	want := strconv.Itoa(len(matrix))
	for name, got := range map[string]string{
		"REUSE_SHARDS":            one("REUSE_SHARDS", ci, `(?m)^          REUSE_SHARDS: "([0-9]+)"$`),
		"ci-shard-cost-drift":     one("SHARDS", ci, `(?m)^      SHARDS: "([0-9]+)"$`),
		"qualification default":   one("qualification", qualification, `(?m)^      shards:\n(?:        .*\n)*?        default: "([0-9]+)"$`),
		"qualification CI remark": one("qualification", qualification, `CI uses ([0-9]+);`),
	} {
		if got != want {
			t.Errorf("%s = %s, want the go-product-shard matrix length %s", name, got, want)
		}
	}
}
