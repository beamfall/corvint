package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func TestPSRLogEncodingBounds(t *testing.T) {
	raw := EncodePoolSweepLog([]byte("out"), []byte("err"))
	out, err, e := DecodePoolSweepLog(raw)
	if e != nil || string(out) != "out" || string(err) != "err" {
		t.Fatal(out, err, e)
	}
	if _, _, e = DecodePoolSweepLog(make([]byte, MaxPoolSweepOutput+5)); e == nil {
		t.Fatal("unbounded log")
	}
	if _, _, e = DecodePoolSweepLog([]byte{0, 0, 0, 9}); e == nil {
		t.Fatal("bad length")
	}
}
func TestPSRObservationCanonical(t *testing.T) {
	d := wire.Sum(nil)
	x := wire.Count("0")
	o := PoolSweepObservation{AllocationID: d, DefinitionSha256: d, Owner: d, Log: d, Environment: d, EnvFile: d, Phase: "verify", Attempt: "1", Revision: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40), Class: "EXIT_ZERO", Exit: &x, Passed: true, GroupClean: true, Stdout: d, Stderr: d, Timing: PoolSweepTiming{StartedAt: "1", Deadline: "2", WaitReturnedAt: "2", CleanupEndedAt: "3", ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: "5000"}}
	raw := o.Encode()
	got, e := DecodePoolSweepObservation(raw)
	if e != nil || !bytes.Equal(raw, got.Encode()) {
		t.Fatal(e)
	}
	if _, e = DecodePoolSweepObservation(append([]byte(" "), raw...)); e == nil {
		t.Fatal("noncanonical accepted")
	}
	o.GroupClean = false
	if _, e = DecodePoolSweepObservation(o.Encode()); e == nil {
		t.Fatal("unsafe passed accepted")
	}
}

func TestPSRObservationEvidenceBounds(t *testing.T) {
	d := wire.Sum(nil)
	sig := wire.Count("15")
	o := PoolSweepObservation{AllocationID: d, DefinitionSha256: d, Owner: d, Previous: &d, Log: d, Stdout: d, Stderr: d, Environment: d, EnvFile: d, Phase: "verify", Attempt: "2", Revision: strings.Repeat("1", 64), Tree: strings.Repeat("2", 64), Class: "SIGNAL", Signal: &sig, GroupClean: true, Timing: PoolSweepTiming{StartedAt: "18446744073709551615", Deadline: "18446744073709551615", WaitReturnedAt: "18446744073709551615", CleanupEndedAt: "18446744073709551615", ExecutionMillis: "18446744073709551615", CleanupMillis: "18446744073709551615", CleanupAllowanceMillis: "5000"}}
	raw := o.Encode()
	decoded, e := DecodePoolSweepObservation(raw)
	if e != nil || !bytes.Equal(raw, decoded.Encode()) || len(raw) > MaxPoolObservationBytes {
		t.Fatal("max-field fixture", len(raw), e)
	}
	t.Logf("max-width signal observation %d bytes", len(raw))
	for _, mode := range []string{"missing-timing", "signal-without-identity", "signal-plus-exit", "bad-allowance", "missing-stream"} {
		bad := o
		switch mode {
		case "missing-timing":
			bad.Timing = PoolSweepTiming{}
		case "signal-without-identity":
			bad.Signal = nil
		case "signal-plus-exit":
			x := wire.Count("0")
			bad.Exit = &x
		case "bad-allowance":
			bad.Timing.CleanupAllowanceMillis = "5001"
		case "missing-stream":
			bad.Stdout = ""
		}
		if _, e := DecodePoolSweepObservation(bad.Encode()); e == nil {
			t.Fatal("invalid observation accepted", mode)
		}
	}
}
