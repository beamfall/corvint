package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/groupreap"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func TestAggregateCallerHoldRetainsLockAndStopsFollowOnWork(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native Owner unavailable")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, release, err := dogfoodoperation.Acquire(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ctx = context.WithValue(ctx, aggregateOwnerFinishKey{}, func(owner *groupreap.Owner, bound groupreap.RetirementBound) groupreap.Result {
		calls++
		result := owner.FinishBounded(bound)
		if result.State != groupreap.Released {
			t.Fatalf("real test group not released: %+v", result)
		}
		result.State = groupreap.Hold
		return result
	})
	_, _, _, err = runAggregateOwnedProcess(ctx, exec.Command("/bin/sh", "-c", "printf retained-prefix"), 1024, 1024, time.Now().Add(time.Minute))
	if !errors.Is(err, dogfoodoperation.ErrCleanupHold) {
		t.Fatal(err)
	}
	release()
	if _, next, err := dogfoodoperation.Acquire(context.Background(), dir); err == nil || next != nil {
		t.Fatal("next writer admitted")
	}
	if _, err := os.Stat(filepath.Join(dir, "corvint/local-completion/operation.lock/hold.json")); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "must-not-start")
	_, _, _, err = runAggregateOwnedProcess(ctx, exec.Command("/bin/sh", "-c", "touch \"$1\"", "sh", marker), 1024, 1024, time.Now().Add(time.Minute))
	if !errors.Is(err, dogfoodoperation.ErrCleanupHold) || calls != 1 {
		t.Fatal("follow-on owner admitted", err, calls)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("follow-on process ran")
	}
	published := false
	c := check{flow: flow{ctx: ctx}, options: CheckOptions{AggregatePublish: func(context.Context, string, AggregateCheckCapture) error { published = true; return nil }}}
	if err := c.finishAggregateCapture(2, nil); !errors.Is(err, dogfoodoperation.ErrCleanupHold) || published {
		t.Fatal("HOLD capture started publication", err)
	}
}

func workerTestRequest() AggregateWorkerRequest {
	return AggregateWorkerRequest{AggregateWorkerProfile, "produce", tracerecordrepo.AggregateExpected{
		ObjectFormat: "sha1", Base: strings.Repeat("1", 40), Target: strings.Repeat("2", 40), Tree: strings.Repeat("3", 40), AdmissionPolicy: tracerecordrepo.AggregateAdmissionPolicy, Task: "Local completion " + strings.Repeat("a", 64), Verification: []string{"go test ./internal/parser"}, Outcome: "passed"}, 56, 10000, json.RawMessage("null")}
}

func TestAggregateWorkerClosedRequest(t *testing.T) {
	request := workerTestRequest()
	raw, err := aggregateWorkerJSON(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAggregateWorkerRequest(raw); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown", func(m map[string]any) { m["argv"] = []string{"sh"} }},
		{"missing", func(m map[string]any) { delete(m, "receipt") }},
		{"zero-quota", func(m map[string]any) { m["remainingGitOperations"] = 0 }},
		{"large-quota", func(m map[string]any) { m["remainingGitOperations"] = 65 }},
		{"fractional-quota", func(m map[string]any) { m["remainingGitOperations"] = 1.5 }},
		{"null-quota", func(m map[string]any) { m["remainingGitOperations"] = nil }},
		{"zero-time", func(m map[string]any) { m["remainingWorkMilliseconds"] = 0 }},
		{"large-time", func(m map[string]any) { m["remainingWorkMilliseconds"] = 300001 }},
		{"unknown-profile", func(m map[string]any) { m["profile"] = "other" }},
		{"mixed-mode", func(m map[string]any) { m["mode"] = "verify" }},
		{"missing-expected-member", func(m map[string]any) { delete(m["expected"].(map[string]any), "task") }},
		{"unknown-expected-member", func(m map[string]any) { m["expected"].(map[string]any)["command"] = "sh" }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			mutation.change(value)
			encoded, err := aggregateWorkerJSON(value)
			if err != nil {
				return
			}
			if _, err := ParseAggregateWorkerRequest(encoded); err == nil {
				t.Fatal("accepted mutation")
			}
		})
	}
	for _, invalid := range [][]byte{append([]byte(" "), raw...), bytes.TrimSuffix(raw, []byte{'\n'}), bytes.Replace(raw, []byte(`"mode":"produce"`), []byte(`"mode":"produce","mode":"produce"`), 1), bytes.Repeat([]byte(" "), aggregateWorkerFrameLimit+1)} {
		if _, err := ParseAggregateWorkerRequest(invalid); err == nil {
			t.Fatal("accepted noncanonical/duplicate/oversized request")
		}
	}
	if _, err := ExecuteAggregateWorker(context.Background(), t.TempDir(), request); err == nil {
		t.Fatal("execution outside owned worker accepted")
	}
}

