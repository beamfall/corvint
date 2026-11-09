// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"
)

func TestDTLV0009TestFormatJSON(t *testing.T) {
	t.Run("DTL-V0-009", func(t *testing.T) {
		for _, args := range [][]string{{"--base", "HEAD", "--dry-run", "--format", "json"}, {"--base", "HEAD", "--format=json"}} {
			options, err := parseTestOptions(args)
			if err != nil {
				t.Fatal(err)
			}
			if !options.full || testSummarySelected(options) {
				t.Fatalf("explicit JSON selected a summary: %v", args)
			}
		}
		for _, args := range [][]string{{"--base", "HEAD", "--format", "xml"}, {"--base", "HEAD", "--format=json", "--dry-run", "--summary"}, {"--base", "HEAD", "--format=json", "--format=json"}} {
			if _, err := parseTestOptions(args); err == nil {
				t.Fatalf("admitted %v", args)
			}
		}
	})
}
