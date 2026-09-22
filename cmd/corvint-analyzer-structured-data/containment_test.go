package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// directoryDigest is a write/content/mode spy. It follows no links, captures
// every entry's type and mode, and hashes regular-file contents.
func directoryDigest(t testing.TB, root string) string {
	t.Helper()
	entries := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		row := strings.TrimPrefix(path, root) + ":" + entry.Type().String() + ":" + info.Mode().String()
		if entry.Type().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			row += ":" + hex.EncodeToString(sum[:])
		}
		entries = append(entries, row)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}

func TestAmbientFilesystemSpiesAndPositiveControls(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	cwd := filepath.Join(root, "cwd")
	for _, path := range []string{home, cwd} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	probe := filepath.Join(home, "read-only-probe")
	if err := os.WriteFile(probe, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	before := directoryDigest(t, root)
	binary := buildCandidate(t)
	command := exec.Command(binary)
	command.Dir = cwd
	command.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(root, "poison"), "CORVINT_SDA_CANARY=present"}
	command.Stdin = bytes.NewReader(cliFrame(t, "request-isolated"))
	output, err := command.Output()
	if err != nil || !bytes.Contains(output, []byte(`"status":"CANDIDATE"`)) {
		t.Fatalf("isolated run err=%v output=%s", err, output)
	}
	if after := directoryDigest(t, root); after != before {
		t.Fatalf("candidate changed filesystem content/mode: %s -> %s", before, after)
	}

	if body, err := os.ReadFile(probe); err != nil || string(body) != "before" {
		t.Fatalf("read positive control body=%q err=%v", body, err)
	}
	if err := os.WriteFile(probe, []byte("after"), 0400); err != nil {
		t.Fatal(err)
	}
	if directoryDigest(t, root) == before {
		t.Fatal("write/content positive control was not observed")
	}
	afterWrite := directoryDigest(t, root)
	if err := os.Chmod(probe, 0400); err != nil {
		t.Fatal(err)
	}
	if directoryDigest(t, root) == afterWrite {
		t.Fatal("mode positive control was not observed")
	}
}

func TestAmbientEnvironmentWorkingDirectoryProcessNetworkAndSyscallControls(t *testing.T) {
	binary := buildCandidate(t)
	t.Setenv("CORVINT_SDA_CANARY", "present")
	if value, ok := os.LookupEnv("CORVINT_SDA_CANARY"); !ok || value != "present" {
		t.Fatal("environment positive control was not observed")
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	canonicalCWD, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.Getwd(); err != nil || got != canonicalCWD {
		t.Fatalf("working-directory positive control got=%q err=%v", got, err)
	}

	command := exec.Command(binary, "--version")
	if output, err := command.Output(); err != nil || !bytes.Contains(output, []byte(`"status":"UNSELECTED"`)) {
		t.Fatalf("process positive control output=%s err=%v", output, err)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatal("process positive control did not observe clean child exit")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
		}
		accepted <- acceptErr
	}()
	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	if os.Getpid() <= 0 {
		t.Fatal("syscall positive control did not return a process identifier")
	}
}

func TestCandidateDependencyClosureHasNoAmbientChannels(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "-deps", "./cmd/corvint-analyzer-structured-data")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"os/exec", "net", "net/http", "plugin", "runtime/cgo"} {
		if bytes.Contains(output, []byte(forbidden+"\n")) {
			t.Fatalf("ambient dependency %q in closure", forbidden)
		}
	}
}
