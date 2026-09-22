package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const piHostVersion = "0.85.1"
const piAdapterVersion = "0.2.0"
const piInputLimit = 131072

var piReceipt = regexp.MustCompile(`^harness-receipt:sha256:[0-9a-f]{64}$`)
var piDegradations = map[string]bool{"compaction-critical-evidence-overflow": true, "compaction-dirty-set-over-budget": true, "compaction-untracked-paths-not-rehydratable": true, "frontier-authority-unavailable": true, "outcome-persistence-unavailable": true}

func piEnvelope(event, version any, fault any) map[string]any {
	return map[string]any{"profile": "corvint-pi-adapter/0", "event": event, "host": "pi", "surface": "extension", "hostVersion": version, "adapterVersion": piAdapterVersion, "support": "FALLBACK", "receiptId": nil, "context": "", "degradations": []string{}, "fault": fault, "shouldContinue": false}
}
func piNoNull(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		for _, x := range v {
			if !piNoNull(x) {
				return false
			}
		}
	case []any:
		for _, x := range v {
			if !piNoNull(x) {
				return false
			}
		}
	}
	return true
}
func runPiAdapter(parent context.Context, args []string, stdin io.Reader, stdout io.Writer) int {
	var event any
	if len(args) == 1 {
		for _, e := range gokernel.SupportedEvents() {
			if args[0] == e {
				event = e
			}
		}
	}
	if event == nil {
		return emitAdapterOutput(stdout, piEnvelope(nil, nil, "unsupported-event"))
	}
	// Leave process startup, typed output and group cleanup inside the 2s transport cap.
	ctx, cancel := context.WithTimeout(parent, 1100*time.Millisecond)
	defer cancel()
	done := make(chan map[string]any, 1)
	go func() { done <- piAdapterResult(ctx, event.(string), stdin) }()
	select {
	case result := <-done:
		return emitAdapterOutput(stdout, result)
	case <-ctx.Done():
		return emitAdapterOutput(stdout, piEnvelope(event, nil, "deadline"))
	}
}
func piAdapterResult(ctx context.Context, event string, stdin io.Reader) map[string]any {
	fault := func(version any, code string) map[string]any { return piEnvelope(event, version, code) }
	raw, err := io.ReadAll(io.LimitReader(stdin, piInputLimit+1))
	if err != nil || len(raw) > piInputLimit {
		return fault(nil, "invalid-input")
	}
	payload, err := decodeAdapterJSON(raw)
	if err != nil || len(payload) != 2 || !piNoNull(payload) {
		return fault(nil, "invalid-input")
	}
	version, ok := payload["hostVersion"].(string)
	if !ok {
		return fault(nil, "invalid-input")
	}
	if version != piHostVersion {
		return fault(nil, "unsupported-host-version")
	}
	input, ok := payload["input"].(map[string]any)
	if !ok {
		return fault(version, "invalid-input")
	}
	data, _ := json.Marshal(input)
	root, err := os.Getwd()
	if err != nil {
		return fault(version, "core-unavailable")
	}
	result, err := gokernel.HandleEventContext(ctx, gokernel.EventRequest{Root: root, Host: "pi", HostVersion: version, Surface: "extension", AdapterVersion: piAdapterVersion, Event: event, Input: data, BudgetBytes: gokernel.MinOutputBytes, IndexedContext: harnessIndexedContext, SharedIndexedContext: sharedIndexedContextFromEnvironment()})
	if err != nil {
		if ctx.Err() != nil {
			return fault(version, "deadline")
		}
		var inputError *gokernel.Error
		if errors.As(err, &inputError) && inputError.Code == "invalid-harness-input" {
			return fault(version, "invalid-input")
		}
		return fault(version, "core-unavailable")
	}
	adapter, _ := result["adapter"].(map[string]any)
	receipt, _ := result["receiptId"].(string)
	if result["profile"] != gokernel.Profile || result["ok"] != true || result["support"] != "FALLBACK" || result["event"] != event || adapter["host"] != "pi" || adapter["surface"] != "extension" || adapter["hostVersion"] != version || adapter["adapterVersion"] != piAdapterVersion || !piReceipt.MatchString(receipt) {
		return fault(version, "invalid-core-response")
	}
	codes, ok := result["degradations"].([]any)
	if !ok {
		return fault(version, "invalid-core-response")
	}
	seen := map[string]bool{}
	degradations := []string{}
	for _, v := range codes {
		code, ok := v.(string)
		if !ok || !piDegradations[code] || seen[code] {
			return fault(version, "invalid-core-response")
		}
		seen[code] = true
		degradations = append(degradations, code)
	}
	if event == "stop" {
		frontier, _ := result["frontier"].(map[string]any)
		if frontier["state"] != "UNAVAILABLE" || frontier["shouldContinue"] != false {
			return fault(version, "invalid-core-response")
		}
	}
	output := piEnvelope(event, version, nil)
	output["receiptId"] = receipt
	output["degradations"] = degradations
	if event == "session-start" || event == "user-prompt" {
		raw, err := json.Marshal(result)
		if err != nil {
			return fault(version, "invalid-core-response")
		}
		framed, err := repoenvelope.Frame(string(raw))
		if err != nil {
			return fault(version, "invalid-core-response")
		}
		output["context"] = framed
	}
	encoded, err := json.Marshal(output)
	if err != nil || len(encoded)+1 > adapterOutputLimit {
		return fault(version, "output-too-large")
	}
	return output
}
