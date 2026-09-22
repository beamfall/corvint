//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloneTrustChildTransport(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "source")
		env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_COUNT=0", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
		run := func(e []string, args ...string) (int, string) {
			t.Helper()
			var b bytes.Buffer
			code, err := command(context.Background(), dir, e, &b, &b, 10*time.Second, "git", args...)
			if err != nil {
				t.Fatal(err)
			}
			return code, b.String()
		}
		for _, name := range []string{root, filepath.Join(dir, "sibling")} {
			if code, out := run(env, "init", "-q", name); code != 0 {
				t.Fatal(out)
			}
			if code, out := run(env, "-C", name, "commit", "--allow-empty", "-qm", "fixture"); code != 0 {
				t.Fatal(out)
			}
		}
		record := filepath.Join(dir, "child-config")
		wrapper := filepath.Join(dir, "upload-pack")
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
		script := "#!/bin/sh\ngit config --get-all safe.directory > " + quote(record) + "\nexec git upload-pack \"$@\"\n"
		if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		clone := func(e []string, name string, trust ...string) {
			t.Helper()
			args := append(trust, "clone", "--no-hardlinks", "--no-checkout", "--upload-pack="+wrapper, root, filepath.Join(dir, name))
			if code, out := run(e, args...); code != 0 {
				t.Fatal(out)
			}
		}
		clone(env, "command-scope", "-c", "safe.directory="+root, "-c", "safe.directory="+root+"/.git")
		if b, err := os.ReadFile(record); err != nil || len(b) != 0 {
			t.Fatalf("command trust unexpectedly reached child: %q %v", b, err)
		}
		path := filepath.Join(dir, "clone.gitconfig")
		err := withCloneTrust(path, root, 0600, func(config string) error {
			scoped := append([]string{}, env...)
			for i := range scoped {
				if strings.HasPrefix(scoped[i], "GIT_CONFIG_GLOBAL=") {
					scoped[i] = "GIT_CONFIG_GLOBAL=" + config
				}
			}
			clone(scoped, "private-scope")
			b, err := os.ReadFile(record)
			if err != nil || string(b) != root+"\n"+root+"/.git\n" {
				t.Fatalf("exact trust missing in upload-pack: %q %v", b, err)
			}
			siblingEnv := append(scoped, "GIT_TEST_ASSUME_DIFFERENT_OWNER=1")
			if code, out := run(siblingEnv, "-C", filepath.Join(dir, "sibling"), "status", "--porcelain"); code == 0 || !strings.Contains(out, "dubious ownership") {
				t.Fatalf("sibling trusted: %d %s", code, out)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("config survived: %v", err)
		}
		a, source := run(env, "-C", root, "rev-parse", "HEAD", "HEAD^{tree}")
		b, target := run(env, "-C", filepath.Join(dir, "private-scope"), "rev-parse", "HEAD", "HEAD^{tree}")
		if a != 0 || b != 0 || source != target {
			t.Fatal("clone identity changed")
		}
		if err := cloneRepository(context.Background(), options{root: root, runtime: dir}, filepath.Join(dir, "production")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("production clone retained trust")
		}
		code, production := run(env, "-C", filepath.Join(dir, "production"), "rev-parse", "HEAD", "HEAD^{tree}")
		if code != 0 || production != source {
			t.Fatal("production clone identity changed")
		}

	})
}

func TestCloneTrustEncodingAndLifetime(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "clone.gitconfig")
		root := filepath.Join(dir, `space #;"\source`)
		err := withCloneTrust(path, root, 0600, func(config string) error {
			var b bytes.Buffer
			code, e := command(context.Background(), dir, []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}, &b, &b, time.Second, "git", "config", "--file", config, "--get-all", "safe.directory")
			if e != nil || code != 0 || b.String() != root+"\n"+root+"/.git\n" {
				t.Fatalf("unsafe encoding: %d %v %q", code, e, b.String())
			}
			interruptionLeavesNoLiveDescendant(t)
			return context.Canceled
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("cancelled config survived")
		}
		for _, s := range []string{"newline\n[include]", "tab\t", "nul\x00", "cr\r"} {
			if err = withCloneTrust(path, s, 0600, func(string) error { t.Fatal("control admitted"); return nil }); err == nil {
				t.Fatal("control accepted")
			}
		}
		if err = os.WriteFile(path, []byte("preserved"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = withCloneTrust(path, root, 0600, func(string) error { return nil }); err == nil {
			t.Fatal("existing file replaced")
		}
		if b, _ := os.ReadFile(path); string(b) != "preserved" {
			t.Fatal("existing bytes changed")
		}
		os.Remove(path)
		if err = os.Symlink(filepath.Join(dir, "absent"), path); err != nil {
			t.Fatal(err)
		}
		if err = withCloneTrust(path, root, 0600, func(string) error { return nil }); err == nil {
			t.Fatal("symlink accepted")
		}
		os.Remove(path)
		err = withCloneTrust(path, root, 0600, func(config string) error {
			if err := os.Remove(config); err != nil {
				return err
			}
			if err := os.Mkdir(config, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(config, "residue"), nil, 0600)
		})
		if err == nil {
			t.Fatal("cleanup refusal passed")
		}
		for _, e := range closedEnv(options{runtime: dir}) {
			if strings.HasPrefix(e, "GIT_CONFIG_GLOBAL=") && e != "GIT_CONFIG_GLOBAL="+os.DevNull {
				t.Fatal("normal environment widened")
			}
		}
	})
}

func TestContainerCloneTrust(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		want := []string{"exec", "-e", "GIT_CONFIG_GLOBAL=/profile/clone.gitconfig", "owned", "/usr/bin/git", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "clone", "--no-hardlinks", "--no-checkout", "/input", containerCheckout}
		if !equal(containerCloneArgs("owned"), want) {
			t.Fatal("container clone scope changed")
		}
	})
}
