//go:build darwin || linux

package jstestprovider

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// PTF-V0-003/007: unsupported inputs must fail closed without leaving a
// blocked reader behind. A child process bounds the pre-repair FIFO failure.
func TestPTFV0DependencySpecialFiles(t *testing.T) {
	if path := os.Getenv("CORVINT_PTF_DEPENDENCY_OPEN_CHILD"); path != "" {
		f, err := freshOpenDependency(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.Mode().IsRegular() {
			t.Fatalf("FIFO descriptor not rejected by regular-file check: %v", err)
		}
		return
	}
	if encoded := os.Getenv("CORVINT_PTF_DEPENDENCY_READER_CHILD"); encoded != "" {
		var m FreshDependencyManifest
		if err := json.Unmarshal([]byte(encoded), &m); err != nil {
			t.Fatal(err)
		}
		if o := ObserveFreshDependencies(&m); len(o.Failures) == 0 || FreshDependenciesMatch(&m, o) {
			t.Fatalf("special input admitted: %+v", o)
		}
		return
	}
	for _, kind := range []string{"root-fifo", "explicit-fifo", "root-socket", "explicit-directory", "open-fifo"} {
		t.Run(kind, func(t *testing.T) {
			root, err := os.MkdirTemp("/tmp", "ptf-special-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(root) })
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			valid := filepath.Join(root, "valid.cjs")
			if err := os.WriteFile(valid, []byte("module.exports=1;"), 0600); err != nil {
				t.Fatal(err)
			}
			special := filepath.Join(root, "unsupported")
			m := FreshDependencyManifest{Roots: []string{root}, Files: map[string]string{valid: sha256Hex([]byte("module.exports=1;"))}}
			switch kind {
			case "explicit-fifo", "explicit-directory":
				outside, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				special = filepath.Join(outside, "unsupported")
				m.Files[special] = sha256Hex(nil)
			}
			switch kind {
			case "root-socket":
				listener, err := net.Listen("unix", special)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
			case "explicit-directory":
				if err := os.Mkdir(special, 0700); err != nil {
					t.Fatal(err)
				}
			default:
				if err := syscall.Mkfifo(special, 0600); err != nil {
					t.Fatal(err)
				}
			}
			encoded, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPTFV0DependencySpecialFiles$")
			command.Env = append(os.Environ(), "CORVINT_PTF_DEPENDENCY_READER_CHILD="+string(encoded))
			if kind == "open-fifo" {
				command.Env = append(command.Env, "CORVINT_PTF_DEPENDENCY_OPEN_CHILD="+special)
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("special input reader did not return successfully (context=%v): %v\n%s", ctx.Err(), err, output)
			}
		})
	}
}
