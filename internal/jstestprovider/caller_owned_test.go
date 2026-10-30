package jstestprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// callerOwnedConfig stages a stand-in Playwright runner that prints one passing spec and records
// that it ran; the caller-owned app is the given loopback server.
func callerOwnedConfig(t *testing.T, readyURL string) (E2EConfig, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"new.spec.js", "playwright.config.js", "package.json", "package-lock.json"} {
		writeFile(t, filepath.Join(dir, name), "{}\n")
	}
	report := `{"suites":[{"title":"new.spec.js","specs":[{"title":"counter","file":"new.spec.js","line":1,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":3}]}]}]}],"errors":[]}`
	ran := filepath.Join(t.TempDir(), "ran")
	runner := filepath.Join(t.TempDir(), "runner")
	writeFile(t, runner, "#!/bin/sh\n: > '"+ran+"'\nprintf '%s' '"+report+"'\n")
	if err := os.Chmod(runner, 0o755); err != nil {
		t.Fatal(err)
	}
	return E2EConfig{
		Config:            Config{Dir: dir, TestFiles: []string{filepath.Join(dir, "new.spec.js")}, ConfigFile: filepath.Join(dir, "playwright.config.js"), PackageJSON: filepath.Join(dir, "package.json"), Lockfile: filepath.Join(dir, "package-lock.json"), RunnerName: "playwright", RunnerVersion: "fixture", Timeout: 20 * time.Second},
		CallerOwnedServer: true, ServerReadyURL: readyURL, ServerReadyLimit: 5 * time.Second, TestArgv: []string{runner}, AppBuildDir: dir,
	}, ran
}

// TestCallerOwnedServerAdmission covers NEA-V0-012's provider mode: it composes with no
// provider-started server, external PWP mode or freshness, and needs a ready URL.
func TestCallerOwnedServerAdmission(t *testing.T) {
	base, _ := callerOwnedConfig(t, "http://127.0.0.1:1/")
	if err := AdmitE2E(base); err != nil {
		t.Fatalf("caller-owned mode refused: %v", err)
	}
	for name, mutate := range map[string]func(*E2EConfig){
		"provider server":  func(c *E2EConfig) { c.ServerArgv = []string{"/bin/true"} },
		"external mode":    func(c *E2EConfig) { c.ExternalServer = true },
		"freshness":        func(c *E2EConfig) { c.Freshness = &FreshnessConfig{} },
		"no ready url":     func(c *E2EConfig) { c.ServerReadyURL = "" },
		"keep reporters":   func(c *E2EConfig) { c.KeepReporters = true },
		"attempt details":  func(c *E2EConfig) { c.RetainAttemptDetails = true },
		"sensitive inputs": func(c *E2EConfig) { c.SensitiveInputPolicy = &SensitiveInputPolicy{} },
	} {
		cfg := base
		mutate(&cfg)
		if err := AdmitE2E(cfg); err == nil || !strings.Contains(err.Error(), "caller-owned-server-composition-unsupported") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	for _, address := range []string{"http://localhost:4173/", "http://10.0.0.1:4173/", "https://127.0.0.1:4173/", "http://u:p@127.0.0.1:4173/"} {
		cfg := base
		cfg.ServerReadyURL = address
		if err := AdmitE2E(cfg); err == nil || !strings.Contains(err.Error(), "caller-owned-server-not-loopback") {
			t.Fatalf("%s: err = %v", address, err)
		}
	}
}

// TestCallerOwnedServerUnavailableAtPublish runs the stand-in runner against a caller-owned
// server: the provider never owns it, publishes the runner's outcomes while it answers, and names
// server-unavailable-at-publish when it stops answering after the runner exits.
func TestCallerOwnedServerUnavailableAtPublish(t *testing.T) {
	t.Run("answering", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		defer server.Close()
		cfg, ran := callerOwnedConfig(t, server.URL+"/")
		receipt, err := RunE2E(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Infrastructure != nil || len(receipt.Tests) != 1 || receipt.Tests[0].State != StatePassed || receipt.ServerDescendantsGone != nil {
			t.Fatalf("receipt = %+v infra=%+v", receipt, receipt.Infrastructure)
		}
		if _, err := os.Stat(ran); err != nil {
			t.Fatalf("runner did not run: %v", err)
		}
	})
	t.Run("stopped answering", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if requests.Add(1) > 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}))
		defer server.Close()
		cfg, ran := callerOwnedConfig(t, server.URL+"/")
		receipt, err := RunE2E(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "server-unavailable-at-publish" || receipt.Infrastructure.Detail != "caller-owned-server-not-answering" || len(receipt.Tests) != 0 {
			t.Fatalf("receipt = %+v infra=%+v", receipt, receipt.Infrastructure)
		}
		if _, err := os.Stat(ran); err != nil {
			t.Fatalf("runner did not run: %v", err)
		}
	})
	t.Run("redirect not followed", func(t *testing.T) {
		var followed atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed.Add(1) }))
		defer target.Close()
		server := httptest.NewServer(http.RedirectHandler(target.URL+"/", http.StatusFound))
		defer server.Close()
		cfg, ran := callerOwnedConfig(t, server.URL+"/")
		cfg.ServerReadyLimit = 300 * time.Millisecond
		receipt, err := RunE2E(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "server-not-ready" || followed.Load() != 0 {
			t.Fatalf("infra = %+v followed=%d", receipt.Infrastructure, followed.Load())
		}
		if _, err := os.Stat(ran); !os.IsNotExist(err) {
			t.Fatalf("runner ran after a redirect: %v", err)
		}
	})
	t.Run("never ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := server.URL + "/"
		server.Close()
		cfg, ran := callerOwnedConfig(t, url)
		cfg.ServerReadyLimit = 300 * time.Millisecond
		receipt, err := RunE2E(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "server-not-ready" {
			t.Fatalf("infra = %+v", receipt.Infrastructure)
		}
		if _, err := os.Stat(ran); !os.IsNotExist(err) {
			t.Fatalf("runner ran without a ready server: %v", err)
		}
	})
}
