package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProviderKitProducerRefusals(t *testing.T) {
	args := []string{"--provider-version", "0.1.0", "--profile", "external-evidence-provider/0", "--revision", strings.Repeat("a", 40), "--path", "pkg/main.go"}
	for name, arguments := range map[string][]string{
		"valid":                args,
		"missing-pins":         {},
		"unsupported-profile":  append(append([]string{}, args...), "--profile", "external-evidence-provider/3"),
		"unsupported-version":  append(append([]string{}, args...), "--provider-version", "0.2.0"),
		"invalid-path":         append(append([]string{}, args...), "--path", "../private"),
		"origin-in-v0":         append(append([]string{}, args...), "--origin", strings.Repeat("a", 40)),
		"missing-origin-in-v1": append(append([]string{}, args...), "--profile", "external-evidence-provider/1"),
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(arguments, &out)
			if name == "valid" {
				if err != nil || out.Len() == 0 {
					t.Fatalf("valid: %v", err)
				}
				return
			}
			if err == nil || out.Len() != 0 {
				t.Fatalf("refusal must emit no record: %v %s", err, out.Bytes())
			}
		})
	}
}
