package main

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const containerManifest = "sha256:eef6a67266eeed3c86dd47fd01b32faa8bf0229eb83eb3d4d466e80391bd3820"
const containerImage = "docker.io/library/golang@" + containerManifest
const containerConfig = "sha256:ccb6f18cbf10486608b5fea50834a621b9fada5dd869841f08c486ba307155a3"
const containerCheckout = "/work/checkout"
const workTmpfs = "size=15032385536,uid=65532,gid=65532,mode=0700,nosuid,nodev,exec"
const memoryBytes int64 = 8 << 30

// Only the trusted launcher writes this file, mounted read-only outside PR source.
// Per-run container IDs and host/kernel observations belong in inspect.json, not
// the portable qualification identity.
type containerProfile struct {
	Version, Image, Config, Platform, Cache string
	CPUs, Memory, Swap, WorkBytes           int64
	Source                                  string
	Tools                                   map[string]string
}

func (p containerProfile) validate() error {
	if p.Version != "corvint-pr-container/1" || p.Image != containerImage || p.Config != containerConfig || p.Platform != "linux/amd64" || p.Cache != "empty-immediately-before-test/1" || p.CPUs != 2 || p.Memory != memoryBytes || p.Swap != memoryBytes || p.WorkBytes != 14<<30 || !oid.MatchString(p.Source) || len(p.Tools) != 3 {
		return errors.New("unrecognized container profile")
	}
	for _, name := range trustedNames {
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(p.Tools[name]) {
			return errors.New("missing trusted binary digest")
		}
	}
	return nil
}

var trustedNames = []string{"corvint", "gate-affected-select", "corvint-pr-tests"}
var containerID = regexp.MustCompile(`^[0-9a-f]{64}$`)

type containerInspection struct {
	ID, Image string
	Config    struct {
		User            string
		Image           string
		Entrypoint, Cmd []string
		Volumes         map[string]json.RawMessage
	}
	HostConfig struct {
		LogConfig struct {
			Type   string
			Config map[string]string
		}
		NetworkMode                           string
		ReadonlyRootfs, Privileged            bool
		NanoCpus, Memory, MemorySwap, ShmSize int64
		CapDrop, CapAdd, SecurityOpt          []string
		Tmpfs                                 map[string]string
		PortBindings                          map[string]json.RawMessage
	}
	Mounts []struct {
		Type, Source, Destination string
		RW                        bool
	}
}

func inspectProfile(b []byte, mounts map[string]string) error {
	var all []containerInspection
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	if len(all) != 1 {
		return errors.New("expected one container inspection")
	}
	v := all[0]
	h := v.HostConfig
	// Classic/config and containerd/manifest IDs name the same verified image;
	// the requested reference must still be the exact digest-qualified image.
	if !containerID.MatchString(v.ID) || (v.Image != containerConfig && v.Image != containerManifest) || v.Config.Image != containerImage || v.Config.User != "65532:65532" || !equal(v.Config.Entrypoint, []string{"/bin/sleep"}) || !equal(v.Config.Cmd, []string{"infinity"}) || len(v.Config.Volumes) != 0 || h.LogConfig.Type != "none" || len(h.LogConfig.Config) != 0 || h.NetworkMode != "none" || !h.ReadonlyRootfs || h.Privileged || h.NanoCpus != 2_000_000_000 || h.Memory != memoryBytes || h.MemorySwap != memoryBytes || h.ShmSize != 1<<20 || !equal(h.CapDrop, []string{"ALL"}) || len(h.CapAdd) != 0 || !noNewPrivileges(h.SecurityOpt) || len(h.PortBindings) != 0 || !equal(h.Tmpfs, map[string]string{"/work": workTmpfs}) {
		return errors.New("container isolation inspection mismatch")
	}
	seen := map[string]bool{}
	for _, m := range v.Mounts {
		// Docker also reports tmpfs mounts on some engine versions.
		if m.Type == "tmpfs" && m.Destination == "/work" {
			continue
		}
		if m.Type != "bind" || m.RW || mounts[m.Destination] == "" || m.Source != mounts[m.Destination] || seen[m.Destination] {
			return errors.New("container mount inspection mismatch")
		}
		seen[m.Destination] = true
	}
	if len(seen) != len(mounts) {
		return errors.New("missing readonly mount")
	}
	return nil
}

// Docker may normalize the bare CLI option to its explicit true spelling.
func noNewPrivileges(options []string) bool {
	return len(options) == 1 && (options[0] == "no-new-privileges" || options[0] == "no-new-privileges:true")
}

