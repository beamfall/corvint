package jstestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/jstestprovider/loopbackhttp"
	"testing"
	"time"
)

// TestMTVV0950ProbesFailClosedWithoutClient: in a binary that never imported
// loopbackhttp every probe refuses at once instead of waiting out its bound.
func TestMTVV0950ProbesFailClosedWithoutClient(t *testing.T) {
	start := time.Now()
	if err := waitReady(context.Background(), "http://127.0.0.1:1/", time.Minute, nil); !errors.Is(err, errNoLoopbackGet) {
		t.Errorf("waitReady = %v", err)
	}
	if err := externalReady(context.Background(), "http://127.0.0.1:1/", time.Minute, nil); !errors.Is(err, errNoLoopbackGet) {
		t.Errorf("externalReady = %v", err)
	}
	if reason, _ := probeHTTP(context.Background(), "http://127.0.0.1:1/", nil, time.Minute, nil); reason != "http-unreachable" {
		t.Errorf("probeHTTP reason = %q", reason)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("probes waited %v without a client", elapsed)
	}
	if client := LoopbackGet(loopbackhttp.Get); client == nil {
		t.Fatal("the test binary links no loopback client; loopback_probe_test.go must import loopbackhttp")
	}
}

func TestCallerOwnedLoopbackAddresses(t *testing.T) {
	for address, want := range map[string]bool{
		"http://127.0.0.1:3000/":          true,
		"http://[::1]:3000/":              true,
		"http://[::ffff:127.0.0.1]:3000/": true,
		"http://[::1%25lo0]:3000/":        false,
		"http://localhost:3000/":          false,
		"http://10.0.0.1:3000/":           false,
		"https://127.0.0.1/":              false,
	} {
		if got := callerOwnedLoopback(address); got != want {
			t.Errorf("callerOwnedLoopback(%q) = %v, want %v", address, got, want)
		}
	}
}

// LGP-V0-8600: probe clients are supplied per request in either invocation order.
func TestExplicitProbeClientsAreIndependent(t *testing.T) {
	calls := [2]int{}
	a := func(context.Context, string, bool) (int, error) { calls[0]++; return 200, nil }
	b := func(context.Context, string, bool) (int, error) { calls[1]++; return 204, nil }
	for _, get := range []LoopbackGet{a, b, b, a} {
		if err := externalReady(context.Background(), "http://127.0.0.1/", time.Second, get); err != nil {
			t.Fatal(err)
		}
	}
	if calls != [2]int{2, 2} {
		t.Fatalf("clients crossed requests: %v", calls)
	}
	if err := externalReady(context.Background(), "http://127.0.0.1/", time.Second, nil); !errors.Is(err, errNoLoopbackGet) {
		t.Fatalf("client leaked: %v", err)
	}
}

// Runtime wiring is never part of the serialized worker job (MTV-V0-950).
func TestExplicitProbeClientDoesNotChangeWorkerJSON(t *testing.T) {
	plain, err := json.Marshal(E2EConfig{})
	if err != nil {
		t.Fatal(err)
	}
	wired, err := json.Marshal(E2EConfig{LoopbackGet: loopbackhttp.Get})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain, wired) || bytes.Contains(wired, []byte("LoopbackGet")) {
		t.Fatalf("runtime client changed job: %s", wired)
	}
	cfg := E2EConfig{LoopbackGet: loopbackhttp.Get}
	if err := json.Unmarshal(wired, &cfg); err != nil || cfg.LoopbackGet == nil {
		t.Fatalf("job lost supplied runtime client: %v", err)
	}
}
