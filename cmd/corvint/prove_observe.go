package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/observations"
)

// maxProofDocumentBytes bounds the document `prove-observe` reads: a proof
// embeds its packet, so it is larger than a harness input but never unbounded.
const maxProofDocumentBytes = 8 << 20

var proofFalsifiers = map[string]bool{
	falsifierHistory: true, falsifierReference: true, falsifierVerifier: true, falsifierMutant: true, falsifierNone: true,
}

var proofVerdicts = map[string]bool{falsifiedPass: true, falsifiedFail: true, falsifiedNotRun: true}

func parseProveObserveInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "prove-observe" {
		return "", nil, false, nil
	}
	resolved, err := normalizeRoot(root)
	return resolved, arguments[index+1:], true, err
}

// runProveObserve is the one writer for `proof` rows: it reads a `prove`
// document from stdin and appends its verdict counts, and nothing else about
// the document, to the self-observation ledger. `prove` itself never writes.
func runProveObserve(ctx context.Context, root string, arguments []string, stdin io.Reader, stderr io.Writer) int {
	if len(arguments) != 0 {
		emitError(stderr, argumentError("unrecognized arguments: "+strings.Join(arguments, " ")))
		return 2
	}
	document, err := readProofDocument(ctx, stdin)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	counts, err := proofCounts(document)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := observations.Append(root, observations.Event{Kind: "proof", Counts: counts}); err != nil {
		emitError(stderr, &gokernel.Error{Code: "observation-failed", Message: err.Error()})
		return 2
	}
	return 0
}

func readProofDocument(ctx context.Context, stdin io.Reader) (map[string]any, error) {
	data, err := readInputBounded(ctx, stdin, maxProofDocumentBytes)
	if err != nil {
		return nil, &gokernel.Error{Code: "invalid-proof-document", Message: "cannot read the proof document"}
	}
	if len(data) > maxProofDocumentBytes {
		return nil, proofDocumentSizeRefusal()
	}
	var document map[string]any
	if json.Unmarshal(data, &document) != nil {
		return nil, proofDocumentJSONRefusal()
	}
	return document, nil
}

// proofCounts recounts the document's own rows and accepts the document only
// when `proof.counts` agrees: a row list is what `prove` judged, so a document
// cannot claim verdicts it does not carry, and every falsifier and verdict
// must be one `prove` emits.
func proofCounts(document map[string]any) (map[string]map[string]int, error) {
	if stringAt(document, "tool") != "prove" || stringAt(document, "profile") != proveProfile || historicalDocument(document) {
		return nil, proofDocumentProfileRefusal()
	}
	proof, _ := document["proof"].(map[string]any)
	rows, hasRows := proof["rows"].([]any)
	claimed, hasCounts := proof["counts"].(map[string]any)
	if !hasRows || !hasCounts {
		return nil, proofDocumentMembersRefusal()
	}
	counted := map[string]map[string]int{}
	for _, row := range mapsFromAny(rows) {
		falsifier, verdict := stringAt(row, "falsifier"), stringAt(row, "falsified")
		if !proofFalsifiers[falsifier] || !proofVerdicts[verdict] {
			return nil, proofDocumentRowRefusal()
		}
		if counted[falsifier] == nil {
			counted[falsifier] = map[string]int{}
		}
		counted[falsifier][verdict]++
	}
	recounted, _ := json.Marshal(counted)
	stated, _ := json.Marshal(claimed)
	if string(recounted) != string(stated) {
		return nil, proofDocumentCountsRefusal()
	}
	return counted, nil
}

// proveLedger is the advisory falsification rate `prove` reports from the
// local ledger; it is read, never written, and absent without proof rows.
func proveLedger(root string) *observations.Falsification {
	digest, err := observations.Read(root)
	if err != nil || digest.Proofs == 0 {
		return nil
	}
	rate := digest.Falsification()
	return &rate
}