// Docker runs only under an explicitly named existing context. No shell parsing,
// engine startup, context switching, image pulling, or owner configuration changes.
func dockerCall(ctx context.Context, dockerContext string, stdout io.Writer, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", append([]string{"--context", dockerContext}, args...)...)
	if runtime.GOOS != "windows" {
		attachGroup(cmd)
		defer killGroup(cmd)
	}
	cmd.Stdout = stdout
	stderr := &boundedBuffer{limit: 1 << 20, cancel: cancel}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s: %w: %s", args[0], err, stderr.String())
	}
	return nil
}
func dockerCapture(ctx context.Context, dc string, args ...string) ([]byte, error) {
	return dockerCaptureFor(ctx, dc, 2*time.Minute, args...)
}
func dockerCaptureFor(ctx context.Context, dc string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	b := &boundedBuffer{limit: 8 << 20, cancel: cancel}
	err := dockerCall(ctx, dc, b, args...)
	return b.Bytes(), err
}
func mountArg(source, dest string) (string, error) {
	if strings.ContainsAny(source, ",\r\n") {
		return "", errors.New("unsupported mount path")
	}
	return "type=bind,source=" + source + ",target=" + dest + ",readonly", nil
}
func containerCreateArgs(mounts map[string]string) ([]string, error) {
	args := []string{"create", "--pull=never", "--platform=linux/amd64", "--network=none", "--log-driver=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--cpus=2", "--memory=8589934592", "--memory-swap=8589934592", "--shm-size=1048576", "--tmpfs=/work:" + workTmpfs, "--entrypoint=/bin/sleep"}
	keys := []string{}
	for dest := range mounts {
		keys = append(keys, dest)
	}
	for _, dest := range sorted(keys) {
		m, err := mountArg(mounts[dest], dest)
		if err != nil {
			return nil, err
		}
		args = append(args, "--mount", m)
	}
	return append(args, containerImage, "infinity"), nil
}
func realInput(path string, directory bool) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() != directory || (!directory && !st.Mode().IsRegular()) {
		return "", errors.New("input has wrong type")
	}
	return p, nil
}

