package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

const watchInterval = 250 * time.Millisecond
const watchBytes = 8 << 20

func configureWatchHelp(fs *flag.FlagSet) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", fs.Name())
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), `
Foreground watch (experimental trusted-local execution):
  --watch RELPATH       observe a regular file; repeat for each source input
  --foreground --experimental --trusted-local --retain
All four admission flags and package-json, lockfile, config and at least one
nonempty test-file binding are required. The configured bindings are also watched.
Limits: 128 files, 8 MiB total, 250 ms polling/debounce, 60-minute session.
Only in-place saves are supported; replaced/symlinked/missing files or directories
stop the session. Polling is not an atomic filesystem transaction.
Completed original JSON documents stream to stdout and are retained for MCP.
Pending results remain historical; UNKNOWN freshness and unmeasured strength
stay explicit. Watch scheduling does not qualify source-cohort freshness.`)
	}
}

type watchOptions struct{ paths []string }

func splitWatchOptions(args []string) ([]string, *watchOptions, error) {
	var filtered, paths []string
	admitted := map[string]bool{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--" || !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			filtered = append(filtered, args[i:]...)
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(args[i], "-"), "-"), "=")
		switch name {
		case "dir", "config", "package-json", "lockfile", "runner-version", "timeout", "test-file", "env-key", "app-build-dir", "server-ready-url", "server-ready-timeout", "server-arg", "test-arg":
			filtered = append(filtered, args[i])
			if !hasValue && i+1 < len(args) {
				i++
				filtered = append(filtered, args[i])
			}
		case "watch":
			if !hasValue {
				i++
				if i == len(args) {
					return nil, nil, fmt.Errorf("watch requires a relative file")
				}
				value = args[i]
			}
			if value == "" || filepath.IsAbs(value) || !filepath.IsLocal(value) {
				return nil, nil, fmt.Errorf("watch requires a confined relative file")
			}
			paths = append(paths, value)
		case "foreground", "experimental", "trusted-local":
			if admitted[name] || hasValue && value != "true" {
				return nil, nil, fmt.Errorf("watch admission %s must be explicit and true", name)
			}
			admitted[name] = true
		default:
			filtered = append(filtered, args[i])
		}
	}
	if len(paths) == 0 && len(admitted) == 0 {
		return filtered, nil, nil
	}
	if len(paths) == 0 || len(admitted) != 3 {
		return nil, nil, fmt.Errorf("watch requires --watch, --foreground, --experimental and --trusted-local")
	}
	if len(paths) > 128 {
		return nil, nil, fmt.Errorf("watch file count exceeds 128")
	}
	return filtered, &watchOptions{paths: paths}, nil
}

type watchedFile struct {
	path     string
	original os.FileInfo
}
type watchCohort struct {
	root     *os.Root
	files    []watchedFile
	rootPath string
	rootInfo os.FileInfo
	dirPath  string
	dirInfo  os.FileInfo
}

func prepareWatch(cfg jstestprovider.Config, retain bool, opts *watchOptions) (*watchCohort, error) {
	if !retain || cfg.PackageJSON == "" || cfg.Lockfile == "" || cfg.ConfigFile == "" || len(cfg.TestFiles) == 0 {
		return nil, fmt.Errorf("watch requires --retain and package-json, lockfile, config and test-file bindings")
	}
	rootPath, err := enclosingWorktree(cfg.Dir)
	if err != nil {
		return nil, err
	}
	rootInfo, err := os.Lstat(rootPath)
	if err != nil || !rootInfo.IsDir() {
		return nil, fmt.Errorf("watch root is not an unchanged directory")
	}
	dirInfo, err := os.Lstat(cfg.Dir)
	if err != nil || !dirInfo.IsDir() {
		return nil, fmt.Errorf("watch working directory is not a directory")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	openedInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedInfo) {
		root.Close()
		return nil, fmt.Errorf("watch root changed while opening")
	}
	cohort := &watchCohort{root: root, rootPath: rootPath, rootInfo: rootInfo, dirPath: cfg.Dir, dirInfo: dirInfo}
	inputs := append(append([]string{}, opts.paths...), cfg.TestFiles...)
	inputs = append(inputs, cfg.ConfigFile, cfg.PackageJSON, cfg.Lockfile)
	seen := map[string]bool{}
	for _, input := range inputs {
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(cfg.Dir, path)
		}
		relative, err := filepath.Rel(rootPath, path)
		if err != nil || !filepath.IsLocal(relative) {
			root.Close()
			return nil, fmt.Errorf("watch binding escapes worktree")
		}
		if seen[relative] {
			continue
		}
		seen[relative] = true
		info, err := watchFileInfo(root, relative)
		if err != nil {
			root.Close()
			return nil, err
		}
		cohort.files = append(cohort.files, watchedFile{relative, info})
	}
	if len(cohort.files) > 128 {
		root.Close()
		return nil, fmt.Errorf("watch bound file count exceeds 128")
	}
	sort.Slice(cohort.files, func(i, j int) bool { return cohort.files[i].path < cohort.files[j].path })
	return cohort, nil
}

func watchFileInfo(root *os.Root, path string) (os.FileInfo, error) {
	parts := strings.Split(path, string(filepath.Separator))
	var info os.FileInfo
	for i := range parts {
		current, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil {
			return nil, fmt.Errorf("watch input unavailable: %w", err)
		}
		if current.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("watch input symlink refused")
		}
		info = current
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("watch input is not a regular file")
	}
	return info, nil
}

