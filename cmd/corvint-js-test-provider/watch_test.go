package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

func TestWatchRequiresBindings(t *testing.T) {
	cfg := jstestprovider.UnitConfig{Config: jstestprovider.Config{Dir: t.TempDir(), PackageJSON: "package.json", Lockfile: "package-lock.json", ConfigFile: "vitest.config.js", TestFiles: []string{"unit.test.js"}}}
	for _, missing := range []string{"retain", "package", "lock", "config", "tests"} {
		t.Run(missing, func(t *testing.T) {
			c := cfg
			retain := true
			switch missing {
			case "retain":
				retain = false
			case "package":
				c.PackageJSON = ""
			case "lock":
				c.Lockfile = ""
			case "config":
				c.ConfigFile = ""
			case "tests":
				c.TestFiles = nil
			}
			if _, err := prepareWatch(c.Config, retain, &watchOptions{paths: []string{"math.js"}}); err == nil {
				t.Fatal("missing binding accepted")
			}
		})
	}
	for _, args := range [][]string{{"--watch", "a"}, {"--watch", "../a", "--foreground", "--experimental", "--trusted-local"}, {"--foreground", "--experimental", "--trusted-local"}} {
		if _, _, err := splitWatchOptions(args); err == nil {
			t.Fatal("invalid admission accepted", args)
		}
	}
}

func TestWatchSnapshotRefusesReplacementAndSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a")
	os.WriteFile(path, []byte("a"), 0600)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	info, _ := os.Stat(path)
	c := watchCohort{root: root, files: []watchedFile{{"a", info}}}
	before, err := c.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("b"), 0600)
	after, err := c.snapshot()
	if err != nil || before == after {
		t.Fatal("edit not observed", err)
	}
	os.WriteFile(filepath.Join(dir, "new"), []byte("b"), 0600)
	os.Rename(filepath.Join(dir, "new"), path)
	if _, err := c.snapshot(); err == nil {
		t.Fatal("replacement accepted")
	}
	os.Remove(path)
	os.Symlink("new", path)
	if _, err := watchFileInfo(root, "a"); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestWatchSupersessionABAAndShutdownWait(t *testing.T) {
	var identity atomic.Int32
	var runs atomic.Int32
	var active atomic.Int32
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	published := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- watchRuns(ctx, func() (string, error) {
			if identity.Load() == 0 {
				return "a", nil
			}
			return "b", nil
		}, func(ctx context.Context) (jstestprovider.Receipt, error) {
			active.Add(1)
			defer active.Add(-1)
			if runs.Add(1) == 1 {
				close(started)
				<-ctx.Done()
				close(cancelled)
				<-release
				return jstestprovider.Receipt{}, ctx.Err()
			}
			<-ctx.Done()
			return jstestprovider.Receipt{}, ctx.Err()
		}, func(jstestprovider.Receipt) error { published <- struct{}{}; return nil }, io.Discard)
	}()
	<-started
	identity.Store(1)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("not cancelled")
	}
	identity.Store(0)
	time.Sleep(2 * watchInterval)
	close(release)
	time.Sleep(3 * watchInterval)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not join runner")
	}
	if active.Load() != 0 {
		t.Fatal("runner left active")
	}
	select {
	case <-published:
		t.Fatal("superseded ABA result published")
	default:
	}
	if runs.Load() < 2 {
		t.Fatal("newest cohort not rerun")
	}
}

func TestWatchParsingPreservesOneShotValues(t *testing.T) {
	for _, args := range [][]string{{"--test-file", "--watch", "--config", "--foreground"}, {"--env-key", "--experimental"}, {"--server-arg", "--watch", "--test-arg", "--foreground"}, {"--", "--watch"}, {"positional", "--watch"}, {"--test-file=--watch"}} {
		filtered, watch, err := splitWatchOptions(args)
		if err != nil || watch != nil || !reflect.DeepEqual(filtered, args) {
			t.Fatalf("one-shot arguments changed: %q => %q, %v, %v", args, filtered, watch, err)
		}
	}
}

func TestWatchRefusesReplacedRoot(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "root")
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0600)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	info, _ := os.Lstat(dir)
	fileInfo, _ := os.Stat(filepath.Join(dir, "a"))
	c := watchCohort{root: root, rootPath: dir, rootInfo: info, dirPath: dir, dirInfo: info, files: []watchedFile{{"a", fileInfo}}}
	if _, err := c.snapshot(); err != nil {
		t.Fatal(err)
	}
	os.Rename(dir, dir+"-old")
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0600)
	if _, err := c.snapshot(); err == nil {
		t.Fatal("replacement root accepted")
	}
}

func TestWatchCancellationCannotPublishCompletedResult(t *testing.T) {
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		published := false
		err := watchRuns(ctx, func() (string, error) { return "a", nil }, func(context.Context) (jstestprovider.Receipt, error) { cancel(); return jstestprovider.Receipt{}, nil }, func(jstestprovider.Receipt) error { published = true; return nil }, io.Discard)
		if err != context.Canceled || published {
			t.Fatal("cancelled completion published", err)
		}
	}
}

func TestWatchHelpNamesAdmissionAndLimits(t *testing.T) {
	fs := flag.NewFlagSet("unit", flag.ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)
	configureWatchHelp(fs)
	fs.Usage()
	for _, text := range []string{"--watch RELPATH", "--foreground --experimental --trusted-local --retain", "package-json, lockfile, config", "60-minute", "in-place", "UNKNOWN"} {
		if !strings.Contains(out.String(), text) {
			t.Fatal("missing help", text)
		}
	}
}
