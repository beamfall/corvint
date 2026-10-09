package snapshot

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-027: shared observation admits supported nonfixture identity, never cleanup.
func TestCALV0027_NonfixtureStageBinding(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_NonfixtureStageBinding)
}

func testCALV0027_NonfixtureStageBinding(t *testing.T) {
	for _, op := range []string{StageInit, StageUnpause, StageMutate} {
		t.Run(op, func(t *testing.T) {
			kind := map[string]string{StageInit: "INIT", StageUnpause: "UNPAUSE", StageMutate: "MUTATION"}[op]
			o, b := completedStageBinding(t, op, kind)
			q, err := intent.DecodeQueue(b.QueueRaw)
			if err != nil {
				t.Fatal(err)
			}
			q.Fixture = false
			b.QueueRaw = wire.EncodeFile(q.Value())
			before := bytes.Clone(b.QueueRaw)
			if err := o.Bind(b); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, b.QueueRaw) {
				t.Fatal("observation mutated queue")
			}
			for _, mode := range []string{"import-map", "fixture-cutover", "init-cutover", "wrong-queue", "stale", "malformed"} {
				t.Run(mode, func(t *testing.T) {
					copyBinding := b
					queue, err := intent.DecodeQueue(b.QueueRaw)
					if err != nil {
						t.Fatal(err)
					}
					want := wire.CodeUnsupported
					switch mode {
					case "import-map":
						d := wire.Sum([]byte("map"))
						queue.ImportMapSha256 = &d
					case "fixture-cutover", "init-cutover":
						if mode == "init-cutover" && op != StageInit {
							t.Skip("INIT-specific boundary")
						}
						queue.Fixture = mode == "fixture-cutover"
						v := queue.Value()
						v.Obj.Set("executionCutover", stageObject("enabledBy", wire.String("owner"), "decisionRef", wire.String("decision"), "gateEvidence", wire.Strings([]string{string(wire.Sum([]byte("evidence")))})))
						copyBinding.QueueRaw = wire.EncodeFile(v)
					case "wrong-queue":
						copyBinding.QueueID = "queue:other:main"
						want = wire.CodeJournalForked
					case "stale":
						h, err := DecodeHead(b.HeadRaw)
						if err != nil {
							t.Fatal(err)
						}
						h.LastSeq = "1000001"
						copyBinding.HeadRaw = wire.EncodeFile(h.Value())
						want = wire.CodeJournalForked
					case "malformed":
						copyBinding.QueueRaw = []byte("{")
						want = wire.CodeMalformed
					}
					if mode != "fixture-cutover" && mode != "init-cutover" && mode != "malformed" {
						copyBinding.QueueRaw = wire.EncodeFile(queue.Value())
					}
					if err := o.Bind(copyBinding); wire.CodeOf(err) != want {
						t.Fatalf("want %s: %v", want, err)
					}
				})
			}
			if op != StageInit {
				v := q.Value()
				v.Obj.Set("executionCutover", stageObject("enabledBy", wire.String("owner"), "decisionRef", wire.String("decision"), "gateEvidence", wire.Strings([]string{string(wire.Sum([]byte("evidence")))})))
				b.QueueRaw = wire.EncodeFile(v)
				if err := o.Bind(b); err != nil {
					t.Fatalf("postcutover observation: %v", err)
				}
			}
		})
	}
}

// CAL-V0-027: completed classes remain closed, and observation preserves input bytes.
func TestCALV0027_CompletedStageReceiptKinds(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_CompletedStageReceiptKinds)
}

