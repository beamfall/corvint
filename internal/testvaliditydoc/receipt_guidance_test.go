// SPDX-License-Identifier: Apache-2.0
package testvaliditydoc

import (
	"strings"
	"testing"
)

func TestLPCVV07705PlainPlaywrightReceiptGuidance(t *testing.T) {
	_, err := Decode([]byte(`{"config":{"version":"1.62.1"},"suites":[],"errors":[],"stats":{}}`))
	if err == nil {
		t.Fatal("plain report admitted as provider receipt")
	}

	const want = "plain Playwright JSON report is not a provider receipt; rerun with the qualified reporter through the provider (there is no evidence-preserving conversion of an ordinary report); accepted --receipt kinds: corvint-js-test-provider document (receipt.kind unit or e2e, including qualified corvint-playwright-external profiles), completed corvint-go-live-session-event/0 from corvint-go-test-provider, or corvint-ruby-test-provider/0 from corvint-ruby-test-provider. Produce JavaScript receipts with corvint-js-test-provider unit or e2e; for Playwright use corvint-js-test-provider e2e --external-server --config FILE (the provider installs qualified-reporter.cjs and binds its observations)."
	if err.Error() != want {
		t.Fatalf("refusal text drift:\n got %s\nwant %s", err.Error(), want)
	}

	_, err = Decode([]byte(`{"profile":"unknown"}`))
	if err == nil || !strings.Contains(err.Error(), "accepted") || strings.Contains(err.Error(), "plain Playwright") {
		t.Fatalf("unknown kind: %v", err)
	}
}
