//go:build darwin || linux

package trace

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestStableMetadataIncludesChangeTime(t *testing.T) {
	name := filepath.Join(t.TempDir(), "trace.jsonl")
	if err := os.WriteFile(name, []byte("trace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	// A change that lands within one ctime tick is invisible to the witness
	// (coarse ctime on ext4 containers): re-apply the mode round trip until the
	// change time actually advances, bounded, and skip when it never does.
	after := advanceChangeTime(t, name, before, func() {
		if err := os.Chmod(name, 0o400); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(name, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if sameMetadata(before, after) {
		t.Fatal("ctime-only metadata change accepted")
	}
}

func advanceChangeTime(t *testing.T, name string, before os.FileInfo, apply func()) os.FileInfo {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		apply()
		after, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if changeTime(t, after) != changeTime(t, before) {
			return after
		}
		if time.Now().After(deadline) {
			t.Skip("file change time did not advance within 2s on this filesystem")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func changeTime(t *testing.T, info os.FileInfo) syscall.Timespec {
	t.Helper()
	stat := reflect.ValueOf(info.Sys()).Elem()
	for _, field := range []string{"Ctimespec", "Ctim"} {
		if value := stat.FieldByName(field); value.IsValid() {
			return value.Interface().(syscall.Timespec)
		}
	}
	t.Fatal("stat carries no change time field")
	return syscall.Timespec{}
}
