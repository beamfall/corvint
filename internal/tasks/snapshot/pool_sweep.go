package snapshot

import (
	"bytes"
	"encoding/binary"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const MaxPoolSweepOutput = 65536

// PoolSweepTiming records wall-clock instants as Unix nanoseconds and monotonic
// durations as milliseconds. WaitReturned includes bounded pipe draining; the
// separate cleanup interval covers subsequent owned-group retirement probes.
type PoolSweepTiming struct {
	StartedAt, Deadline, WaitReturnedAt, CleanupEndedAt wire.Size
	ExecutionMillis, CleanupMillis                      wire.Size
	CleanupAllowanceMillis                              wire.Count
}

func (t PoolSweepTiming) value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("startedAt", wire.String(string(t.StartedAt))).Set("deadline", wire.String(string(t.Deadline))).Set("waitReturnedAt", wire.String(string(t.WaitReturnedAt))).Set("cleanupEndedAt", wire.String(string(t.CleanupEndedAt))).Set("executionMillis", wire.String(string(t.ExecutionMillis))).Set("cleanupMillis", wire.String(string(t.CleanupMillis))).Set("cleanupAllowanceMillis", wire.String(string(t.CleanupAllowanceMillis))))
}
func readSweepTiming(r *wire.Reader) PoolSweepTiming {
	r.Closed("startedAt", "deadline", "waitReturnedAt", "cleanupEndedAt", "executionMillis", "cleanupMillis", "cleanupAllowanceMillis")
	t := PoolSweepTiming{r.Field("startedAt").Size(), r.Field("deadline").Size(), r.Field("waitReturnedAt").Size(), r.Field("cleanupEndedAt").Size(), r.Field("executionMillis").Size(), r.Field("cleanupMillis").Size(), r.Field("cleanupAllowanceMillis").Count()}
	if t.StartedAt.Uint64() == 0 || t.Deadline.Uint64() == 0 || t.WaitReturnedAt.Uint64() == 0 || t.CleanupEndedAt.Uint64() == 0 || t.CleanupAllowanceMillis.Int() != 5000 {
		r.Fail(wire.CodeMalformed, "missing timing or invalid cleanup allowance")
	}
	return t
}

// Ownership remains durable between every phase receipt, including failed retries.
type PoolSweepOwner struct {
	RequestSha256 wire.Digest
	Previous      *wire.Digest
	Phase         string
	Attempt       wire.Count
	Tree          string
}

func ReadPoolSweepOwner(r *wire.Reader) *PoolSweepOwner {
	r.Closed("requestSha256", "previous", "phase", "attempt", "tree")
	o := &PoolSweepOwner{r.Field("requestSha256").Digest(), r.Field("previous").DigestOrNull(), r.Field("phase").Enum("cleanup", "reset", "verify", "confirm"), r.Field("attempt").Count(), r.Field("tree").OID()}
	if o.Attempt.Int() < 1 || o.Attempt.Int() > 2 {
		r.Fail(wire.CodeMalformed, "sweep attempt outside1..2")
	}
	return o
}
func PoolSweepOwnerValue(o *PoolSweepOwner) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("requestSha256", wire.String(string(o.RequestSha256))).Set("previous", sweepDigestOrNull(o.Previous)).Set("phase", wire.String(o.Phase)).Set("attempt", wire.String(string(o.Attempt))).Set("tree", wire.String(o.Tree)))
}

// The observation stores no structured environment values; private command logs can contain secrets.
type PoolSweepObservation struct {
	AllocationID, DefinitionSha256, Owner, Log, Environment, EnvFile wire.Digest
	Stdout, Stderr                                                   wire.Digest
	Timing                                                           PoolSweepTiming
	Signal                                                           *wire.Count
	Previous                                                         *wire.Digest
	Phase, Revision, Tree, Class                                     string
	Attempt                                                          wire.Count
	Exit                                                             *wire.Count
	Passed, GroupClean                                               bool
}

