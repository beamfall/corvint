package testacceptance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/procgroup"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var oidPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var hashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func Hash(data []byte) string { h := sha256.Sum256(data); return "sha256:" + hex.EncodeToString(h[:]) }
func Digest(r Request) string { b, _ := json.Marshal(r); return Hash(b) }

// Decode refuses duplicate keys as well as unknown fields and oversized input.
func Decode(data []byte, out any) error {
	if len(data) > InputLimit {
		return errors.New("input-bound")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("input-trailing")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("input-shape")
	}
	return nil
}
func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("input-depth-bound")
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	if k, ok := t.(json.Delim); ok {
		switch k {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				t, e = d.Token()
				if e != nil {
					return e
				}
				s, ok := t.(string)
				if !ok || seen[s] {
					return errors.New("duplicate-key")
				}
				seen[s] = true
				if e = uniqueValue(d, depth+1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = uniqueValue(d, depth+1); e != nil {
					return e
				}
			}
		default:
			return errors.New("input-shape")
		}
		_, e = d.Token()
		return e
	}
	return nil
}
func cleanAbsolute(path string) bool { return filepath.IsAbs(path) && filepath.Clean(path) == path }
func regular(path string) error {
	if !cleanAbsolute(path) {
		return errors.New("path-not-absolute")
	}
	for p := path; p != "/"; p = filepath.Dir(p) {
		s, e := os.Lstat(p)
		if e != nil || s.Mode()&os.ModeSymlink != 0 {
			return errors.New("path-symlink-or-absent")
		}
	}
	s, e := os.Stat(path)
	if e != nil || !s.Mode().IsRegular() || s.Size() > 128<<20 {
		return errors.New("file-not-bounded-regular")
	}
	return nil
}
func within(path, root string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func git(root string, args ...string) (string, error) {
	exe, e := exec.LookPath("git")
	if e != nil {
		return "", e
	}
	argv := append([]string{exe, "-C", root}, args...)
	o := procgroup.Run(context.Background(), procgroup.Spec{Argv: argv, Dir: root, Env: []string{"PATH=/opt/homebrew/bin:/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0"}, Timeout: 10 * time.Second, OutputLimit: 1 << 20})
	if o.Err != nil || !o.Started || !o.WaitCompleted || o.ExitStatus != 0 || o.OutputOverflow || !o.OwnedProcessGroupCleanup {
		return "", errors.New("git-observation-failed")
	}
	return strings.TrimSpace(string(o.Stdout)), nil
}
func verifyRepo(r Repository) error {
	if !cleanAbsolute(r.Root) || !oidPattern.MatchString(r.Commit) || !oidPattern.MatchString(r.Tree) {
		return errors.New("repository-pin-invalid")
	}
	top, e := git(r.Root, "rev-parse", "--show-toplevel")
	if e != nil || top != r.Root {
		return errors.New("repository-root-mismatch")
	}
	head, e := git(r.Root, "rev-parse", "HEAD")
	if e != nil || head != r.Commit {
		return errors.New("repository-commit-drift")
	}
	tree, e := git(r.Root, "rev-parse", "HEAD^{tree}")
	if e != nil || tree != r.Tree {
		return errors.New("repository-tree-drift")
	}
	status, e := git(r.Root, "status", "--porcelain=v1", "--untracked-files=all")
	if e != nil || status != "" {
		return errors.New("repository-not-clean")
	}
	return nil
}
func Validate(r Request) error {
	if (r.Schema != RequestSchema && r.Schema != FreshRequestSchema) || (r.Schema == RequestSchema && r.Freshness != nil) || (r.Schema == FreshRequestSchema && r.Freshness == nil) || !idPattern.MatchString(r.Environment) || r.Repeat < 2 || r.Repeat > 100 || r.TimeoutSeconds < 1 || r.TimeoutSeconds > 60 || len(r.Tests) < 1 || len(r.Tests) > 32 || len(r.Inputs) > 128 {
		return errors.New("request-bound")
	}
	if e := verifyRepo(r.Product); e != nil {
		return e
	}
	if e := verifyRepo(r.TestRepository); e != nil {
		return e
	}
	seen := map[string]bool{}
	files := map[string]bool{}
	for _, f := range r.Inputs {
		if seen[f.Path] || !hashPattern.MatchString(f.SHA256) || regular(f.Path) != nil {
			return errors.New("input-pin-invalid")
		}
		seen[f.Path] = true
		root := r.TestRepository.Root
		if !within(f.Path, root) {
			root = r.Product.Root
		}
		if !within(f.Path, root) {
			return errors.New("input-outside-roots")
		}
		rel, _ := filepath.Rel(root, f.Path)
		if _, e := git(root, "ls-files", "--error-unmatch", "--", rel); e != nil {
			return errors.New("input-not-tracked")
		}
		b, e := os.ReadFile(f.Path)
		if e != nil || Hash(b) != f.SHA256 {
			return errors.New("input-byte-drift")
		}
	}
	for _, p := range []string{r.Config, r.Package, r.Lockfile} {
		if !seen[p] || !within(p, r.TestRepository.Root) {
			return errors.New("configuration-not-pinned")
		}
	}
	ids := map[string]bool{}
	for _, t := range r.Tests {
		if !idPattern.MatchString(t.ID) || ids[t.ID] || !seen[t.File] || !within(t.File, r.TestRepository.Root) || t.Line < 1 || t.Line > 1000000 || len(t.Title) == 0 || len(t.Title) > 512 || len(t.Project) > 128 {
			return errors.New("test-identity-invalid")
		}
		ids[t.ID] = true
		key := t.File + "\x00" + t.Title
		if files[key] {
			return errors.New("test-identity-duplicate")
		}
		files[key] = true
		if (t.Control == nil) != (t.Tool == nil) || (t.Control == nil) != (t.ControlCommand == nil) {
			return errors.New("control-tool-required")
		}
		if t.Control != nil {
			if len(t.ControlCommand.Argv) != 1 || regular(t.ControlCommand.Argv[0]) != nil {
				return errors.New("control-command-invalid")
			}
			cb, ce := os.ReadFile(t.ControlCommand.Argv[0])
			if ce != nil || Hash(cb) != t.ControlCommand.ExecutableSHA256 || t.Tool.Executable != t.ControlCommand.ExecutableSHA256 {
				return errors.New("control-tool-byte-mismatch")
			}
			p := t.Control
			target := p.Request.Target
			if p.Digest == "" || p.Schema != behaviorfalsify.PlanSchema || target.TestID != t.ID || filepath.Join(r.TestRepository.Root, target.TestFile) != t.File || target.TestLine != t.Line || target.TestTitle != t.Title || target.Project != t.Project || target.TestRevision != r.TestRepository.Commit || target.ApplicationRevision != r.Product.Commit || filepath.Join(r.TestRepository.Root, p.Request.Runner.ConfigFile) != r.Config || p.Request.Runner.RunnerVersion != r.RunnerVersion || p.Request.Repositories.Test != r.TestRepository.Root || p.Request.Repositories.Application != r.Product.Root {
				return errors.New("control-binding-mismatch")
			}
			for _, k := range p.Request.DeclaredEnvKeys {
				if k != "LANG" && k != "LC_ALL" {
					return errors.New("control-environment-not-safe")
				}
			}
			if p.Request.TimeoutSeconds > 60 || p.Request.WallClockSeconds > 120 || p.Request.Attempts > 2 || len(p.Controls) > 16 {
				return errors.New("control-bound")
			}
		}
	}
	for _, c := range []Command{r.Runner, r.Server} {
		if len(c.Argv) < 1 || len(c.Argv) > 16 || !hashPattern.MatchString(c.ExecutableSHA256) || regular(c.Argv[0]) != nil {
			return errors.New("command-invalid")
		}
		b, e := os.ReadFile(c.Argv[0])
		if e != nil || Hash(b) != c.ExecutableSHA256 {
			return errors.New("executable-drift")
		}
		for _, arg := range c.Argv {
			if len(arg) > 2048 || strings.ContainsAny(arg, "\x00\n\r") {
				return errors.New("argv-invalid")
			}
		}
	}
	if len(r.Server.Argv) > 1 && (!seen[r.Server.Argv[1]] || !within(r.Server.Argv[1], r.Product.Root)) {
		return errors.New("server-entrypoint-not-pinned")
	}
	// Runner prefix is an approved executable plus its independently approved CLI
	// entrypoint. Generated test arguments cannot be overridden by a prefix flag.
	if len(r.Runner.Argv) > 2 {
		return errors.New("runner-prefix-invalid")
	}
	if len(r.Runner.Argv) == 2 {
		entry, entryErr := os.ReadFile(r.Runner.Argv[1])
		if entryErr != nil || Hash(entry) != r.Runner.EntrypointSHA256 || regular(r.Runner.Argv[1]) != nil {
			return errors.New("runner-entrypoint-invalid")
		}
	}
	u, e := url.Parse(r.ReadyURL)
	if e != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		return errors.New("ready-url-not-loopback")
	}
	if r.Schema == RequestSchema && (!cleanAbsolute(r.AppBuildDir) || !within(r.AppBuildDir, r.Product.Root)) {
		return errors.New("build-root-invalid")
	}
	if r.Schema == FreshRequestSchema {
		if err := validateFreshnessOptions(r); err != nil {
			return err
		}
	}
	if len(r.RunnerVersion) > 64 || r.RunnerVersion == "" {
		return errors.New("runner-version-invalid")
	}
	return nil
}
func approval(r Request, approved string) error {
	if !hashPattern.MatchString(approved) || Digest(r) != approved {
		return fmt.Errorf("request-approval-mismatch")
	}
	return Validate(r)
}
