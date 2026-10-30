package gokernel

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// SOL-V0-010, AHI-022 (V1-0767): an OpenCode post-tool event's adapterCodes become one
// content-free adapter-degradation row per code, deduplicated per window, and change
// neither the receipt nor the response.
func TestOpenCodeAdapterCodesAreLedgeredOutsideTheReceipt(t *testing.T) {
	root := testRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	openCode := func(input string) EventRequest {
		request := request(root, "post-tool", input)
		request.Host, request.CorvintVersion = "opencode", "1.0.0-test"
		return request
	}
	plain, err := HandleEvent(openCode(`{"changedPaths":["src/main.go"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(root, ".corvint", "self-observations.jsonl")
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Fatalf("an event without adapterCodes wrote the ledger: %v", err)
	}
	coded := openCode(`{"adapterCodes":["changed-paths-truncated","post-tool-path-not-project-relative"],"changedPaths":["src/main.go"]}`)
	for range 2 {
		result, err := HandleEvent(coded)
		if err != nil {
			t.Fatal(err)
		}
		if result["receiptId"] != plain["receiptId"] {
			t.Fatalf("adapterCodes changed the receipt: %v != %v", result["receiptId"], plain["receiptId"])
		}
		encoded, err := CanonicalJSON(result)
		if err != nil || bytes.Contains(encoded, []byte("adapterCodes")) {
			t.Fatalf("adapterCodes reached the response: %s (%v)", encoded, err)
		}
	}
	// An append slower than adapterCodeAppendBound finishes after HandleEvent returns.
	adapterCodeAppends.Wait()
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"changed-paths-truncated", "post-tool-path-not-project-relative"} {
		if want := `{"kind":"adapter-degradation","event":"post-tool","host":"opencode","adapterCodes":["` + code + `"],"window":"`; bytes.Count(data, []byte(want)) != 1 {
			t.Fatalf("ledger lacks exactly one %s row:\n%s", code, data)
		}
	}
	if bytes.Count(data, []byte("\n")) != 2 || bytes.Contains(data, []byte("src/")) {
		t.Fatalf("ledger rows are not two content-free rows:\n%s", data)
	}
}

func TestAdapterCodesAreClosedToOpenCodePathEvents(t *testing.T) {
	root := testRepository(t)
	for _, test := range []struct{ name, host, event, input string }{
		{"other-host", "codex", "post-tool", `{"adapterCodes":["changed-paths-truncated"]}`},
		{"other-event", "opencode", "stop", `{"adapterCodes":["changed-paths-truncated"]}`},
		{"unknown-code", "opencode", "post-tool", `{"adapterCodes":["src/secret.go"]}`},
		{"unsorted", "opencode", "post-tool", `{"adapterCodes":["post-tool-path-not-project-relative","changed-paths-truncated"]}`},
		{"duplicate", "opencode", "post-tool", `{"adapterCodes":["changed-paths-truncated","changed-paths-truncated"]}`},
		{"empty", "opencode", "post-tool", `{"adapterCodes":[]}`},
		{"not-a-list", "opencode", "file-change", `{"adapterCodes":"changed-paths-truncated","paths":["src/main.go"]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := request(root, test.event, test.input)
			request.Host, request.CorvintVersion = test.host, "1.0.0-test"
			if _, err := HandleEvent(request); kernelCode(err) != "invalid-harness-input" {
				t.Fatalf("code = %q, want invalid-harness-input (%v)", kernelCode(err), err)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("a refused event touched .corvint: %v", err)
	}
}
