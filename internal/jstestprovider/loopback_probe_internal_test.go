package jstestprovider

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestMTVV0950ProbesFailClosedWithoutClient: in a binary that never imported
// loopbackhttp every probe refuses at once instead of waiting out its bound.
func TestMTVV0950ProbesFailClosedWithoutClient(t *testing.T) {
	saved := loopbackGet
	loopbackGet = nil
	t.Cleanup(func() { loopbackGet = saved })
	start := time.Now()
	if err := waitReady(context.Background(), "http://127.0.0.1:1/", time.Minute); !errors.Is(err, errNoLoopbackGet) {
		t.Errorf("waitReady = %v", err)
	}
	if err := externalReady(context.Background(), "http://127.0.0.1:1/", time.Minute); !errors.Is(err, errNoLoopbackGet) {
		t.Errorf("externalReady = %v", err)
	}
	if reason, _ := probeHTTP(context.Background(), "http://127.0.0.1:1/", nil, time.Minute); reason != "http-unreachable" {
		t.Errorf("probeHTTP reason = %q", reason)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("probes waited %v without a client", elapsed)
	}
	if saved == nil {
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
