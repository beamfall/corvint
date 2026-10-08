//go:build darwin || linux

package testrunner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// Execute runs an already re-derived profile invocation. Caller must bind its
// admitted request and exact invocation digest before calling; parsers never
// call Execute. The declared input inventory is not full dependency attestation.
func Execute(ctx context.Context, r Request, inv Invocation) (out Execution, retErr error) {
	out = Execution{Profile: "corvint-test-runner-execution/0", Runner: r.Runner, InputSha256: Identity(r.InputFiles), InvocationSha256: Identity(inv), ReportSha256: map[string]string{}, ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"}
	out.Input = Input{Target: r.Target, SourceRoot: r.Root, Selectors: append([]string{}, r.Selectors...), SourceFile: r.Project, Runner: r.Runner, Reports: map[string][]byte{}, Expected: append([]string{}, r.ExpectedTests...), OutcomeNeutralExitCodes: inv.OutcomeNeutralExitCodes, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes, ExitCode: -1}
	if r.ExpectedSelection != nil {
		s := *r.ExpectedSelection
		s.Tests = append([]string{}, s.Tests...)
		out.Input.ExpectedSelection = &s
	}
	defer func() {
		if retErr != nil {
			detail := retErr.Error()
			if len(detail) > 1024 {
				detail = detail[:1024]
			}
			out.Input.ExecutionProblems = append(out.Input.ExecutionProblems, Problem{"execution-boundary", detail})
		}
	}()
	if err := exitProfile(inv.SuccessExitCodes, inv.FailureExitCodes, inv.OutcomeNeutralExitCodes); err != nil {
		return out, err
	}
	if inv.RetireDetachedDescendants {
		if inv.GracefulInterrupt {
			return out, fmt.Errorf("detached retirement excludes graceful interrupt")
		}
		if !groupreap.RetirementSupported {
			return out, fmt.Errorf("detached descendant retirement unsupported on this platform")
		}
		out.Retirement = &groupreap.Retirement{Retired: []groupreap.RetiredProcess{}, Unretired: []groupreap.RetiredProcess{}, Problems: []string{}}
	}
	if !filepath.IsAbs(r.Root) || !filepath.IsAbs(r.ReportDir) || r.TimeoutSeconds < 1 || r.TimeoutSeconds > 1800 || len(r.InputFiles) == 0 || len(r.InputFiles) > 4096 {
		return out, fmt.Errorf("incomplete execution admission")
	}
	if err := checkReportInventory(r, inv); err != nil {
		return out, err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return out, err
	}
	defer root.Close()
	checkBindings := func() error {
		if e := checkInputs(root, r.InputFiles); e != nil {
			return e
		}
		if e := checkPinnedFile(r.Root, r.Config, r.ConfigSha256); e != nil {
			return fmt.Errorf("config: %w", e)
		}
		if e := checkPinnedFile(r.Root, r.Reporter, r.ReporterSha256); e != nil {
			return fmt.Errorf("reporter: %w", e)
		}
		return checkTools(r)
	}
	if err = checkBindings(); err != nil {
		return out, err
	}
	defer func() {
		if e := checkBindings(); e != nil && retErr == nil {
			retErr = e
		}
	}()
	if err = checkInputs(root, r.InputFiles); err != nil {
		return out, err
	}
	phases := inv.Phases
	if len(phases) == 0 {
		if len(inv.Argv) == 0 {
			return out, fmt.Errorf("empty runner invocation")
		}
		phases = []Phase{{Kind: "TEST", Tool: "primary", Argv: inv.Argv, Environment: inv.Environment}}
	} else if len(inv.Argv) != 0 {
		return out, fmt.Errorf("ambiguous invocation phases")
	}
	if len(phases) > 16 {
		return out, fmt.Errorf("phase bound")
	}
	tests := 0
	outputs := map[string]bool{}
	for _, p := range phases {
		if p.Kind != "BUILD" && p.Kind != "DISCOVER" && p.Kind != "TEST" && p.Kind != "DECODE" {
			return out, fmt.Errorf("unknown phase")
		}
		if p.Kind == "TEST" {
			tests++
		}
		if p.StdoutReport != "" {
			if err = relative(p.StdoutReport); err != nil {
				return out, err
			}
			if outputs[p.StdoutReport] {
				return out, fmt.Errorf("duplicate stdout artifact")
			}
			outputs[p.StdoutReport] = true
		}
		t, ok := tool(r, p.Tool)
		if !ok {
			return out, fmt.Errorf("undeclared phase tool")
		}
		if err = checkTool(t); err != nil {
			return out, err
		}
		// A native test binary may need no arguments. Require an explicit
		// primary TEST phase; an empty implicit invocation remains invalid.
		if len(p.Argv) > 256 || (len(p.Argv) == 0 && (p.Kind != "TEST" || p.Tool != "primary")) {
			return out, fmt.Errorf("argv bound")
		}
		for _, a := range p.Argv {
			if len(a) > 4096 || strings.ContainsRune(a, 0) {
				return out, fmt.Errorf("invalid argv")
			}
		}
	}
	if tests != 1 {
		return out, fmt.Errorf("exactly one test phase required")
	}
	if len(inv.Files) > 32 {
		return out, fmt.Errorf("template count bound")
	}
	total := 0
	for n, b := range inv.Files {
		if err = relative(n); err != nil {
			return out, err
		}
		if outputs[n] {
			return out, fmt.Errorf("template/output alias")
		}
		outputs[n] = true
		total += len(b)
	}
	if total > 1<<20 {
		return out, fmt.Errorf("template byte bound")
	}
	if err = os.Mkdir(r.ReportDir, 0700); err != nil {
		return out, fmt.Errorf("report directory must be fresh: %w", err)
	}
	for n, b := range inv.Files {
		p := filepath.Join(r.ReportDir, filepath.FromSlash(n))
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return out, err
		}
		if err = os.WriteFile(p, b, 0600); err != nil {
			return out, err
		}
	}
	for _, n := range []string{".home", ".tmp"} {
		if err = os.Mkdir(filepath.Join(r.ReportDir, n), 0700); err != nil {
			return out, err
		}
	}
	reportRoot, err := os.OpenRoot(r.ReportDir)
	if err != nil {
		return out, err
	}
	defer reportRoot.Close()
	templateDigests := map[string]string{}
	for name, data := range inv.Files {
		templateDigests[name] = Digest(data)
	}
	checkTemplates := func() error { return checkInputs(reportRoot, templateDigests) }
	defer func() {
		if e := checkTemplates(); e != nil && retErr == nil {
			retErr = fmt.Errorf("fixed template changed: %w", e)
		}
	}()
	limited, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutSeconds)*time.Second)
	defer cancel()
	for i, p := range phases {
		t, _ := tool(r, p.Tool)
		if err = checkBindings(); err != nil {
			return out, err
		}
		if err = checkTemplates(); err != nil {
			return out, err
		}
		phaseCtx, phaseCancel := context.WithCancel(limited)
		cmd := exec.CommandContext(phaseCtx, t.Executable, p.Argv...)
		cmd.Dir = r.Root
		env := map[string]string{"PATH": declaredPath(r), "HOME": filepath.Join(r.ReportDir, ".home"), "TMPDIR": ExecutionTempDir(r.ReportDir), "LANG": "C.UTF-8", "TZ": "UTC"}
		for k, v := range p.Environment {
			if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
				phaseCancel()
				return out, fmt.Errorf("invalid environment")
			}
			if k == "HOME" || k == "TMPDIR" || k == "PATH" || k == groupreap.OwnerEnvironmentKey {
				phaseCancel()
				return out, fmt.Errorf("reserved execution environment")
			}
			env[k] = v
		}
		var retirer *groupreap.Retirer
		if inv.RetireDetachedDescendants {
			if retirer, err = groupreap.NewRetirer(); err != nil {
				phaseCancel()
				return out, err
			}
			env[groupreap.OwnerEnvironmentKey] = retirer.Token()
		}
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			cmd.Env = append(cmd.Env, k+"="+env[k])
		}
		containPhase(cmd, inv.GracefulInterrupt, retirer)
		stdout, stderr := &limitedBuffer{cancel: phaseCancel}, &limitedBuffer{cancel: phaseCancel}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		runErr := groupreap.RunRetiring(cmd, retirer)
		var retireErr error
		if retirer != nil {
			// Retain the record before any fallible post-processing, so a
			// later binding or write failure cannot discard it (TRE-V0-030).
			got := retirer.Result()
			out.Retirement.Merge(got)
			if !got.Clean() {
				retireErr = fmt.Errorf("detached descendant retirement incomplete: %d unretired, %d problems", len(got.Unretired), len(got.Problems))
			}
		}
		a, ovA := stdout.value()
		b, ovB := stderr.value()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		result := PhaseResult{p.Kind, t.Sha256, Identity(p.Argv), Digest(a), Digest(b), code, limited.Err() == context.DeadlineExceeded, ctx.Err() != nil, ovA || ovB}
		phaseCancel()
		out.Phases = append(out.Phases, result)
		for suffix, data := range map[string][]byte{"stdout": a, "stderr": b} {
			if err = writeArtifact(reportRoot, fmt.Sprintf(".phase-%02d-%s", i, suffix), data); err != nil {
				return out, err
			}
		}
		if p.StdoutReport != "" {
			if err = writeArtifact(reportRoot, p.StdoutReport, a); err != nil {
				return out, err
			}
		}
		if err = checkBindings(); err != nil {
			return out, err
		}
		if err = checkTemplates(); err != nil {
			return out, err
		}

		if p.Kind == "TEST" {
			out.Input.Stdout = a
			out.Input.Stderr = b
			out.Input.ExitCode = code
			out.Input.TimedOut = result.TimedOut
			out.Input.Interrupted = result.Interrupted
			out.Input.Overflow = result.Overflow
		}
		if retireErr != nil {
			// A cleanup failure is an execution problem, so no complete or
			// passing observation can hide it (TRE-V0-030).
			return out, retireErr
		}
		if result.TimedOut || result.Interrupted || result.Overflow || code < 0 || (p.Kind != "TEST" && runErr != nil) {
			out.Input.TimedOut = result.TimedOut
			out.Input.Interrupted = result.Interrupted
			out.Input.Overflow = result.Overflow
			if p.Kind != "TEST" {
				out.Input.ExitCode = -1
			}
			break
		}
	}
	names := append([]string{}, inv.ReportPaths...)
	names = append(names, r.ReportFiles...)
	scanned := 0
	for _, pat := range inv.ReportPatterns {
		matches, e := boundedReportGlob(limited, reportRoot, pat, &scanned)
		if e != nil {
			return out, e
		}
		if len(matches) == 0 {
			return out, fmt.Errorf("report pattern has no matches")
		}
		names = append(names, matches...)
	}

	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			// Cross-manifest and glob overlap name the same artifact; duplicates
			// within an explicit inventory were rejected before execution.
			continue
		}
		seen[n] = true
		if len(seen) > MaxReports {
			return out, fmt.Errorf("report count bound")
		}
		if err = relative(n); err != nil {
			return out, err
		}
		if err = regularPath(reportRoot, n); err != nil {
			return out, err
		}
		b, e := testvaliditydoc.ReadFileBounded(reportRoot, n, MaxReportBytes)
		if e != nil {
			return out, e
		}
		out.Input.Reports[n] = b
		out.ReportSha256[n] = Digest(b)
	}
	if err = checkInputs(root, r.InputFiles); err != nil {
		return out, err
	}
	return out, nil
}
func tool(r Request, name string) (Tool, bool) {
	if name == "" || name == "primary" {
		return Tool{r.Executable, r.ExecutableSha256}, true
	}
	v, ok := r.Tools[name]
	return v, ok
}
func checkTool(t Tool) error {
	if !filepath.IsAbs(t.Executable) || len(t.Sha256) != 64 {
		return fmt.Errorf("tool identity incomplete")
	}
	s, e := os.Lstat(t.Executable)
	if e != nil || !s.Mode().IsRegular() || s.Size() > 256<<20 {
		return fmt.Errorf("tool must be bounded regular file")
	}
	f, _, e := openCheckedRegular(t.Executable, s)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (256<<20)+1))
	if e != nil || len(b) > 256<<20 || Digest(b) != t.Sha256 {
		return fmt.Errorf("tool identity changed")
	}
	return nil
}
func regularPath(root *os.Root, n string) error {
	parts := strings.Split(n, "/")
	for i := range parts {
		s, e := root.Lstat(strings.Join(parts[:i+1], "/"))
		if e != nil {
			return e
		}
		if s.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink input refused")
		}
		if i < len(parts)-1 && !s.IsDir() {
			return fmt.Errorf("non-directory segment")
		}
		if i == len(parts)-1 && !s.Mode().IsRegular() {
			return fmt.Errorf("nonregular file refused")
		}
	}
	return nil
}
func checkInputs(root *os.Root, files map[string]string) error {
	for n, h := range files {
		if e := relative(n); e != nil {
			return e
		}
		if e := regularPath(root, n); e != nil {
			return e
		}
		b, e := testvaliditydoc.ReadFileBounded(root, n, MaxReportBytes)
		if e != nil || Digest(b) != h {
			return fmt.Errorf("declared source identity changed")
		}
	}
	return nil
}

type limitedBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	overflow bool
	cancel   context.CancelFunc
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	room := MaxReportBytes - b.b.Len()
	if len(p) > room {
		b.overflow = true
		b.cancel()
		p = p[:room]
	}
	_, _ = b.b.Write(p)
	return n, nil
}
func (b *limitedBuffer) value() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte{}, b.b.Bytes()...), b.overflow
}

// Every tool in the admitted dependency inventory is bound, including helpers
// discovered through PATH by the primary runner. No host PATH is inherited.
func checkTools(r Request) error {
	if len(r.Tools) > 32 {
		return fmt.Errorf("tool inventory bound")
	}
	if err := checkTool(Tool{r.Executable, r.ExecutableSha256}); err != nil {
		return err
	}
	for name, t := range r.Tools {
		if name == "" || name == "primary" || len(name) > 128 {
			return fmt.Errorf("invalid declared tool name")
		}
		if err := checkTool(t); err != nil {
			return fmt.Errorf("declared tool %s: %w", name, err)
		}
	}
	return nil
}
func declaredPath(r Request) string {
	dirs := []string{filepath.Dir(r.Executable)}
	seen := map[string]bool{dirs[0]: true}
	names := make([]string, 0, len(r.Tools))
	for name := range r.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dir := filepath.Dir(r.Tools[name].Executable)
		if !seen[dir] {
			dirs = append(dirs, dir)
			seen[dir] = true
		}
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// openPinnedRegular binds the opened descriptor to a no-follow Lstat. The open
// goes through the absolute path with O_NOFOLLOW because os.Root follows a final
// symlink even with that flag (V1-0624).
func openPinnedRegular(root *os.Root, rel string) (*os.File, os.FileInfo, error) {
	before, err := root.Lstat(rel)
	if err != nil {
		return nil, nil, err
	}
	return openCheckedRegular(filepath.Join(root.Name(), rel), before)
}

// openCheckedRegular opens a path whose no-follow check saw before. O_NONBLOCK
// keeps a FIFO swapped in after that check from blocking admission, O_NOFOLLOW
// refuses a final symlink swapped in, and the same-file check refuses any other
// replacement (V1-0624).
func openCheckedRegular(name string, before os.FileInfo) (*os.File, os.FileInfo, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, nil, fmt.Errorf("nonregular file refused")
	}
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || !before.Mode().IsRegular() || !os.SameFile(before, st) {
		f.Close()
		return nil, nil, fmt.Errorf("nonregular file refused")
	}
	return f, st, nil
}
func checkPinnedFile(base, name, digest string) error {
	if name == "" && digest == "" {
		return nil
	}
	if name == "" || len(digest) != 64 {
		return fmt.Errorf("incomplete pinned file identity")
	}
	if !filepath.IsAbs(name) {
		if err := relative(name); err != nil {
			return err
		}
		name = filepath.Join(base, filepath.FromSlash(name))
	}
	root, err := os.OpenRoot(string(filepath.Separator))
	if err != nil {
		return err
	}
	defer root.Close()
	rel := strings.TrimPrefix(filepath.Clean(name), string(filepath.Separator))
	if err = relative(rel); err != nil {
		return err
	}
	if err = regularPath(root, rel); err != nil {
		return err
	}
	f, st, err := openPinnedRegular(root, rel)
	if err != nil {
		return err
	}
	defer f.Close()
	if st.Size() > MaxPinnedArtifactBytes {
		return fmt.Errorf("pinned artifact byte bound")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, MaxPinnedArtifactBytes+1))
	if err != nil {
		return err
	}
	if n > MaxPinnedArtifactBytes || hex.EncodeToString(h.Sum(nil)) != digest {
		return fmt.Errorf("pinned artifact binding changed")
	}
	return nil
}

