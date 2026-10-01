package opencodequalification

import (
	"strings"
	"testing"
)

func TestAHI032HiddenPromptDelivery(t *testing.T) {
	// AHI-032: qualification must observe the same hidden frame at the hook and provider boundary.
	receipt := "harness-receipt:sha256:" + strings.Repeat("a", 64)
	frame := "BEGIN CORVINT REPOSITORY DATA\n" + receipt + "\nEND CORVINT REPOSITORY DATA"
	events := []Object{{"kind": "context", "system": []any{Object{"type": "text", "text": frame}}}}
	for _, tc := range []struct {
		name, prompt string
		system       any
		user         string
		eventFrame   string
		want         bool
	}{
		{"string system", "task", "instructions\n" + frame, "task", frame, true},
		{"text part system", "task", []any{Object{"type": "text", "text": frame}}, "task", frame, true},
		{"visible frame", "task\n" + frame, frame, "task\n" + frame, frame, false},
		{"leaked user context", "task", frame, "task\n" + frame, frame, false},
		{"missing provider frame", "task", "instructions", "task", frame, false},
		{"different receipt", "task", strings.ReplaceAll(frame, receipt, "wrong-receipt"), "task", frame, false},
		{"oversized hook frame", "task", frame + strings.Repeat("x", 8000), "task", frame + strings.Repeat("x", 8000), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events[0]["system"] = []any{Object{"type": "text", "text": tc.eventFrame}}
			messages := []any{Object{"role": "system", "content": tc.system}, Object{"role": "user", "content": tc.user}}
			bytes, ok := hiddenPromptDelivery(tc.prompt, events, messages, receipt)
			if ok != tc.want || (ok && bytes != len(tc.eventFrame)) {
				t.Fatalf("bytes=%d delivered=%v want=%v", bytes, ok, tc.want)
			}
		})
	}
}

func TestAHI032UnchangedNativePromptIsBound(t *testing.T) {
	// AHI-032: a later unchanged sample cannot satisfy the provider-bound prompt's observation.
	prompt := Object{"sessionID": "first-session", "messageID": "first-message"}
	timings := []Object{
		{"kind": "session.prompt", "sessionID": "first-session", "messageID": "first-message", "unchanged": false, "delivered": true},
		{"kind": "session.prompt", "sessionID": "later-session", "messageID": "later-message", "unchanged": true, "delivered": true},
	}
	if unchangedPromptObservation(prompt, timings) {
		t.Fatal("unrelated unchanged timing masked changed native prompt")
	}
	timings[0]["unchanged"] = true
	if !unchangedPromptObservation(prompt, timings) {
		t.Fatal("matching unchanged observation refused")
	}
	delete(prompt, "messageID")
	if unchangedPromptObservation(prompt, timings) {
		t.Fatal("missing identity accepted")
	}
}
