package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// TestGPKV0071DubiousOwnershipIsOneCodedRefusal drives several Core verbs
// against a repository whose Git refuses it as owned by another user. A PATH
// shim stands in for Git, because the sanitized Git environment drops any
// variable that could make the real Git assume a different owner. Every verb
// must leave the same coded refusal naming the repository, never Git's
// uncoded exit or its unusable safe.directory advice (V1-0288; affected, init
// and overview since V1-2353).
func TestGPKV0071DubiousOwnershipIsOneCodedRefusal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Git shim is a POSIX shell script")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "a.go"}, {"-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.test", "commit", "-q", "-m", "init"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	script := "#!/bin/sh\nprintf \"fatal: detected dubious ownership in repository at '%s'\\nTo add an exception for this directory, call:\\n\\n\\tgit config --global --add safe.directory %s\\n\" \"@ROOT@\" \"@ROOT@\" >&2\nexit 128\n"
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The shim names the repository by a path baked into the script, because
	// Corvint runs Git in another directory with -C and a sanitized environment.
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(strings.ReplaceAll(script, "@ROOT@", root)), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, verb := range [][]string{{"index"}, {"query", "--task", "where is a"}, {"impact", "a.go"}, {"context", "a.go"}, {"feature", "a"}, {"affected", "--base", strings.TrimSpace(string(head))}, {"init"}, {"overview"}} {
		t.Run(verb[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runContext(context.Background(), append([]string{"--root", root}, verb...), strings.NewReader(""), &stdout, &stderr); code == 0 {
				t.Fatalf("%v must fail under a refused repository: %s", verb, stdout.String())
			}
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
				OK    *bool  `json:"ok"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &refusal); err != nil {
				t.Fatalf("%v: stderr is not one JSON refusal: %v: %s", verb, err, stderr.String())
			}
			if refusal.Code != "repository-ownership-refused" || refusal.OK == nil || *refusal.OK {
				t.Fatalf("%v: want code repository-ownership-refused, got %+v", verb, refusal)
			}
			if !strings.Contains(refusal.Error, root) && !strings.Contains(refusal.Error, filepath.Base(root)) {
				t.Fatalf("%v: refusal does not name the repository %s: %s", verb, root, refusal.Error)
			}
			if strings.Contains(refusal.Error, "git config --global") || !strings.Contains(refusal.Error, "safe.directory exception cannot take effect") {
				t.Fatalf("%v: refusal repeats Git's unusable advice or omits the remedy: %s", verb, refusal.Error)
			}
		})
	}
}

func TestGPKV0071DubiousOwnershipClassifierIsExact(t *testing.T) {
	t.Parallel()
	refused := dubiousOwnershipRefusal(&gokernel.Error{Message: "Git error: fatal: detected dubious ownership in repository at '/srv/r'\nTo add an exception"})
	var kernelError *gokernel.Error
	if refused == nil || !errors.As(refused, &kernelError) || kernelError.Code != "repository-ownership-refused" || !strings.Contains(kernelError.Message, "'/srv/r'") {
		t.Fatalf("classified refusal = %#v", refused)
	}
	for _, message := range []string{"Git error: fatal: not a git repository", "detected dubious ownership in repository at '", "detected dubious ownership in repository at '/srv/r"} {
		if refused := dubiousOwnershipRefusal(&gokernel.Error{Message: message}); refused != nil {
			t.Fatalf("%q must stay unclassified, got %v", message, refused)
		}
	}
}