// Use a rooted exclusive write so a runner-created link cannot redirect an
// executor-owned log or decoded report outside the fresh artifact directory.
func writeArtifact(root *os.Root, name string, data []byte) error {
	if err := relative(name); err != nil {
		return err
	}
	parent := filepath.Dir(filepath.FromSlash(name))
	if parent != "." {
		if err := root.MkdirAll(parent, 0700); err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(parent), "/")
		for i := range parts {
			st, err := root.Lstat(strings.Join(parts[:i+1], "/"))
			if err != nil {
				return err
			}
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("non-directory artifact parent")
			}
		}
	}
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func checkReportInventory(r Request, inv Invocation) error {
	for _, names := range [][]string{r.ReportFiles, inv.ReportPaths, inv.ReportPatterns} {
		if len(names) > MaxReports {
			return fmt.Errorf("report inventory bound")
		}
		seen := map[string]bool{}
		for _, name := range names {
			if relative(name) != nil || seen[name] {
				return fmt.Errorf("invalid or duplicate explicit report inventory")
			}
			seen[name] = true
		}
	}
	for _, pat := range inv.ReportPatterns {
		if strings.Contains(pat, "**") || strings.ContainsAny(filepath.Dir(pat), "*?[") {
			return fmt.Errorf("report patterns require literal parent directories")
		}
		if _, err := filepath.Match(filepath.Base(pat), ""); err != nil {
			return err
		}
	}
	return nil
}

