package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Clone's upload-pack child discards command-scope Git config. A private
// global-scope file carries exact trust across that child boundary only.
func withCloneTrust(path, root string, mode os.FileMode, clone func(string) error) (err error) {
	for _, r := range root {
		if unicode.IsControl(r) {
			return errors.New("control character in clone source")
		}
	}
	quote := func(s string) string { return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"` }
	data := "[safe]\n\tdirectory = " + quote(root) + "\n\tdirectory = " + quote(strings.TrimRight(root, "/")+"/.git") + "\n"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(path)) }()
	_, writeErr := f.WriteString(data)
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	if err = os.Chmod(path, mode); err != nil {
		return err
	}
	return clone(path)
}

func cloneRepository(ctx context.Context, o options, checkout string) error {
	return withCloneTrust(filepath.Join(runtimePath(o), "clone.gitconfig"), o.root, 0600, func(config string) error {
		env := closedEnv(o)
		for i := range env {
			if strings.HasPrefix(env[i], "GIT_CONFIG_GLOBAL=") {
				env[i] = "GIT_CONFIG_GLOBAL=" + config
			}
		}
		stdout := &boundedBuffer{limit: 8 << 20}
		stderr := &boundedBuffer{limit: 1 << 20}
		code, err := command(ctx, o.root, env, stdout, stderr, 2*time.Minute, "git", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "clone", "--no-hardlinks", "--no-checkout", "--", o.root, checkout)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("clone exited %d: %s", code, stderr.String())
		}
		return nil
	})
}

// These paths stay POSIX when the launcher itself runs on Windows.
func containerCloneArgs(id string) []string {
	return []string{"exec", "-e", "GIT_CONFIG_GLOBAL=/profile/clone.gitconfig", id, "/usr/bin/git", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "clone", "--no-hardlinks", "--no-checkout", "/input", containerCheckout}
}