func (c *watchCohort) snapshot() (string, error) {
	for _, binding := range []struct {
		path string
		info os.FileInfo
	}{{c.rootPath, c.rootInfo}, {c.dirPath, c.dirInfo}} {
		if binding.path == "" {
			continue
		}
		current, err := os.Lstat(binding.path)
		if err != nil || !current.IsDir() || !os.SameFile(current, binding.info) {
			return "", fmt.Errorf("watch directory replaced or unavailable")
		}
	}
	hash := sha256.New()
	total := 0
	for _, file := range c.files {
		info, err := watchFileInfo(c.root, file.path)
		if err != nil {
			return "", err
		}
		if !os.SameFile(info, file.original) {
			return "", fmt.Errorf("watch input replaced")
		}
		f, err := c.root.Open(file.path)
		if err != nil {
			return "", err
		}
		opened, statErr := f.Stat()
		if statErr != nil || !os.SameFile(info, opened) {
			f.Close()
			return "", fmt.Errorf("watch input changed while opening")
		}
		data, readErr := io.ReadAll(io.LimitReader(f, int64(watchBytes-total+1)))
		closeErr := f.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		total += len(data)
		if total > watchBytes {
			return "", fmt.Errorf("watch content exceeds 8 MiB")
		}
		after, err := watchFileInfo(c.root, file.path)
		if err != nil {
			return "", err
		}
		if !os.SameFile(info, after) {
			return "", fmt.Errorf("watch input replaced during read")
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", file.path, len(data))
		hash.Write(data)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type watchResult struct {
	generation uint64
	receipt    jstestprovider.Receipt
	err        error
}

func watchRuns(ctx context.Context, snapshot func() (string, error), run func(context.Context) (jstestprovider.Receipt, error), publish func(jstestprovider.Receipt) error, status io.Writer) error {
	current, err := snapshot()
	if err != nil {
		return err
	}
	results := make(chan watchResult, 1)
	var generation uint64
	var cancel context.CancelFunc
	running := false
	pending := true
	settled := time.Now().Add(-watchInterval)
	start := func() {
		child, stop := context.WithCancel(ctx)
		cancel = stop
		running = true
		pending = false
		started := generation
		fmt.Fprintln(status, "watch: running; retained evidence freshness remains independently assessed")
		go func() { receipt, err := run(child); results <- watchResult{started, receipt, err} }()
	}
	defer func() {
		if cancel != nil {
			cancel()
		}
		if running {
			<-results
		}
	}()
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	start()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-results:
			running = false
			cancel()
			observed, err := snapshot()
			if err != nil {
				return err
			}
			if observed != current {
				current = observed
				generation++
				settled = time.Now()
				pending = true
			}
			if result.generation != generation {
				fmt.Fprintln(status, "watch: superseded completion withheld")
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if result.err != nil {
				return result.err
			}
			if err := publish(result.receipt); err != nil {
				return err
			}
		case <-ticker.C:
			observed, err := snapshot()
			if err != nil {
				return err
			}
			if observed != current {
				current = observed
				generation++
				settled = time.Now()
				pending = true
				fmt.Fprintln(status, "watch: source changed; prior retained evidence is historical, not a current source claim")
				if running {
					cancel()
				}
			}
			if pending && !running && time.Since(settled) >= watchInterval {
				start()
			}
		}
	}
}

func runUnitWatch(ctx context.Context, cfg jstestprovider.UnitConfig, retain bool, opts *watchOptions, stdout, stderr io.Writer) error {
	return runProviderWatch(ctx, cfg.Config, retain, opts, stdout, stderr, func(ctx context.Context, bound jstestprovider.Config) (jstestprovider.Receipt, error) {
		cfg.Config = bound
		return jstestprovider.RunUnit(ctx, cfg)
	})
}

func runE2EWatch(ctx context.Context, cfg jstestprovider.E2EConfig, retain bool, opts *watchOptions, stdout, stderr io.Writer) error {
	return runProviderWatch(ctx, cfg.Config, retain, opts, stdout, stderr, func(ctx context.Context, bound jstestprovider.Config) (jstestprovider.Receipt, error) {
		cfg.Config = bound
		return jstestprovider.RunE2E(ctx, cfg)
	})
}

func runProviderWatch(ctx context.Context, cfg jstestprovider.Config, retain bool, opts *watchOptions, stdout, stderr io.Writer, run func(context.Context, jstestprovider.Config) (jstestprovider.Receipt, error)) error {
	var err error
	cfg.PackageJSON, err = resolvePath(cfg.PackageJSON)
	if err != nil {
		return err
	}
	cfg.Lockfile, err = resolvePath(cfg.Lockfile)
	if err != nil {
		return err
	}
	cohort, err := prepareWatch(cfg, retain, opts)
	if err != nil {
		return err
	}
	defer cohort.root.Close()
	ctx, stop := context.WithTimeout(ctx, 60*time.Minute)
	defer stop()
	return watchRuns(ctx, cohort.snapshot, func(ctx context.Context) (jstestprovider.Receipt, error) {
		if _, err := cohort.snapshot(); err != nil {
			return jstestprovider.Receipt{}, err
		}
		return run(ctx, cfg)
	}, func(receipt jstestprovider.Receipt) error { return emit(stdout, stderr, receipt, cfg.Dir) }, stderr)
}
