package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// This direct boundary witness starts no Git or adapter child. Supported argv
// must reach the invalid scratch owner; forbidden argv must fail before it.
func TestProducerOperationBoundary(t *testing.T) {
	root := t.TempDir()
	owner := filepath.Join(root, "corvint-work-queue-owner")
	const sentinel = "invalid owner must remain unchanged\n"
	if err := os.WriteFile(owner, []byte(sentinel), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", root)
	check := func(t *testing.T, arguments []string, want string) {
		t.Helper()
		if err := runProducer(arguments); err == nil || err.Error() != want {
			t.Fatalf("runProducer(%q) = %v; want %q", arguments, err, want)
		}
		raw, err := os.ReadFile(owner)
		if err != nil || !bytes.Equal(raw, []byte(sentinel)) {
			t.Fatalf("scratch sentinel changed: %q, %v", raw, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != "corvint-work-queue-owner" {
			t.Fatalf("scratch fixture changed: %v, %v", entries, err)
		}
		info, err := os.Lstat(owner)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("scratch sentinel type/mode changed: %v, %v", info, err)
		}
	}
	t.Run("WQO-V0-007 closed operation argv before acquisition", func(t *testing.T) {
		for _, operation := range []string{"snapshot", "details", "verify"} {
			t.Run(operation, func(t *testing.T) {
				check(t, []string{operation}, "invalid source scratch owner")
			})
		}
		for _, arguments := range [][]string{nil, {"snapshot", "extra"}, {"details", "--path", "outside"}, {"verify", "--operation", "claim"}} {
			check(t, arguments, "usage: corvint-work-queue snapshot|details|verify")
		}
		for _, operation := range []string{"", "pass-through", "sh", "SNAPSHOT"} {
			check(t, []string{operation}, "unsupported operation")
		}
	})
	t.Run("WQO-V0-030 mutation verbs cannot reach acquisition", func(t *testing.T) {
		for _, operation := range []string{"claim", "release", "heartbeat", "dispatch", "review", "verdict", "finish", "merge", "close", "edit", "add", "amend", "dependency-write", "approval", "repair", "fanout-plan"} {
			t.Run(operation, func(t *testing.T) {
				check(t, []string{operation}, "unsupported operation")
			})
		}
		check(t, []string{"fanout-plan", "--claim"}, "usage: corvint-work-queue snapshot|details|verify")
	})
}
