// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHostEnvelopeCompleteEnvironment(t *testing.T) {
	want := hostExpectedEnvironment()
	env := []string{}
	for _, v := range want {
		env = append(env, v.Key+"="+v.Value)
	}
	observed, err := hostObserveEnvironment(env)
	if err != nil || !hostEnvironmentMatches(observed, want) {
		t.Fatal("valid complete environment refused", err)
	}
	for name, values := range map[string][]string{
		"PCH-V0-011-extra-image-key":  append(append([]string{}, env...), "IMAGE_EXTRA=not-authorized"),
		"PCH-V0-011-extra-credential": append(append([]string{}, env...), "GITHUB_TOKEN=private-value"),
		"PCH-V0-011-missing-key":      env[:2],
		"PCH-V0-011-wrong-value":      {"HOME=/private/other", "TMPDIR=/private/tmp", "PATH=/usr/local/go/bin:/usr/bin:/bin"},
		"PCH-V0-011-duplicate":        append(append([]string{}, env...), env[0]),
		"PCH-V0-011-malformed":        {"no-equals"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := hostObserveEnvironment(values)
			if err == nil && hostEnvironmentMatches(got, want) {
				t.Fatal("incomplete or unapproved environment accepted")
			}
			raw, _ := json.Marshal(got)
			if strings.Contains(string(raw), "private-value") {
				t.Fatal("unknown credential value emitted")
			}
		})
	}
	if hostEnvironmentMatches(nil, []HostEnvironment{}) {
		t.Fatal("missing observation treated as empty")
	}
}
