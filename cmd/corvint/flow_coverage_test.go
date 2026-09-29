package main

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/flowcoverage"
	"github.com/Beamfall/corvint/internal/flowcoverage/testfixture"
	"path/filepath"
	"testing"
)

func TestFlowCoverageCLI(t *testing.T) {
	root := testfixture.Repository(t)
	var out, diag bytes.Buffer
	args := []string{"coverage", "--denominator", "denominator.json", "--receipts", "runs.json"}
	code := runFlows(context.Background(), root, args, &out, &diag)
	if code != 1 {
		t.Fatal(code, diag.String())
	}
	r, e := flowcoverage.Compile(context.Background(), root, flowcoverage.Options{Denominator: "denominator.json", Receipts: "runs.json"})
	if e != nil {
		t.Fatal(e)
	}
	page, _ := r.Page(0, 20)
	if !bytes.Equal(bytes.TrimSpace(out.Bytes()), page) {
		t.Fatal("CLI differs from compiler")
	}
	out.Reset()
	diag.Reset()
	code = runFlows(context.Background(), root, append(args, "--write-back", filepath.Join(root, "fresh")), &out, &diag)
	if code != 1 {
		t.Fatal(code, diag.String())
	}
	out.Reset()
	diag.Reset()
	code = runFlows(context.Background(), root, append(args, "--write-back", filepath.Join(root, "fresh")), &out, &diag)
	if code != 2 {
		t.Fatal("clobber", code)
	}
	if code = runFlows(context.Background(), root, []string{"coverage", "--denominator", "missing"}, &out, &diag); code != 2 {
		t.Fatal("invalid arguments", code)
	}
}
