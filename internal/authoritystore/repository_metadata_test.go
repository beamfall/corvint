//go:build darwin || linux

package authoritystore

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestRepositoryConfigurationDetectsRestoredBytesAndMissingTransitions(t *testing.T) {
	t.Run("PLE-V0-003 configuration witness detects restored bytes", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config")
		before, err := readRepositoryMetadata(p)
		if err != nil || before.present {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("[core]\n"), 0600); err != nil {
			t.Fatal(err)
		}
		first, err := readRepositoryMetadata(p)
		if err != nil {
			t.Fatal(err)
		}
		// A restore that lands within one ctime tick is invisible to the witness
		// (coarse ctime on ext4 containers): re-apply the identical-bytes restore
		// until the change time actually advances, bounded; skip when it never does.
		advanceChangeTime(t, p, first.info, func() {
			if err := os.WriteFile(p, []byte("[core]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(p, first.info.ModTime(), first.info.ModTime()); err != nil {
				t.Fatal(err)
			}
		})
		b := repositoryBinding{configuration: []repositoryMetadata{first}}
		if b.configurationUnchanged() == nil {
			t.Fatal("restored content/mtime hid change time")
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if b.configurationUnchanged() == nil {
			t.Fatal("removed config")
		}
		if err := os.Symlink("missing", p); err != nil {
			t.Fatal(err)
		}
		if _, err := readRepositoryMetadata(p); err == nil {
			t.Fatal("configuration symlink")
		}

	})
}

func advanceChangeTime(t *testing.T, name string, before os.FileInfo, apply func()) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		apply()
		after, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if changeTime(t, after) != changeTime(t, before) {
			return
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
