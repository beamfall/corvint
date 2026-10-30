package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DCW-V0-347 (V1-2810): a passing check leaves a private receipt binding its
// verdict to BASE, HEAD and the inputs digest; a check with ReusePass replays
// it only while every bound input is unchanged, and any change, a check
// without ReusePass, or an aggregate check removes the receipt and reruns.
func TestDCWV0347SealReusesOnlyABoundPass(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(root+"/.corvint", 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(root+"/.corvint/dogfood-report.json", "report\n")
	write(root+"/.corvint/change.ocm-intents", "docs/specs/a.md\n")
	write(evidence+"/local-outcome.json", "outcome\n")
	verifier := filepath.Join(t.TempDir(), "verifier")
	write(verifier, "verifier")
	runner := Runner{Path: verifier}
	base, target := strings.Repeat("a", 40), strings.Repeat("b", 40)
	newRun := func(reuse bool, head string) (*check, *bytes.Buffer, *bytes.Buffer) {
		var stdout, stderr bytes.Buffer
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root, BaseVerifier: runner, TreeVerifier: runner, ReusePass: reuse}, &stdout, &stderr)
		c.base, c.target, c.evidence = base, head, evidence
		c.resolveVerifiers()
		return c, &stdout, &stderr
	}
	receipt := evidence + "/" + passReceiptName
	verdict, warning := "dogfood-check: SUMMARY cem=ready-for-ci\ndogfood-check: PASS\n", "dogfood-check: WARN tier-below-anchored hunks=1\n"
	pass := func() {
		t.Helper()
		c, _, _ := newRun(false, target)
		if c.reusePass() {
			t.Fatal("reused without ReusePass")
		}
		passed := c.capturePass()
		_, _ = io.WriteString(c.stdout, verdict)
		_, _ = io.WriteString(c.stderr, warning)
		write(evidence+"/dogfood-check.stdout", "detail\n")
		passed()
		if !isRegular(receipt) {
			t.Fatal("a passing check wrote no receipt")
		}
	}
	pass()
	// Same inputs: the verdict replays with a NOTE naming the receipt.
	started := time.Now()
	c, stdout, stderr := newRun(true, target)
	if !c.reusePass() || stdout.String() != verdict || !strings.HasPrefix(stderr.String(), "dogfood-check: NOTE reused-pass receipt="+receipt+" inputs=sha256:") || !strings.HasSuffix(stderr.String(), "(DCW-V0-347)\n"+warning) {
		t.Fatalf("reuse: stdout=%q stderr=%q", stdout, stderr)
	}
	t.Logf("reuse decision took %s", time.Since(started))
	// A ledger append is not an input.
	write(root+"/.corvint/self-observations.jsonl", "{}\n")
	if c, _, _ := newRun(true, target); !c.reusePass() {
		t.Fatal("a ledger append blocked reuse")
	}
	// Another HEAD, a changed .corvint input, a changed evidence file, a changed
	// verifier: each reruns and removes the receipt.
	for name, mutate := range map[string]func() string{
		"head":     func() string { return strings.Repeat("c", 40) },
		"report":   func() string { write(root+"/.corvint/dogfood-report.json", "report 2\n"); return target },
		"evidence": func() string { write(evidence+"/local-outcome.json", "outcome 2\n"); return target },
		"detail":   func() string { _ = os.Remove(evidence + "/dogfood-check.stdout"); return target },
		"verifier": func() string { write(verifier, "verifier 2"); return target },
	} {
		pass()
		head := mutate()
		c, stdout, stderr := newRun(true, head)
		if c.reusePass() || stdout.Len() != 0 || !strings.Contains(stderr.String(), "reused-pass NOT_ADMITTED") || isRegular(receipt) {
			t.Fatalf("%s: reused or kept the receipt: stdout=%q stderr=%q", name, stdout, stderr)
		}
	}
	// A receipt whose verdict is not a PASS, or a check without ReusePass,
	// never replays, and the receipt goes.
	pass()
	write(receipt, strings.Replace(string(readFile(receipt)), "dogfood-check: PASS", "dogfood-check: FAIL", 1))
	if c, stdout, _ := newRun(true, target); c.reusePass() || stdout.Len() != 0 {
		t.Fatal("replayed a receipt without PASS")
	}
	pass()
	if c, _, _ := newRun(false, target); c.reusePass() || isRegular(receipt) {
		t.Fatal("a check without ReusePass kept or used the receipt")
	}
	// An aggregate check neither reuses nor captures.
	pass()
	c, _, _ = newRun(true, target)
	c.aggregate = &AggregateReport{}
	if c.reusePass() || isRegular(receipt) {
		t.Fatal("an aggregate check reused a receipt")
	}
	// Seal always asks for reuse.
	if got := newCheck(context.Background(), "", CheckOptions{}, io.Discard, io.Discard); got.options.ReusePass {
		t.Fatal("a plain check defaults to reuse")
	}
}
