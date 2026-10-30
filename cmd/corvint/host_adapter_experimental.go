package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/compactionkernel"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/unplannedread"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// The two experimental host-adapter call sites (decision 0099). Both are off
// unless the operator opts in, so the default adapter output is unchanged and
// neither slice is promoted into the shipped agent path (invariant 8).
const (
	experimentalKernelEnv      = "CORVINT_EXPERIMENTAL_COMPACTION_KERNEL"
	experimentalKernelDeadline = 250 * time.Millisecond
	experimentalKernelLabel    = "\nCorvint experimental compaction kernel (CKN-V0 proposed, not delivered):\n"
	experimentalKernelNotRun   = "\nCorvint experimental compaction kernel NOT_RUN: "
)

// experimentalKernelContext is the trusted-label-plus-framed kernel block for
// the session-start and user-prompt paths when CORVINT_EXPERIMENTAL_COMPACTION_KERNEL=1
// (CKN-V0-009). Every other event, and the unset variable, yields "".
func experimentalKernelContext(ctx context.Context, event, root string) string {
	if runtimeenv.Resolve(adapterLookup(ctx), "EXPERIMENTAL_COMPACTION_KERNEL") != "1" {
		return ""
	}
	if event != "session-start" && event != "user-prompt" {
		return ""
	}
	block, reason := experimentalKernelBlock(ctx, root)
	if reason != "" {
		return experimentalKernelNotRun + reason + "\n"
	}
	return experimentalKernelLabel + block
}

// experimentalKernelBlock loads only a fresh index snapshot, never builds one,
// so the opt-in cannot add an index build to a hook's deadline.
func experimentalKernelBlock(parent context.Context, root string) (string, string) {
	ctx, cancel := context.WithTimeout(parent, experimentalKernelDeadline)
	defer cancel()
	index, hit, err := loadSnapshot(ctx, root)
	if err != nil || !hit {
		return "", "index-snapshot-unavailable"
	}
	governance, err := kernelGovernance(ctx, index)
	if err != nil {
		return "", experimentalKernelCode(err)
	}
	block, err := compactionkernel.InjectionBlock(index, governance)
	if err != nil {
		return "", experimentalKernelCode(err)
	}
	if ctx.Err() != nil {
		return "", "kernel-deadline"
	}
	framed, err := repoenvelope.Frame(block)
	if err != nil {
		return "", repoenvelope.CollisionCode
	}
	return framed, ""
}

// experimentalKernelCode surfaces only a fixed error code, never a message that
// could carry repository text outside the envelope.
func experimentalKernelCode(err error) string {
	var coded *gokernel.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return "kernel-unavailable"
}

// withAdapterContextSuffix appends trusted adapter text after the rendered
// additionalContext; degraded outputs without one are left unchanged.
func withAdapterContextSuffix(output map[string]any, suffix string) map[string]any {
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	context, ok := hook["additionalContext"].(string)
	if !ok || suffix == "" {
		return output
	}
	hook["additionalContext"] = context + suffix
	return output
}

// recordDeliveredPacket is the user-prompt half of the unplanned-read call
// site (URE-V0-008): it records the planned set of the packet just delivered.
// It is a no-op without the operator's marker, and records a refused packet
// when the rendered output degraded and therefore delivered none. A silent
// output (AHI-046) delivered an empty planned set.
func recordDeliveredPacket(root, event string, input, result, output map[string]any) {
	if event != "user-prompt" {
		return
	}
	session, _ := input["sessionIdSha256"].(string)
	// The planned set is what the host received: the projection's sections, or none when the
	// adapter was silent because nothing was actionable.
	projection := hookContextProjection(event, result, false)
	if _, delivered := output["hookSpecificOutput"]; !delivered && (len(output) != 0 || projection != nil) {
		unplannedread.RefusePacket(root, session)
		return
	}
	unplannedread.RecordPacket(root, session, deliveredPacketPaths(projection))
}

// refuseUndeliveredPacket records a refused packet for a degraded user-prompt
// whose session identity is still valid, so later reads in that session
// abstain instead of scoring against an older packet (URE-V0-008).
func refuseUndeliveredPacket(root, event string, payload map[string]any) {
	if event != "user-prompt" {
		return
	}
	session, _ := payload["session_id"].(string)
	if session == "" || len(session) > 4096 {
		return
	}
	unplannedread.RefusePacket(root, claudeSessionHash(session))
}

type packetLedgerKey struct{}

// packetLedger settles a prompt packet's URE-V0-008 row once the watchdog knows
// whether the host receives the worker's output: a delivered output applies the
// worker's staged record, and an abandoned worker's session is refused instead,
// so a late worker never records a packet the host did not see (V1-0297).
type packetLedger struct {
	mu      sync.Mutex
	settled bool
	refuse  func()
	record  func()
}

func withPacketLedger(ctx context.Context) (context.Context, *packetLedger) {
	ledger := &packetLedger{}
	return context.WithValue(ctx, packetLedgerKey{}, ledger), ledger
}

// stagePacket holds the worker's refusal and record for the watchdog to settle.
// Without a watchdog the record applies at once; after the watchdog abandoned
// the worker only the refusal applies.
func stagePacket(ctx context.Context, refuse, record func()) {
	ledger, watched := ctx.Value(packetLedgerKey{}).(*packetLedger)
	if !watched {
		if record != nil {
			record()
		}
		return
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.settled {
		refuse()
		return
	}
	ledger.refuse, ledger.record = refuse, record
}

// settle applies the staged record when the worker's output was delivered, or
// the staged refusal when the watchdog abandoned the worker.
func (ledger *packetLedger) settle(delivered bool) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.settled = true
	action := ledger.refuse
	if delivered {
		action = ledger.record
	}
	if action != nil {
		action()
	}
}

// deliveredPacketPaths is the union of the path members of the delivered
// projection's three evidence sections; a nil projection delivered none.
func deliveredPacketPaths(packet map[string]any) []string {
	paths := []string{}
	for _, section := range []string{"governance", "declared_scope", "task_evidence"} {
		rows, _ := packet[section].([]any)
		for _, row := range rows {
			value, _ := row.(map[string]any)
			if path, ok := value["path"].(string); ok {
				paths = append(paths, path)
			}
		}
	}
	return paths
}
