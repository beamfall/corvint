package main

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func validateSource(source []byte) error {
	if len(source) == 0 || len(source) > maxSource {
		return errors.New("source bound")
	}
	f, e := parser.ParseFile(token.NewFileSet(), "codec.go", source, parser.ParseComments)
	if e != nil {
		return e
	}
	if f.Name.Name != "wp3codec" {
		return errors.New("wrong package")
	}
	allowed := map[string]bool{"bytes": true, "errors": true, "sort": true, "strconv": true, "strings": true, "unicode/utf8": true}
	for _, i := range f.Imports {
		p, e := strconv.Unquote(i.Path.Value)
		if e != nil || !allowed[p] {
			return errors.New("unadmitted source import")
		}
	}
	for _, c := range f.Comments {
		for _, line := range c.List {
			if strings.Contains(line.Text, "go:") {
				return errors.New("compiler directive refused")
			}
		}
	}
	return nil
}
func build(source []byte, goPath string) ([]byte, error) {
	if e := validateSource(source); e != nil {
		return nil, e
	}
	tool, e := executableGo(goPath)
	if e != nil {
		return nil, e
	}
	dir, e := os.MkdirTemp("", "corvint-keyless-build-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	for _, p := range []string{"internal/wp3codec", "cmd/check", "cache", "tmp", "home"} {
		if e = os.MkdirAll(filepath.Join(dir, p), 0700); e != nil {
			return nil, e
		}
	}
	for path, data := range map[string][]byte{"go.mod": []byte(moduleText), "internal/wp3codec/codec.go": source, "cmd/check/main.go": wrapper} {
		if e = os.WriteFile(filepath.Join(dir, path), data, 0400); e != nil {
			return nil, e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 58*time.Second)
	defer cancel()
	env := []string{"GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOENV=off", "GOWORK=off", "GOCACHE=" + filepath.Join(dir, "cache"), "GOTMPDIR=" + filepath.Join(dir, "tmp"), "HOME=" + filepath.Join(dir, "home"), "GOMAXPROCS=2"}
	version := exec.CommandContext(ctx, tool, "version")
	version.Env = env
	v, e := version.Output()
	if e != nil {
		return nil, e
	}
	if !strings.HasPrefix(string(v), "go version go1.27.0 ") {
		return nil, errors.New("toolchain version mismatch")
	}
	cmd := exec.CommandContext(ctx, tool, "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", "candidate.wasm", "./cmd/check")
	cmd.Dir = dir
	cmd.Env = env
	var diagnostic limitedBuffer
	diagnostic.max = maxOutput
	cmd.Stdout = &diagnostic
	cmd.Stderr = &diagnostic
	if e = cmd.Run(); e != nil {
		return nil, fmt.Errorf("build failed: %w %s", e, diagnostic.Bytes())
	}
	f, e := os.Open(filepath.Join(dir, "candidate.wasm"))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return readBound(f, maxModule)
}