func launchContainer(ctx context.Context, o options) (code int, result error) {
	ctx, stop := context.WithTimeout(ctx, 80*time.Minute)
	defer stop()
	if o.dockerContext == "" || o.trusted == "" {
		return 2, errors.New("explicit --docker-context and --trusted required")
	}
	if o.containerMode != "run" && o.containerMode != "freeze" && o.containerMode != "shadow" && o.containerMode != "qualify" {
		return 2, errors.New("unsupported container mode")
	}
	if o.containerMode == "shadow" && (o.row < 1 || o.row > 200 || o.rowsSet) {
		return 2, errors.New("container shadow requires one --row 1..200")
	}
	root, err := realInput(o.root, true)
	if err != nil {
		return 2, err
	}
	// A worktree .git pointer could expose an unrelated host repository. Require a
	// dedicated complete clone, with no borrowed objects, as the read-only snapshot.
	if o.containerMode != "qualify" {
		st, err := os.Lstat(filepath.Join(root, ".git"))
		if err != nil || !st.IsDir() {
			return 2, errors.New("input must be a dedicated full clone")
		}
		if _, err := os.Lstat(filepath.Join(root, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
			return 2, errors.New("borrowed Git objects refused")
		}
	}
	trusted, err := realInput(o.trusted, true)
	if err != nil {
		return 2, err
	}
	if within(root, trusted) {
		return 2, errors.New("trusted tools belong to input")
	}
	out, err := externalPath(root, o.out)
	if err != nil {
		return 2, err
	}
	if err = os.Mkdir(out, 0700); err != nil {
		return 2, fmt.Errorf("new exclusive output required: %w", err)
	}
	if err = realExternalDirectory(root, out); err != nil {
		return 2, err
	}
	profileDir := filepath.Join(out, "profile")
	if err = os.Mkdir(profileDir, 0755); err != nil {
		return 2, err
	}
	p := containerProfile{Version: "corvint-pr-container/1", Image: containerImage, Config: containerConfig, Platform: "linux/amd64", Cache: "empty-immediately-before-test/1", CPUs: 2, Memory: memoryBytes, Swap: memoryBytes, WorkBytes: 14 << 30, Source: o.source, Tools: map[string]string{}}
	for _, name := range trustedNames {
		path, err := realInput(filepath.Join(trusted, name), false)
		if err != nil {
			return 2, err
		}
		if filepath.Dir(path) != trusted {
			return 2, errors.New("trusted binary symlink refused")
		}
		p.Tools[name], err = digestFile(path)
		if err != nil {
			return 2, err
		}
	}
	if err = p.validate(); err != nil {
		return 2, err
	}
	mounts := map[string]string{"/input": root, "/trusted": trusted, "/profile": profileDir}
	if o.containerMode == "shadow" || o.containerMode == "qualify" {
		path, err := realInput(o.corpus, false)
		if err != nil {
			return 2, err
		}
		mounts["/corpus.json"] = path
	}
	if o.containerMode == "qualify" {
		path, err := realInput(o.evidence, true)
		if err != nil {
			return 2, err
		}
		mounts["/rows"] = path
	}
	if o.qualification != "" {
		path, err := realInput(o.qualification, false)
		if err != nil {
			return 2, err
		}
		mounts["/qualification.json"] = path
	}
	args, err := containerCreateArgs(mounts)
	if err != nil {
		return 2, err
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		return 2, err
	}
	name := "corvint-pr-" + hex.EncodeToString(random)
	args = append(args[:1], append([]string{"--name", name}, args[1:]...)...)
	id := name
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, err := dockerCapture(cleanupCtx, o.dockerContext, "rm", "--force", id)
		if err == nil {
			b, e := dockerCapture(cleanupCtx, o.dockerContext, "ps", "--all", "--quiet", "--no-trunc", "--filter", "name=^/"+name+"$")
			if e != nil {
				err = e
			} else if strings.TrimSpace(string(b)) != "" {
				err = errors.New("owned container survived removal")
			}
		}
		witness := map[string]any{"container": id, "removed": err == nil}
		if e := writeJSON(filepath.Join(out, "cleanup.json"), witness); e != nil {
			err = errors.Join(err, e)
		}
		if err != nil {
			code = 2
			result = errors.Join(result, fmt.Errorf("container cleanup: %w", err))
		}
	}()
	createCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	b, err := dockerCapture(createCtx, o.dockerContext, args...)
	cancel()
	if err != nil {
		return 2, err
	}
	id = strings.TrimSpace(string(b))
	if !containerID.MatchString(id) {
		id = name
		return 2, errors.New("invalid created container ID")
	}
	inspect, err := dockerCapture(ctx, o.dockerContext, "inspect", id)
	if err != nil {
		return 2, err
	}
	if err = os.WriteFile(filepath.Join(out, "inspect.json"), inspect, 0600); err != nil {
		return 2, err
	}
	if err = inspectProfile(inspect, mounts); err != nil {
		return 2, err
	}
	if err = writeJSON(filepath.Join(profileDir, "profile.json"), p); err != nil {
		return 2, err
	}
	if err = os.Chmod(filepath.Join(profileDir, "profile.json"), 0644); err != nil {
		return 2, err
	}
	if _, err = dockerCapture(ctx, o.dockerContext, "start", id); err != nil {
		return 2, err
	}
	driverArgs := []string{"exec", id, "/trusted/corvint-pr-tests", "--mode", o.containerMode, "--root", "/input", "--out", "/work/out", "--runtime", "/work/runtime", "--source", o.source, "--planner", "/trusted/corvint", "--selector", "/trusted/gate-affected-select", "--container-profile", "/profile/profile.json"}
	if o.containerMode == "run" {
		if err = withCloneTrust(filepath.Join(profileDir, "clone.gitconfig"), "/input", 0644, func(string) error {
			_, cloneErr := dockerCapture(ctx, o.dockerContext, containerCloneArgs(id)...)
			return cloneErr
		}); err != nil {
			return 2, err
		}
		if _, err = dockerCapture(ctx, o.dockerContext, "exec", id, "/usr/bin/git", "-C", containerCheckout, "-c", "core.hooksPath=/dev/null", "checkout", "--detach", o.target); err != nil {
			return 2, err
		}
		for i := range driverArgs {
			if driverArgs[i] == "/input" {
				driverArgs[i] = containerCheckout
			}
		}
		driverArgs = append(driverArgs, "--base", o.base, "--head", o.head, "--target", o.target)
		if o.qualification != "" {
			driverArgs = append(driverArgs, "--qualification", "/qualification.json", "--qualification-sha256", o.qualificationSHA)
		}
	}
	if o.containerMode == "freeze" {
		driverArgs = append(driverArgs, "--target", o.target)
	}
	if o.containerMode == "shadow" {
		driverArgs = append(driverArgs, "--corpus", "/corpus.json", "--row", fmt.Sprint(o.row))
	}
	if o.containerMode == "qualify" {
		driverArgs = append(driverArgs, "--corpus", "/corpus.json", "--evidence", "/rows")
	}
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Minute)
	_, runErr := dockerCaptureFor(runCtx, o.dockerContext, 75*time.Minute, driverArgs...)
	cancel()
	// Retain bounded diagnostic evidence even when a test or row fails. Interrupted
	// containers are removed immediately; no success or complete-evidence claim.
	if ctx.Err() != nil {
		return 130, ctx.Err()
	}
	folder := "/work/out"
	dest := out
	names := exportFiles(o.containerMode)
	if o.containerMode == "shadow" {
		suffix := fmt.Sprintf("row-%03d", o.row)
		folder += "/" + suffix
		dest = filepath.Join(out, suffix)
		if err = os.Mkdir(dest, 0700); err != nil {
			return 2, err
		}
	}

	exportErrors := map[string]string{}
	for _, name := range names {
		limit := int64(8 << 20)
		if name == "go.json" {
			limit = maxStdout
		}
		if name == "go.stderr" {
			limit = maxStderr
		}
		// Missing optional files are retained in the export error ledger; a successful
		// driver must still produce its terminal record below.
		if err = copyContainerFile(ctx, o.dockerContext, id, folder+"/"+name, filepath.Join(dest, name), limit); err != nil {
			exportErrors[name] = err.Error()
			if name == "plan.json" || name == "selector.txt" || runErr != nil {
				continue
			}
			return 2, err
		}
	}
	if err = writeJSON(filepath.Join(out, "export.json"), exportErrors); err != nil {
		return 2, err
	}
	if o.containerMode == "run" {
		return containerRunResult(o, p, dest, exportErrors, runErr)
	}
	if runErr != nil {
		return 2, runErr
	}
	return 0, nil
}