func testCALV0027_CompletedStageReceiptKinds(t *testing.T) {
	for _, tc := range []struct{ op, kind string }{
		{StageMutate, "MUTATION"}, {StageMutate, "ARCHIVE"}, {StageMutate, "RESTORE"},
		{StageLease, "ADMIT"}, {StageLease, "TRANSITION"}, {StageLease, "GATE_RESULT"}, {StageLease, "MANIFEST"},
		{StageRelease, "RELEASE"}, {StageRelease, "RECONCILE"},
		// V1-0466: exact direct and reconciliation aliases stay bounded.
		{StageKeepJournal, "RECONCILE"}, {StageAdoptFile, "RECONCILE"}, {StageEscalation, "TRANSITION"},
		{StagePolicyUpdate, "POLICY_UPDATE"}, {StageImportApply, "IMPORT_APPLY"}, {StageAuthoritySwitch, "AUTHORITY_SWITCH"}, {StageQualification, "QUALIFICATION"},
	} {
		t.Run(tc.op+"/"+tc.kind, func(t *testing.T) {
			o, b := completedStageBinding(t, tc.op, tc.kind)
			q, err := intent.DecodeQueue(b.QueueRaw)
			if err != nil {
				t.Fatal(err)
			}
			q.Fixture = false
			b.QueueRaw = wire.EncodeFile(q.Value())
			descriptor, err := o.Descriptor.Encode()
			if err != nil {
				t.Fatal(err)
			}
			head, receipt, request := bytes.Clone(b.HeadRaw), bytes.Clone(b.ReceiptRaw), bytes.Clone(b.RequestRaw)
			if err := o.Bind(b); err != nil {
				t.Fatal(err)
			}
			after, err := o.Descriptor.Encode()
			if err != nil || !bytes.Equal(descriptor, after) || !bytes.Equal(head, b.HeadRaw) || !bytes.Equal(receipt, b.ReceiptRaw) || !bytes.Equal(request, b.RequestRaw) {
				t.Fatal("binding changed evidence", err)
			}
		})
	}
	for _, tc := range []struct{ op, kind string }{
		{StageMutate, "ADMIT"}, {StageLease, "MUTATION"}, {StageRelease, "MANIFEST"},
		{StageMutate, "RECONCILE"}, {StageLease, "RELEASE"}, {StageRelease, "ARCHIVE"},
		{StageEscalation, "ADMIT"}, {StageKeepJournal, "RELEASE"},
		{StagePolicyUpdate, "MUTATION"}, {StageQualification, "TRANSITION"}, {StageImportApply, "IMPORT_PLAN"},
	} {
		t.Run("refuse/"+tc.op+"/"+tc.kind, func(t *testing.T) {
			o, b := completedStageBinding(t, tc.op, tc.kind)
			before, err := o.Descriptor.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if err := o.Bind(b); wire.CodeOf(err) != wire.CodeJournalForked {
				t.Fatalf("cross-class accepted: %v", err)
			}
			after, err := o.Descriptor.Encode()
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal changed descriptor", err)
			}
		})
	}
	// FENCED and unknown labels are not receipt kinds: decoding refuses them first, and
	// the mapping itself abstains.
	if stageReceiptKind("UNKNOWN", "UNKNOWN") || stageReceiptKind(StageLease, "FENCED") || stageReceiptKind(StageMutate, "UNKNOWN") {
		t.Fatal("unknown pair admitted")
	}
}

// CAL-V0-027: recomputing outer hashes cannot erase the inner receipt/request bindings.
func TestCALV0027_CompletedStageInnerBindings(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_CompletedStageInnerBindings)
}

func testCALV0027_CompletedStageInnerBindings(t *testing.T) {
	for _, mode := range []string{"outcome", "codes", "timestamp", "previous", "request-id", "request-digest", "generation", "receipt-hash"} {
		t.Run(mode, func(t *testing.T) {
			o, b := completedStageBinding(t, StageMutate, "MUTATION")
			receipt, err := wire.Parse(b.ReceiptRaw)
			if err != nil {
				t.Fatal(err)
			}
			head, err := DecodeHead(b.HeadRaw)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "outcome":
				receipt.Obj.Set("outcome", wire.String("REVISION_CONFLICT"))
			case "codes":
				receipt.Obj.Set("codes", wire.Strings([]string{wire.CodeFenced}))
			case "timestamp":
				receipt.Obj.Set("recordedAt", wire.String("2026-09-07T00:00:00Z"))
			case "previous":
				receipt.Obj.Set("prev", wire.String(string(wire.Sum([]byte("wrong previous")))))
			case "request-id":
				receipt.Obj.Set("requestId", wire.String("other"))
			case "request-digest":
				o.Descriptor.RequestSha256 = wire.Sum([]byte("different request"))
			case "generation":
				head.Generation = "1"
			}
			b.ReceiptRaw = wire.EncodeFile(receipt)
			digest := wire.Sum(b.ReceiptRaw)
			head.LastReceiptSha256 = &digest
			b.HeadRaw = wire.EncodeFile(head.Value())
			for i := range o.Descriptor.Artifacts {
				a := &o.Descriptor.Artifacts[i]
				var raw []byte
				if a.Role == "RECEIPT" {
					raw = b.ReceiptRaw
				}
				if a.Role == "HEAD" {
					raw = b.HeadRaw
				}
				if raw != nil {
					a.Bytes = wire.SizeOf(uint64(len(raw)))
					a.Sha256 = wire.Sum(raw)
				}
			}
			descriptor, err := o.Descriptor.Encode()
			if err != nil {
				t.Fatal(err)
			}
			o, err = ObserveStage([]StageFile{{"active.json", descriptor}})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "receipt-hash" {
				b.ReceiptRaw = append(bytes.Clone(b.ReceiptRaw), ' ')
			}
			beforeHead, beforeReceipt, beforeRequest := bytes.Clone(b.HeadRaw), bytes.Clone(b.ReceiptRaw), bytes.Clone(b.RequestRaw)
			if err := o.Bind(b); wire.CodeOf(err) != wire.CodeJournalForked {
				t.Fatalf("inconsistent %s accepted: %v", mode, err)
			}
			after, err := o.Descriptor.Encode()
			if err != nil || !bytes.Equal(descriptor, after) || !bytes.Equal(beforeHead, b.HeadRaw) || !bytes.Equal(beforeReceipt, b.ReceiptRaw) || !bytes.Equal(beforeRequest, b.RequestRaw) {
				t.Fatal("refusal modified evidence", err)
			}
		})
	}
}
