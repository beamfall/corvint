package doccorpus

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaintenancePairRecovery(t *testing.T) {
	for _, kind := range []string{"success", "second-failure", "cancel", "competing-writer", "open-writer", "replaced-inode", "parent-swap", "create-race"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
				t.Fatal(err)
			}
			files := [2]MaintenanceFile{{Path: "docs/a.md", Before: []byte("old-a"), Next: []byte("next-a")}, {Path: "docs/b.json", Before: []byte("old-b"), Next: []byte("next-b")}}
			for _, f := range files {
				if err := os.WriteFile(filepath.Join(root, f.Path), f.Before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "create-race" {
				for i, f := range files {
					if err := os.Remove(filepath.Join(root, f.Path)); err != nil {
						t.Fatal(err)
					}
					files[i].Before = nil
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer, err := os.OpenFile(filepath.Join(root, files[1].Path), os.O_WRONLY, 0600)
			if err != nil && kind != "create-race" {
				t.Fatal(err)
			}
			if writer != nil {
				defer writer.Close()
			}
			hook := func(stage string, i int) {
				if i != 1 {
					return
				}
				if stage == "before-capture" {
					switch kind {
					case "second-failure":
						_ = os.WriteFile(filepath.Join(root, files[i].Path), []byte("competitor"), 0600)
					case "cancel":
						cancel()
					case "replaced-inode":
						_ = os.Rename(filepath.Join(root, files[i].Path), filepath.Join(root, "held-original"))
						_ = os.WriteFile(filepath.Join(root, files[i].Path), files[i].Before, 0600)
					case "parent-swap":
						_ = os.Rename(filepath.Join(root, "docs"), filepath.Join(root, "old-docs"))
						_ = os.Mkdir(filepath.Join(root, "docs"), 0700)
					}
				}
				if stage == "before-publish" {
					switch kind {
					case "competing-writer", "create-race":
						_ = os.WriteFile(filepath.Join(root, files[i].Path), []byte("competitor"), 0600)
					case "open-writer":
						_, _ = writer.WriteAt([]byte("writer"), 0)
					}
				}
			}
			states, err := publishMaintenancePair(ctx, root, files, nil, hook)
			success := kind == "success" || kind == "open-writer"
			if (err == nil) != success || len(states) != 2 || !states[0].Published {
				t.Fatalf("state %+v err %v", states, err)
			}
			if !success && (!strings.Contains(err.Error(), "incomplete") || states[1].Published) {
				t.Fatalf("partial hidden %+v %v", states, err)
			}
			if kind == "competing-writer" || kind == "create-race" || kind == "second-failure" {
				raw, _ := os.ReadFile(filepath.Join(root, files[1].Path))
				if string(raw) != "competitor" {
					t.Fatalf("competitor lost %q", raw)
				}
			}
			if kind == "open-writer" {
				raw, _ := os.ReadFile(filepath.Join(root, states[1].Recovery))
				if string(raw) != "writer" {
					t.Fatalf("open writer lost %q", raw)
				}
				raw, _ = os.ReadFile(filepath.Join(root, files[1].Path))
				if string(raw) != "next-b" {
					t.Fatal("writer reached new output")
				}
			}
			if kind == "success" {
				for i, s := range states {
					raw, e := os.ReadFile(filepath.Join(root, s.Recovery))
					if e != nil || string(raw) != string(files[i].Before) {
						t.Fatalf("recovery missing %q %v", raw, e)
					}
				}
			}
		})
	}
}

func TestMaintenancePairInterruptedProcess(t *testing.T) {
	if root := os.Getenv("CORVINT_PAIR_INTERRUPT_FIXTURE"); root != "" {
		files := [2]MaintenanceFile{{Path: "a", Before: []byte("old-a"), Next: []byte("new-a")}, {Path: "b", Before: []byte("old-b"), Next: []byte("new-b")}}
		_, err := publishMaintenancePair(context.Background(), root, files, nil, func(stage string, i int) {
			if stage == "before-capture" && i == 1 {
				fmt.Println("first-published")
				_, _ = io.Copy(io.Discard, os.Stdin)
			}
		})
		t.Fatalf("interrupted helper returned: %v", err)
	}
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("old-"+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMaintenancePairInterruptedProcess$")
	cmd.Env = append(os.Environ(), "CORVINT_PAIR_INTERRUPT_FIXTURE="+root)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(pipe).ReadString('\n')
	if err != nil || line != "first-published\n" {
		t.Fatalf("helper handshake %q %v", line, err)
	}
	if err = cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		t.Fatal("interrupted publication succeeded")
	}
	a, _ := os.ReadFile(filepath.Join(root, "a"))
	b, _ := os.ReadFile(filepath.Join(root, "b"))
	if string(a) != "new-a" || string(b) != "old-b" {
		t.Fatalf("partial state %q %q", a, b)
	}
	names, _ := filepath.Glob(filepath.Join(root, ".corvint-flow-recovery-*"))
	if len(names) != 1 {
		t.Fatalf("original inode recovery missing: %v", names)
	}
	original, _ := os.ReadFile(names[0])
	if string(original) != "old-a" {
		t.Fatal("original lost")
	}
}