func TestAggregateWorkerBufferHardLimit(t *testing.T) {
	b := &aggregateWorkerBuffer{limit: 4, overflow: make(chan struct{})}
	if n, err := b.Write([]byte("1234")); n != 4 || err != nil {
		t.Fatal(n, err)
	}
	if n, err := b.Write([]byte("5678")); n != 0 || err == nil {
		t.Fatal(n, err)
	}
	if string(b.data()) != "1234" {
		t.Fatal("buffer grew beyond bound")
	}
	select {
	case <-b.overflow:
	default:
		t.Fatal("overflow not reported")
	}
}

func TestAggregateVerifierClosedReadCommands(t *testing.T) {
	oid := strings.Repeat("a", 40)
	commands := [][]string{
		{"query", "--task", "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket", "--limit", "1"},
		{"impact", "--base", oid, "--range-profile", "expanded-256", "--limit", "20"},
		{"dogfood-ocm", "status", "--expected-base", oid, "--target", oid},
		{"cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", oid, "--target", oid, "--max-unknown", "12", "--max-mechanical", "0"},
	}
	for _, args := range commands {
		if !ValidAggregateVerifierArguments(args) {
			t.Fatalf("valid read refused %v", args)
		}
		if ValidAggregateVerifierArguments(append(append([]string{}, args...), "--write")) {
			t.Fatal("extra flag admitted")
		}
		changed := append([]string{}, args...)
		changed[0] = "dogfood-record"
		if ValidAggregateVerifierArguments(changed) {
			t.Fatal("write admitted")
		}
	}
	if ValidAggregateVerifierArguments([]string{"cem", "prepare"}) {
		t.Fatal("write admitted")
	}
	for _, task := range []string{"", " ", "write a poem", "work queue\x00", string([]byte{255}), strings.Repeat("work queue ", 1000)} {
		if ValidAggregateVerifierArguments([]string{"query", "--task", task, "--limit", "1"}) {
			t.Fatalf("invalid query task admitted %q", task)
		}
	}
	if ValidAggregateVerifierArguments([]string{"query", "--task", "identify the work queue", "--limit", "2"}) {
		t.Fatal("different query limit admitted")
	}
}

func TestAggregateOwnedProcessRetiresDescendants(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native owner unavailable")
	}
	for _, mode := range []string{"normal", "deadline", "cancel", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			script := `sleep 60 & child=$!; printf '%s' "$child" > "$1"; `
			switch mode {
			case "normal":
				script += "exit 0"
			case "overflow":
				script += "yes x"
			default:
				script += "wait"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			work := time.Now().Add(5 * time.Second)
			if mode == "deadline" {
				work = time.Now().Add(250 * time.Millisecond)
			}
			if mode == "cancel" {
				timer := time.AfterFunc(250*time.Millisecond, cancel)
				defer timer.Stop()
			}
			command := exec.Command("/bin/sh", "-c", script, "fixture", pidFile)
			_, _, status, err := runAggregateOwnedProcess(ctx, command, 1024, 1024, work)
			if mode == "normal" && (err != nil || status != 0) {
				t.Fatalf("normal: %d %v", status, err)
			}
			if mode != "normal" && err == nil {
				t.Fatal("failed process accepted")
			}
			raw, readErr := os.ReadFile(pidFile)
			if readErr != nil {
				t.Fatal(readErr)
			}
			pid, parseErr := strconv.Atoi(string(raw))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			process, findErr := os.FindProcess(pid)
			if findErr != nil {
				t.Fatal(findErr)
			}
			defer process.Release()
			if err = process.Signal(syscall.Signal(0)); err == nil {
				t.Fatalf("owned descendant %d survived %s", pid, mode)
			}
		})
	}
}

func TestAggregateOwnedProcessReservesRetirementBeforeSpawn(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native owner unavailable")
	}
	marker := filepath.Join(t.TempDir(), "spawned")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := exec.Command("/bin/sh", "-c", `echo started > "$1"`, "fixture", marker)
	_, _, _, err := runAggregateOwnedProcess(ctx, command, 1024, 1024, time.Now().Add(time.Minute))
	if err == nil || err.Error() != "aggregate-operation-deadline" {
		t.Fatalf("short admission: %v", err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("spawned without retirement allowance: %v", err)
	}
}

