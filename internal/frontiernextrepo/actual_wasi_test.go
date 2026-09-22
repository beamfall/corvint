//go:build darwin || linux

package frontiernextrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
)

// This explicit optional campaign runs the actual separately built protected
// driver prototype. Its fixture signature remains unadmitted, and NONE in the
// prototype never becomes production authority. The default gate reports SKIP.
func TestActualWASICandidateClosesOnlyFixtureRelation(t *testing.T) {
	binary, driverPath := os.Getenv("PLE_PROTOTYPE"), os.Getenv("PLE_DRIVER_SOURCE")
	if binary == "" || driverPath == "" {
		t.Skip("NOT_RUN: explicit optional WASI prototype and source required")
	}
	compiler, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../wp3codec/codec.go")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := os.ReadFile(driverPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutant := range []bool{false, true} {
		t.Run(map[bool]string{false: "good", true: "mutant"}[mutant], func(t *testing.T) {
			root, r, p := fixtureRequest(t, source, driver, mutant)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "prototype", filepath.Join(root, "internal/wp3codec/codec.go"), compiler)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				if cmd.Process == nil {
					return nil
				}
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			cmd.WaitDelay = time.Second
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()
			var actual struct{ Authority, Status, Cleanup, SourceSHA256, RecipeSHA256, WasmSHA256, WorkerSHA256, DriverSHA256, Nonce string }
			if err := json.Unmarshal(stdout.Bytes(), &actual); err != nil {
				t.Fatalf("prototype %v: %s %s", runErr, stdout.Bytes(), stderr.Bytes())
			}
			if actual.Authority != "NONE" || actual.Cleanup != "OWNED_GROUP_REAPED" {
				t.Fatalf("unqualified prototype lifecycle: %s", stdout.Bytes())
			}
			if actual.DriverSHA256 != localauthority.BytesDigest(driver) {
				t.Fatal("prototype driver differs from base-stable claim source")
			}
			expectedStatus := "PASS"
			expectedState := "EMPTY"
			if mutant {
				expectedStatus = "FAIL"
				expectedState = "OPEN"
			}
			if actual.Status != expectedStatus {
				t.Fatalf("actual execution %v: %s", runErr, stdout.Bytes())
			}
			r.Enrollment.Nonce = actual.Nonce
			r.Enrollment.Binding.SourceSHA256 = actual.SourceSHA256
			r.Enrollment.Binding.RecipeSHA256 = actual.RecipeSHA256
			r.Enrollment.Binding.WasmSHA256 = actual.WasmSHA256
			r.Enrollment.Binding.WorkerSHA256 = actual.WorkerSHA256
			sign(t, &r, &p, actual.Status)
			result, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
			if err != nil || result.State != expectedState || result.Authority != "NONE" {
				t.Fatalf("actual %s: %+v %v", actual.Status, result, err)
			}
			t.Logf("actual %s candidate; %s authority %s; source %s wasm %s", actual.Status, result.State, result.Authority, actual.SourceSHA256, actual.WasmSHA256)
		})
	}
}
