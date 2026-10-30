package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// PSR-V0-006: file overrides captured ambient, literal values, phase filtering and snapshot stability.
func TestPSREnvironmentSnapshot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "private.env")
	t.Setenv("PSR_ONE", "ambient")
	t.Setenv("PSR_TWO", "two")
	if e := os.WriteFile(file, []byte("PSR_ONE=$(literal)\n"), 0600); e != nil {
		t.Fatal(e)
	}
	env, e := loadSweepEnvironment(root, []string{"PSR_ONE", "PSR_TWO"}, file)
	if e != nil {
		t.Fatal(e)
	}
	got, _ := env.phase([]string{"PSR_ONE"})
	if !reflect.DeepEqual(got, []string{"PSR_ONE=$(literal)"}) {
		t.Fatal(got)
	}
	if e = os.Remove(file); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PSR_ONE", "changed")
	again, _ := env.phase([]string{"PSR_ONE"})
	if !reflect.DeepEqual(got, again) {
		t.Fatal("mutable snapshot")
	}
}
func TestPSREnvironmentRefusals(t *testing.T) {
	for _, test := range []struct {
		name, body string
		mode       os.FileMode
	}{{"duplicate", "KEY=a\nKEY=b\n", 0600}, {"undeclared", "OTHER=x\n", 0600}, {"shell", "export KEY=x\n", 0600}, {"public", "KEY=x\n", 0644}, {"nul", "KEY=\x00\n", 0600}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "env")
			if e := os.WriteFile(file, []byte(test.body), test.mode); e != nil {
				t.Fatal(e)
			}
			if _, e := loadSweepEnvironment(root, []string{"KEY"}, file); e == nil {
				t.Fatal("accepted invalid env")
			}
		})
	}
	root := t.TempDir()
	target := filepath.Join(root, "real")
	if e := os.WriteFile(target, []byte("KEY=x\n"), 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "link")
	if e := os.Symlink(target, link); e != nil {
		t.Fatal(e)
	}
	if _, e := loadSweepEnvironment(root, []string{"KEY"}, link); e == nil {
		t.Fatal("symlink accepted")
	}
}

func TestPSREnvironmentParentAndReadRace(t *testing.T) {
	for _, mode := range []string{"parent-link", "leaf-swap", "parent-swap", "bytes-after-read", "size-after-read"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "private")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "env")
			if err := os.WriteFile(path, []byte("KEY=x\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadSweepEnvironment(root, []string{"KEY"}, path); err != nil {
				t.Fatal("valid initial fixture", err)
			}
			reached := false
			hook := func(stage, path string) {
				if reached || (strings.Contains(mode, "after-read") && stage != "afterRead") || (!strings.Contains(mode, "after-read") && stage != "beforeRead") {
					return
				}
				reached = true
				switch mode {
				case "leaf-swap":
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("KEY=x\n"), 0600); err != nil {
						t.Fatal(err)
					}
				case "parent-link":
					if err := os.Rename(dir, dir+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(dir+".old", dir); err != nil {
						t.Fatal(err)
					}
				case "parent-swap":
					if err := os.Rename(dir, dir+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(dir, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Link(filepath.Join(dir+".old", "env"), path); err != nil {
						t.Fatal(err)
					}
				default:
					body := "KEY=y\n"
					if mode == "size-after-read" {
						body = "KEY=longer\n"
					}
					if err := os.WriteFile(path, []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
					at := time.Now().Add(time.Hour)
					if err := os.Chtimes(path, at, at); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := loadSweepEnvironmentAt(root, []string{"KEY"}, path, hook); err == nil || !reached {
				t.Fatal("race not refused/reached", err, reached)
			}
		})
	}
	root := t.TempDir()
	path := filepath.Join(root, "env")
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"64KiB", "KEY=" + strings.Repeat("x", 65531) + "\n", true}, {"over64KiB", "KEY=" + strings.Repeat("x", 65532) + "\n", false}, {"invalidUTF8", string([]byte{0xff}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := loadSweepEnvironment(root, []string{"KEY"}, path)
			if (err == nil) != tc.want {
				t.Fatal(err)
			}
		})
	}
	keys := []string{}
	lines := []string{}
	for i := 0; i < 65; i++ {
		keys = append(keys, fmt.Sprintf("K%d", i))
		lines = append(lines, fmt.Sprintf("K%d=x", i))
	}
	for _, n := range []int{64, 65} {
		if err := os.WriteFile(path, []byte(strings.Join(lines[:n], "\n")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := loadSweepEnvironment(root, keys, path)
		if (err == nil) != (n == 64) {
			t.Fatal("line bound", n, err)
		}
	}
	for _, bad := range []string{filepath.Join(root, "missing"), root + "/./env", root + "/../env"} {
		if _, err := loadSweepEnvironment(root, keys, bad); err == nil {
			t.Fatal("bad path", bad)
		}
	}
}
