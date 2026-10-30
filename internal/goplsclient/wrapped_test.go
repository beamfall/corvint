// SPDX-License-Identifier: Apache-2.0
package goplsclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// wrappedInitialize is each wrapped fake's initialize result (LWB-V0-004).
var wrappedInitialize = map[string]any{
	"wrapped-number":  map[string]any{"capabilities": map[string]any{"definitionProvider": map[string]any{}, "textDocumentSync": 1}},
	"wrapped-object":  map[string]any{"serverInfo": map[string]string{"name": "fake-ts", "version": "1.2.3"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "definitionProvider": true, "textDocumentSync": map[string]any{"openClose": true, "change": 2}}},
	"wrapped-utf8":    map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-8", "definitionProvider": true, "textDocumentSync": 1}},
	"wrapped-nodef":   map[string]any{"capabilities": map[string]any{"textDocumentSync": 1}},
	"wrapped-nosync":  map[string]any{"capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": 0}},
	"wrapped-badname": map[string]any{"serverInfo": map[string]string{"name": "fake\u0001ts"}, "capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": 1}},
	"wrapped-gopls":   map[string]any{"serverInfo": map[string]string{"name": "not-gopls", "version": "dev"}, "capabilities": map[string]any{"definitionProvider": true, "textDocumentSync": 2}},
}

func argsAfterDashes() []string {
	if i := slices.Index(os.Args, "--"); i >= 0 {
		return os.Args[i+1:]
	}
	return nil
}

func startWrapped(t *testing.T, mode string, args ...string) (*Client, *snapshots, Config, error) {
	t.Helper()
	cfg, store := fakeConfig(t, mode)
	cfg.Wrapped, cfg.Args = true, args
	cfg.LanguageID = func(uri string) string {
		if strings.HasSuffix(uri, ".go") {
			return "fake-language"
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c, err := Start(ctx, cfg)
	if err == nil {
		t.Cleanup(func() {
			cancel()
			select {
			case <-c.Done():
			case <-time.After(5 * time.Second):
				t.Error("cleanup did not finish")
			}
		})
	}
	return c, store, cfg, err
}

// LWB-V0-004: the generic profile admits number or object sync, optional
// serverInfo and a boolean or options definition provider, forwards argv
// verbatim, sends no gopls initialization options, and names each document's
// languageId from the caller.
func TestWrappedNegotiationAndLanguage(t *testing.T) {
	for _, v := range []struct{ mode, name, version string }{{"wrapped-number", "", ""}, {"wrapped-object", "fake-ts", "1.2.3"}, {"wrapped-gopls", "not-gopls", "dev"}} {
		t.Run(v.mode, func(t *testing.T) {
			c, _, cfg, err := startWrapped(t, v.mode, "--stdio")
			if err != nil {
				t.Fatal(err)
			}
			p := c.Profile()
			if p.ServerName != v.name || p.ServerVersion != v.version || p.PositionEncoding != "utf-16" || !p.Definition || len(p.ExecutableSHA256) != 64 {
				t.Fatalf("%+v", p)
			}
			argv, _ := os.ReadFile(filepath.Join(cfg.Root, "argv"))
			if string(argv) != "--stdio\nfalse" {
				t.Fatalf("argv/options %q", argv)
			}
			uri := fileURI(filepath.Join(cfg.Root, "main.go"))
			if err := c.Open(context.Background(), uri); err != nil {
				t.Fatal(err)
			}
			rows, err := c.Definition(context.Background(), uri, Position{2, 8})
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			language, _ := os.ReadFile(filepath.Join(cfg.Root, "language-id"))
			if string(language) != "fake-language" {
				t.Fatalf("languageId %q", language)
			}
			if err := c.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// LWB-V0-004: a server outside the generic profile is refused before use.
func TestWrappedNegotiationRefusals(t *testing.T) {
	for _, mode := range []string{"wrapped-utf8", "wrapped-nodef", "wrapped-nosync", "wrapped-badname"} {
		t.Run(mode, func(t *testing.T) {
			if _, _, _, err := startWrapped(t, mode); !errors.Is(err, ErrUnsupported) {
				t.Fatal(err)
			}
		})
	}
	cfg, _ := fakeConfig(t, "wrapped-number")
	cfg.Wrapped = true
	if _, err := Start(context.Background(), cfg); !errors.Is(err, ErrUnsupported) {
		t.Fatal("wrapped profile without a languageId mapping", err)
	}
}

// LWB-V0-006: a configured executable that does not exist is the named
// missing-binary refusal, for gopls and wrapped servers alike.
func TestMissingExecutable(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		cfg, _ := fakeConfig(t, "wrapped-number")
		cfg.Executable = filepath.Join(cfg.Root, "absent-server")
		cfg.Wrapped, cfg.LanguageID = wrapped, func(string) string { return "x" }
		if _, err := Start(context.Background(), cfg); !errors.Is(err, ErrMissing) {
			t.Fatal(wrapped, err)
		}
	}
}