// Export tmpfs through the container process: Docker cp cannot see this mount.
// Accept exactly one bounded regular file; never
// extract archive paths or follow a PR-controlled symlink onto the host.
func copyContainerFile(ctx context.Context, dc, id, source, dest string, limit int64) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "--context", dc, "exec", id, "/usr/bin/tar", "-C", path.Dir(source), "-cf", "-", "--", path.Base(source))
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr := &boundedBuffer{limit: 1 << 20, cancel: cancel}
	cmd.Stderr = stderr
	if runtime.GOOS != "windows" {
		attachGroup(cmd)
		defer killGroup(cmd)
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		cancel()
		_ = cmd.Wait()
		return err
	}
	err = copyRegularTar(f, pipe, path.Base(source), limit)
	closeErr := f.Close()
	if err != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	err = errors.Join(err, closeErr, waitErr)
	if err != nil {
		err = errors.Join(err, os.Remove(dest))
		return fmt.Errorf("evidence export: %w: %s", err, stderr.String())
	}
	return nil
}
func copyRegularTar(dst io.Writer, src io.Reader, name string, limit int64) error {
	// One header, block alignment, EOF blocks and GNU record padding are bounded
	// together. Read one extra byte to distinguish a full allowance from overflow.
	bounded := &io.LimitedReader{R: src, N: limit + (16 << 10) + 1}
	r := tar.NewReader(bounded)
	h, err := r.Next()
	if err != nil {
		return err
	}
	if h.Typeflag != tar.TypeReg || h.Name != name || h.Size < 0 || h.Size > limit {
		return errors.New("invalid or oversized evidence archive")
	}
	bounded.N -= limit - h.Size
	if bounded.N <= 0 {
		return errors.New("archive framing limit exceeded")
	}
	if _, err = io.CopyN(dst, r, h.Size); err != nil {
		return err
	}
	if _, err = r.Next(); err != io.EOF {
		return errors.New("unexpected extra evidence archive entry")
	}
	var padding [4096]byte
	for {
		n, e := bounded.Read(padding[:])
		for _, b := range padding[:n] {
			if b != 0 {
				return errors.New("nonzero trailing archive bytes")
			}
		}
		if bounded.N <= 0 {
			return errors.New("archive framing limit exceeded")
		}
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
	}
}

func exportFiles(mode string) []string {
	switch mode {
	case "freeze":
		return []string{"corpus.json"}
	case "qualify":
		return []string{"qualification.json"}
	case "run":
		return []string{"plan.json", "selector.txt", "selection.json", "go.json", "go.stderr", "execution.json"}
	default:
		return append(append([]string{}, rowFiles...), "row.json")
	}
}
