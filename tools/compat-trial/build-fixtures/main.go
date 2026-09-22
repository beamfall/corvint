// Build the closed native fixture registry into a replay runner. This development
// tool has no executable-registration input: package paths and build argv are fixed.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func main() {
	out := flag.String("out", "", "new output directory")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := build(ctx, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func build(ctx context.Context, out string) error {
	if runtime.Version() != "go1.27.1" {
		return errors.New("requires go1.27.1")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	source, err := os.ReadFile(filepath.Join(root, "tools/compat-trial/fixtures/old/main.go"))
	if err != nil {
		return err
	}
	if out == "" {
		return errors.New("-out required")
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if err = os.Mkdir(out, 0700); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "corvint-owned-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	env := []string{"GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "GOPROXY=off", "GOSUMDB=off", "HOME=" + temporary, "GOCACHE=" + filepath.Join(temporary, "cache"), "PATH=/usr/bin:/bin", "TZ=UTC", "LC_ALL=C"}
	// A caller's explicit cache may accelerate compilation; it is never a replay input.
	if cache := os.Getenv("GOCACHE"); cache != "" {
		env[7] = "GOCACHE=" + cache
	}
	run := func(name, pkg, ldflags string) error {
		argv := []string{filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", filepath.Join(out, name), pkg}
		o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: env, Timeout: 60 * time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1 << 20})
		if o.Err != nil {
			return fmt.Errorf("owned build: %w: %s", o.Err, o.Stderr)
		}
		if o.ExitStatus != 0 {
			return fmt.Errorf("owned build exit %d: %s", o.ExitStatus, o.Stderr)
		}
		return nil
	}
	hashes := []string{}
	for _, v := range []string{"old", "new"} {
		if err = run(v, "./tools/compat-trial/fixtures/"+v, ""); err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(out, v))
		if err != nil {
			return err
		}
		hashes = append(hashes, sha(b))
	}
	if err = run("compat-trial", "./tools/compat-trial", "-X main.ownedVariantHashes="+hashes[0]+","+hashes[1]); err != nil {
		return err
	}
	newSource, err := os.ReadFile(filepath.Join(root, "tools/compat-trial/fixtures/new/main.go"))
	if err != nil {
		return err
	}
	manifest := map[string]any{"new_source": map[string]string{"path": "tools/compat-trial/fixtures/new/main.go", "sha256": sha(newSource)}, "source": map[string]string{"path": "tools/compat-trial/fixtures/old/main.go", "sha256": sha(source)}, "build_identity": map[string]any{"go_version": runtime.Version(), "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "CGO_ENABLED": "0", "build_flags": []string{"-trimpath", "-buildvcs=false", "-ldflags="}}, "fixtures": []map[string]string{{"name": "old", "sha256": hashes[0]}, {"name": "new", "sha256": hashes[1]}}}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "fixture-manifest.json"), b, 0600)
}
