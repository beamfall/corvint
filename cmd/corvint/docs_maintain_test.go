package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsMaintainRequiresEnableAndApplyOrPreview checks the CLI's own gates:
// missing --enable, and missing both --preview/--apply, are rejected before
// any repository access happens.
func TestDocsMaintainRequiresEnableAndApplyOrPreview(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	cases := [][]string{
		{"--root", root, "docs", "maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--preview"},
		{"--root", root, "docs", "maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--enable"},
	}
	for _, arguments := range cases {
		_, isMaintain, err := parseDocsMaintainInvocation(arguments)
		if !isMaintain {
			t.Fatalf("expected docs maintain to be recognized for %v", arguments)
		}
		if err == nil {
			t.Fatalf("expected an argument error for %v", arguments)
		}
	}
}

// TestDocsMaintainPreviewThenApplyRoundTrip drives the CLI verb end to end on
// a real fixture: preview reports an eligible block and writes nothing, then
// apply writes it, and a second apply is a byte-identical no-op.
func TestDocsMaintainPreviewThenApplyRoundTrip(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	pagePath := filepath.Join(root, "docs", "page.md")
	if err := os.MkdirAll(filepath.Dir(pagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pagePath, []byte("# Page\n\nHuman intro.\n\n<!-- corvint:docmaintain:insertion-point -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previewArgs := []string{"--root", root, "docs", "maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--enable", "--preview"}
	options, isMaintain, err := parseDocsMaintainInvocation(previewArgs)
	if !isMaintain || err != nil {
		t.Fatalf("parse preview: isMaintain=%v err=%v", isMaintain, err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDocsMaintain(context.Background(), options, &stdout, &stderr); code != 0 {
		t.Fatalf("preview exit=%d stderr=%s", code, stderr.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("preview output not JSON: %v\n%s", err, stdout.String())
	}
	if receipt["applied"] != false || receipt["preview_only"] != true {
		t.Fatalf("unexpected preview receipt: %v", receipt)
	}
	onDisk, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "corvint:docmaintain begin") {
		t.Fatal("preview wrote to disk")
	}

	applyArgs := []string{"--root", root, "docs", "maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--enable", "--apply"}
	options, isMaintain, err = parseDocsMaintainInvocation(applyArgs)
	if !isMaintain || err != nil {
		t.Fatalf("parse apply: isMaintain=%v err=%v", isMaintain, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runDocsMaintain(context.Background(), options, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit=%d stderr=%s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("apply output not JSON: %v", err)
	}
	if receipt["applied"] != true {
		t.Fatalf("expected applied=true: %v", receipt)
	}
	afterFirstApply, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(afterFirstApply), "Human intro.") {
		t.Fatal("human paragraph lost")
	}
	if !strings.Contains(string(afterFirstApply), "corvint:docmaintain begin") {
		t.Fatal("generated block missing after apply")
	}

	stdout.Reset()
	stderr.Reset()
	if code := runDocsMaintain(context.Background(), options, &stdout, &stderr); code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("second apply output not JSON: %v", err)
	}
	if receipt["applied"] != false {
		t.Fatalf("second apply rewrote an unchanged block: %v", receipt)
	}
	afterSecondApply, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterSecondApply) != string(afterFirstApply) {
		t.Fatal("second apply changed page bytes despite no eligible change")
	}
}

func TestDocsMaintainWatchGrammar(t *testing.T) {
	base := []string{"--root", "/definitely-not-a-repository", "docs", "maintain", "--page", "page.md", "--source", "owner.md", "--package", "widget"}
	for _, flags := range [][]string{
		{"--enable", "--preview", "--watch"}, {"--enable", "--apply", "--preview"},
		{"--enable=true", "--apply"}, {"--enable", "--apply", "--watch=true"},
		{"--enable", "--apply", "--watch", "--watch"}, {"--enable", "--apply", "--apply"},
		{"--enable", "--apply", "--watch", "--max-writes", "1025"},
		{"--enable", "--apply", "--watch", "--max-wall-clock", "25h"},
	} {
		_, recognized, err := parseDocsMaintainInvocation(append(append([]string{}, base...), flags...))
		if !recognized || err == nil {
			t.Fatalf("flags=%v recognized=%v err=%v", flags, recognized, err)
		}
	}
}