func TestAggregateWorkerTypedFailureBoundary(t *testing.T) {
	valid := []byte(`{"code": "aggregate-admitted-limit", "error": "changed_paths exceeds 512 paths", "ok": false}` + "\n")
	if err := aggregateWorkerFailure(2, nil, valid); err.Error() != "aggregate-admitted-limit" {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"code":"aggregate-admitted-limit","error":"x","ok":false,"extra":1}`,
		`{"code":"foreign-code","error":"x","ok":false}`,
		`{"code":"aggregate-admitted-limit","error":"x","ok":true}`,
		`{"code":"aggregate-admitted-limit","error":"x","ok":false,"ok":false}`,
	} {
		if err := aggregateWorkerFailure(2, nil, []byte(raw)); err.Error() != "aggregate-worker-invalid" {
			t.Fatal(err)
		}
	}
	if err := aggregateWorkerFailure(2, []byte("unissued"), valid); err.Error() != "aggregate-worker-invalid" {
		t.Fatal(err)
	}
	if err := aggregateWorkerFailure(1, nil, valid); err.Error() != "aggregate-worker-invalid" {
		t.Fatal(err)
	}
}

func TestAggregateWorkerAllMemberShapes(t *testing.T) {
	raw, err := aggregateWorkerJSON(workerTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]string{"request": {"profile", "mode", "expected", "remainingGitOperations", "remainingWorkMilliseconds", "receipt"}, "expected": {"objectFormat", "base", "target", "tree", "admissionPolicy", "task", "verification", "outcome"}}
	for group, members := range groups {
		for _, member := range members {
			for _, shape := range []string{"missing", "null", "wrong-type"} {
				if group == "request" && member == "receipt" && shape == "null" {
					continue
				} // produce receipt is explicitly null.
				if group == "expected" && member == "verification" && shape == "null" {
					continue
				} // semantic expected validation occurs during acquisition.
				if group == "request" && member == "receipt" && shape == "missing" || group == "request" && member == "remainingGitOperations" && shape == "null" || group == "expected" && member == "task" && shape == "missing" {
					continue
				} // accepted existing cells.
				t.Run(group+"/"+member+"/"+shape, func(t *testing.T) {
					var value map[string]any
					if err := json.Unmarshal(raw, &value); err != nil {
						t.Fatal(err)
					}
					selected := value
					if group == "expected" {
						selected = value["expected"].(map[string]any)
					}
					switch shape {
					case "missing":
						delete(selected, member)
					case "null":
						selected[member] = nil
					case "wrong-type":
						selected[member] = true
					}
					mutant, err := aggregateWorkerJSON(value)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = ParseAggregateWorkerRequest(mutant); err == nil {
						t.Fatal("closed worker mutation accepted")
					}
				})
			}
		}
	}
	for _, tc := range []struct {
		name    string
		value   any
		fresh   func() any
		members []string
	}{
		{"produce", aggregateProduceResponse{AggregateWorkerProfile, "produce", 3, json.RawMessage(`{}`), aggregateLegacyRefusal{"admitted-path-limit", "changed_paths exceeds 200 paths", false}}, func() any { return new(aggregateProduceResponse) }, []string{"profile", "mode", "consumedGitOperations", "receipt", "legacyAdmission"}},
		{"verify", aggregateVerifyResponse{AggregateWorkerProfile, "verify", 3, "sha256:" + strings.Repeat("a", 64)}, func() any { return new(aggregateVerifyResponse) }, []string{"profile", "mode", "consumedGitOperations", "verifiedEnvelopeSha256"}},
	} {
		original, err := aggregateWorkerJSON(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range tc.members {
			for _, shape := range []string{"missing", "null", "wrong-type"} {
				if member == "receipt" && shape != "missing" {
					continue
				} // opaque JSON, semantically parsed by caller.
				t.Run(tc.name+"/"+member+"/"+shape, func(t *testing.T) {
					var value map[string]any
					if err := json.Unmarshal(original, &value); err != nil {
						t.Fatal(err)
					}
					switch shape {
					case "missing":
						delete(value, member)
					case "null":
						value[member] = nil
					case "wrong-type":
						value[member] = true
					}
					mutant, err := aggregateWorkerJSON(value)
					if err != nil {
						t.Fatal(err)
					}
					if err = aggregateWorkerDecode(mutant, tc.fresh()); err == nil {
						t.Fatal("closed response mutation accepted")
					}
				})
			}
		}
	}
}

func TestAggregateWorkerLegacyRefusalMemberShapes(t *testing.T) {
	control := aggregateProduceResponse{AggregateWorkerProfile, "produce", 3, json.RawMessage(`{}`), aggregateLegacyRefusal{"admitted-path-limit", "changed_paths exceeds 200 paths", false}}
	raw, err := aggregateWorkerJSON(control)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"code", "error", "ok"} {
		for _, shape := range []string{"missing", "null", "wrong-type"} {
			t.Run(member+"/"+shape, func(t *testing.T) {
				var value map[string]any
				if err := json.Unmarshal(raw, &value); err != nil {
					t.Fatal(err)
				}
				nested := value["legacyAdmission"].(map[string]any)
				switch shape {
				case "missing":
					delete(nested, member)
				case "null":
					nested[member] = nil
				case "wrong-type":
					nested[member] = []any{}
				}
				mutant, err := aggregateWorkerJSON(value)
				if err != nil {
					t.Fatal(err)
				}
				var decoded aggregateProduceResponse
				if err = aggregateWorkerDecode(mutant, &decoded); err == nil {
					t.Fatal("nested legacy refusal accepted")
				}
			})
		}
	}
}