// Enumerate only literal rooted directories, incrementally and under the same
// deadline as execution. The scan cap includes nonmatching entries and repeated
// scans, so adversarial directories cannot allocate or traverse without bound.
func boundedReportGlob(ctx context.Context, root *os.Root, pattern string, scanned *int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent, leaf := filepath.Dir(pattern), filepath.Base(pattern)
	if parent != "." {
		parts := strings.Split(filepath.ToSlash(parent), "/")
		for i := range parts {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			st, err := root.Lstat(strings.Join(parts[:i+1], "/"))
			if err != nil {
				return nil, err
			}
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("report pattern parent is not a literal directory")
			}
		}
	}
	dir, err := root.Open(parent)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	var matches []string
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, readErr := dir.ReadDir(64)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			*scanned++
			if *scanned > MaxTests {
				return nil, fmt.Errorf("report directory scan bound")
			}
			match, err := filepath.Match(leaf, entry.Name())
			if err != nil {
				return nil, err
			}
			if match {
				name := filepath.ToSlash(filepath.Join(parent, entry.Name()))
				if err := regularPath(root, name); err != nil {
					return nil, err
				}
				matches = append(matches, name)
				if len(matches) > MaxReports {
					return nil, fmt.Errorf("report match bound")
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// containPhase owns a phase's process group, chooses its cancellation signal
// and bounds its pipe drain.
func containPhase(cmd *exec.Cmd, graceful bool, retirer *groupreap.Retirer) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Playwright owns detached browser groups. Interrupt its leader so native
	// worker teardown can close them before the bounded hard-kill fallback.
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			if graceful {
				return cmd.Process.Signal(syscall.SIGINT)
			}
			if retirer != nil {
				// Stop and retire owned detached descendants while the
				// leader still proves their ancestry, then kill its group.
				retirer.Retire()
			}
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	// Without a graceful interrupt the bound detects a descendant holding the
	// output pipes, not a slow reader (V1-0391). With one it is also the grace
	// before os/exec kills an interrupted leader, so it stays short.
	cmd.WaitDelay = time.Minute
	if graceful {
		cmd.WaitDelay = 5 * time.Second
	}
}