func (o PoolSweepObservation) Encode() []byte {
	exit := wire.Null()
	if o.Exit != nil {
		exit = wire.String(string(*o.Exit))
	}
	signal := wire.Null()
	if o.Signal != nil {
		signal = wire.String(string(*o.Signal))
	}
	return wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("timing", o.Timing.value()).Set("stdout", wire.String(string(o.Stdout))).Set("stderr", wire.String(string(o.Stderr))).Set("signal", signal).Set("profile", wire.String("taskman-pool-sweep-observation/0")).Set("allocationId", wire.String(string(o.AllocationID))).Set("definitionSha256", wire.String(string(o.DefinitionSha256))).Set("owner", wire.String(string(o.Owner))).Set("previous", sweepDigestOrNull(o.Previous)).Set("phase", wire.String(o.Phase)).Set("attempt", wire.String(string(o.Attempt))).Set("revision", wire.String(o.Revision)).Set("tree", wire.String(o.Tree)).Set("class", wire.String(o.Class)).Set("exit", exit).Set("passed", wire.Bool(o.Passed)).Set("groupClean", wire.Bool(o.GroupClean)).Set("log", wire.String(string(o.Log))).Set("environment", wire.String(string(o.Environment))).Set("envFile", wire.String(string(o.EnvFile)))))
}
func DecodePoolSweepObservation(raw []byte) (*PoolSweepObservation, error) {
	if len(raw) > MaxPoolObservationBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "sweep observation", "byte bound")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	r := wire.NewReader(v, "sweep observation")
	r.Profile("taskman-pool-sweep-observation/0")
	r.Closed("profile", "allocationId", "definitionSha256", "owner", "previous", "phase", "attempt", "revision", "tree", "class", "exit", "passed", "groupClean", "log", "environment", "envFile", "stdout", "stderr", "signal", "timing")
	r.Field("profile").Exact("taskman-pool-sweep-observation/0")
	o := &PoolSweepObservation{AllocationID: r.Field("allocationId").Digest(), DefinitionSha256: r.Field("definitionSha256").Digest(), Owner: r.Field("owner").Digest(), Previous: r.Field("previous").DigestOrNull(), Phase: r.Field("phase").Enum("cleanup", "reset", "verify"), Attempt: r.Field("attempt").Count(), Revision: r.Field("revision").OID(), Tree: r.Field("tree").OID(), Class: r.Field("class").Enum("EXIT_ZERO", "EXIT_NONZERO", "STDOUT_MISMATCH", "SPAWN_FAILED", "TIMEOUT", "INTERRUPTED", "OUTPUT_LIMIT", "UNKNOWN", "SOURCE_CHANGED", "SIGNAL"), Passed: r.Field("passed").Bool(), GroupClean: r.Field("groupClean").Bool(), Log: r.Field("log").Digest(), Environment: r.Field("environment").Digest(), EnvFile: r.Field("envFile").Digest()}
	o.Stdout = r.Field("stdout").Digest()
	o.Stderr = r.Field("stderr").Digest()
	o.Timing = readSweepTiming(r.Field("timing"))
	if !r.Field("signal").IsNull() {
		x := r.Field("signal").Count()
		o.Signal = &x
		if x.Int() < 1 || x.Int() > 255 {
			r.Fail(wire.CodeMalformed, "invalid signal")
		}
	}
	if o.Class == "SIGNAL" && (o.Signal == nil || o.Exit != nil) {
		r.Fail(wire.CodeMalformed, "signal result needs signal identity")
	}
	if r.Field("exit").Value().Kind != wire.KindNull {
		x := r.Field("exit").Count()
		o.Exit = &x
		if x.Int() < 0 || x.Int() > 255 {
			r.Fail(wire.CodeMalformed, "invalid exit")
		}
	}
	if (o.Signal != nil && o.Exit != nil) || ((o.Class == "EXIT_ZERO" || o.Class == "EXIT_NONZERO") && o.Exit == nil) {
		r.Fail(wire.CodeMalformed, "exit class/identity mismatch")
	}
	if o.Attempt.Int() < 1 || o.Attempt.Int() > 2 || (o.Passed && (!o.GroupClean || o.Exit == nil || (o.Class != "EXIT_ZERO" && o.Class != "EXIT_NONZERO"))) {
		r.Fail(wire.CodeMalformed, "invalid sweep result")
	}
	if e = r.Err(); e != nil {
		return nil, e
	}
	if !bytes.Equal(raw, o.Encode()) {
		return nil, wire.Errorf(wire.CodeMalformed, "sweep observation", "noncanonical")
	}
	return o, nil
}
func EncodePoolSweepLog(stdout, stderr []byte) []byte {
	raw := make([]byte, 4+len(stdout)+len(stderr))
	binary.BigEndian.PutUint32(raw, uint32(len(stdout)))
	copy(raw[4:], stdout)
	copy(raw[4+len(stdout):], stderr)
	return raw
}
func DecodePoolSweepLog(raw []byte) ([]byte, []byte, error) {
	if len(raw) < 4 || len(raw) > MaxPoolSweepOutput+4 {
		return nil, nil, wire.Errorf(wire.CodeLimitExceeded, "sweep log", "byte bound")
	}
	n := int(binary.BigEndian.Uint32(raw))
	if n > len(raw)-4 {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "sweep log", "stdout size")
	}
	return raw[4 : 4+n], raw[4+n:], nil
}

func sweepDigestOrNull(d *wire.Digest) wire.Value {
	if d == nil {
		return wire.Null()
	}
	return wire.String(string(*d))
}
