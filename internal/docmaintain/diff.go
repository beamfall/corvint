package docmaintain

import (
	"bytes"
	"fmt"
	"strings"
)

// unifiedDiff renders a minimal unified diff between before and after, using
// an O(n*m) LCS over lines. Session pages are small (bounded generated
// blocks over a human page), so this stays cheap; it exists only for the
// session preview/receipt, never to drive the write itself.
func unifiedDiff(label string, before, after []byte) string {
	if bytes.Equal(before, after) {
		return ""
	}
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	ops := lcsOps(beforeLines, afterLines)

	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n", label, label)
	for _, op := range ops {
		switch op.kind {
		case ' ':
			fmt.Fprintf(&out, " %s\n", op.text)
		case '-':
			fmt.Fprintf(&out, "-%s\n", op.text)
		case '+':
			fmt.Fprintf(&out, "+%s\n", op.text)
		}
	}
	return out.String()
}

type diffOp struct {
	kind byte
	text string
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSuffix(string(data), "\n")
	return strings.Split(trimmed, "\n")
}

// lcsOps computes a longest-common-subsequence-based edit script over lines.
func lcsOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	table := make([][]int, n+1)
	for i := range table {
		table[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}
