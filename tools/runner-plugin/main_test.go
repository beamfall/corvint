// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/runnerprotocol"
)

// RSP-V0-041: the explicit operator command installs the reviewed bytes.
func TestRSPV0041InstallCommand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("safe-open unavailable")
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	_, err = io.Copy(h, file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "pins.json")
	pin := runnerprotocol.Pin{Name: "example", Protocol: runnerprotocol.Protocol, Executable: source, SHA256: hex.EncodeToString(h.Sum(nil)), Runners: []string{"example"}}
	data, _ := json.Marshal(runnerprotocol.Discovery{Schema: runnerprotocol.DiscoverySchema, Plugins: []runnerprotocol.Pin{pin}})
	if err = os.WriteFile(manifest, data, 0600); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	var out, diagnostics bytes.Buffer
	if rc := run([]string{"install", "--manifest", manifest, "--store", store}, &out, &diagnostics); rc != 0 {
		t.Fatal(rc, diagnostics.String())
	}
	config, err := runnerprotocol.ReadDiscovery(filepath.Join(store, "runner-plugins.json"))
	if err != nil || len(config.Plugins) != 1 || config.Plugins[0].SHA256 != pin.SHA256 {
		t.Fatal(config, err)
	}
}
func TestRSPV0041CommandRefusesMissingInputs(t *testing.T) {
	for _, args := range [][]string{{"install"}, {"install", "--unknown"}, {"unrecognized"}} {
		var out, diagnostics bytes.Buffer
		if rc := run(args, &out, &diagnostics); rc != 2 {
			t.Fatalf("%v: %d", args, rc)
		}
	}
	var out, diagnostics bytes.Buffer
	if rc := run([]string{"--help"}, &out, &diagnostics); rc != 0 || !bytes.Contains(out.Bytes(), []byte("Example:")) {
		t.Fatal("missing help")
	}
}
