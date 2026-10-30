package opencodequalification

import (
	"strings"
	"testing"
)

func TestAHI032HiddenPromptDelivery(t *testing.T) {
	t.Run("AHI-032 hidden prompt delivery", func(t *testing.T) {
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
	})
}

func TestAHI032UnchangedNativePromptIsBound(t *testing.T) {
	t.Run("AHI-032 unchanged native prompt is bound", func(t *testing.T) {
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
	})
}

func latencyRows(query, lifecycle []float64) []Object {
	rows := []Object{}
	for _, ms := range query {
		rows = append(rows, Object{"kind": "session.prompt", "ms": ms, "delivered": true})
	}
	for _, ms := range lifecycle {
		rows = append(rows, Object{"kind": "event.session.created", "ms": ms})
	}
	return rows
}

// series returns a warm-up sample followed by n samples, the first `high` of them at highMs.
func series(n, high int, baseMs, highMs float64) []float64 {
	out := []float64{9999}
	for i := 0; i < n; i++ {
		v := baseMs
		if i < high {
			v = highMs
		}
		out = append(out, v)
	}
	return out
}

func TestAHI056LatencyResample(t *testing.T) {
	t.Run("AHI-056 loaded-host sample passes on its single re-sample", func(t *testing.T) {
		// AHI-056: transient load inflating the first ~20 samples must not decide the verdict alone.
		loaded, delivered := timingSample(latencyRows(series(30, 3, 300, 438), series(30, 20, 200, 299)))
		if !delivered || len(loaded.query) != 30 || len(loaded.lifecycle) != 30 {
			t.Fatalf("warm-up sample not dropped or delivery lost: %+v %v", loaded, delivered)
		}
		if got := latencyFailures(loaded); len(got) != 1 || got[0] != "lifecycle p95 299ms > 250ms" {
			t.Fatalf("loaded failures = %v", got)
		}
		if !latencyResample(loaded) {
			t.Fatal("loaded p95-only miss was not re-sampled")
		}
		calm, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 0, 216, 0)))
		if got := latencyFailures(calm); len(got) != 0 {
			t.Fatalf("calm re-sample failed: %v", got)
		}
	})
	t.Run("AHI-056 a real regression still fails", func(t *testing.T) {
		// AHI-056: a steady slowdown misses both attempts; samples below the minimum are never re-sampled.
		slow, _ := timingSample(latencyRows(series(30, 30, 0, 612), series(30, 30, 0, 290)))
		if !latencyResample(slow) {
			t.Fatal("p95 miss not eligible for re-sample")
		}
		again, _ := timingSample(latencyRows(series(30, 30, 0, 605), series(30, 30, 0, 288)))
		if got := latencyFailures(again); len(got) != 2 {
			t.Fatalf("regressed re-sample passed: %v", got)
		}
		short, _ := timingSample(latencyRows(series(10, 0, 100, 0), series(30, 0, 100, 0)))
		if latencyResample(short) || len(latencyFailures(short)) != 1 {
			t.Fatalf("short sample: resample=%v failures=%v", latencyResample(short), latencyFailures(short))
		}
	})
	t.Run("AHI-056 failure message names checks with measured values and limits", func(t *testing.T) {
		// AHI-056: the campaign failure line names each failed check and every latency attempt's values.
		first, _ := timingSample(latencyRows(series(30, 30, 0, 612), series(30, 30, 0, 299)))
		second, _ := timingSample(latencyRows(series(30, 0, 300, 0), series(30, 30, 0, 260)))
		checks := Object{"latency": false, "native-edit": false, "native-query": true}
		got := campaignFailure(checkFailures(checks, nil, []latencySample{first, second}), "/run")
		// AHI-213: every other failed check also carries its named reason.
		want := "native campaign failed: latency (attempt 1: query p95 612ms > 500ms, lifecycle p95 299ms > 250ms; attempt 2: lifecycle p95 260ms > 250ms); native-edit (edit-not-applied); inspect /run"
		if got != want {
			t.Fatalf("message\n got %q\nwant %q", got, want)
		}
	})
}

func TestBundledSkillAdvertised(t *testing.T) {
	t.Run("AHI-176 provider request advertises the plugin-registered corvint skill", func(t *testing.T) {
		// AHI-176: OpenCode's available-skills guidance counts only in a system/developer message;
		// AHI-212: its appended skill-change notice also counts in a user-role message.
		guidance := "<available_skills><skill><id>corvint</id><name>corvint</name></skill></available_skills>"
		delta := skillDeltaHeader + "  <skill>\n    <id>corvint</id>\n    <name>corvint</name>\n  </skill>"
		for _, tc := range []struct {
			name     string
			messages []any
			want     bool
		}{
			{"string system", []any{Object{"role": "system", "content": "intro\n" + guidance}}, true},
			{"text part developer", []any{Object{"role": "developer", "content": []any{Object{"type": "text", "text": guidance}}}}, true},
			{"user message only", []any{Object{"role": "user", "content": guidance}}, false},
			{"other skill", []any{Object{"role": "system", "content": strings.ReplaceAll(guidance, "<id>corvint</id>", "<id>other</id>")}}, false},
			{"id outside guidance", []any{Object{"role": "system", "content": "<id>corvint</id><available_skills></available_skills>"}}, false},
			{"missing", []any{Object{"role": "system", "content": "instructions"}}, false},
			// AHI-212: registered on the first corvint_context call, the skill arrives in the appended notice.
			{"appended notice user", []any{Object{"role": "user", "content": []any{Object{"type": "text", "text": "The Code Mode tool catalog has changed.\n\n" + delta}}}}, true},
			{"appended notice system", []any{Object{"role": "system", "content": delta}}, true},
			{"appended other skill", []any{Object{"role": "user", "content": strings.ReplaceAll(delta, "<id>corvint</id>", "<id>other</id>")}}, false},
			{"id after appended list", []any{Object{"role": "user", "content": skillDeltaHeader + "  <skill>\n    <id>other</id>\n  </skill>\n\nquoted:\n    <id>corvint</id>"}}, false},
			{"assistant echo", []any{Object{"role": "assistant", "content": delta}}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := bundledSkillAdvertised([]Object{{"messages": tc.messages}}); got != tc.want {
					t.Fatalf("bundledSkillAdvertised = %v, want %v", got, tc.want)
				}
			})
		}
	})
}

func TestAHI219FrameReceiptLink(t *testing.T) {
	t.Run("AHI-219 the frame link is short, derived from the receipt, and binds delivery", func(t *testing.T) {
		receipt := "harness-receipt:sha256:" + strings.Repeat("ab", 32)
		link := receiptLink(receipt)
		if link != "cvr:"+strings.Repeat("ab", 12) || len(link) > 32 || !frameLinkPattern.MatchString(link) {
			t.Fatalf("link %q", link)
		}
		for _, bad := range []string{"", "harness-receipt:sha256:" + strings.Repeat("A", 64), "harness-receipt:sha256:abc", "other:" + strings.Repeat("a", 64)} {
			if got := receiptLink(bad); got != "" {
				t.Fatalf("receiptLink(%q) = %q", bad, got)
			}
		}
		frame := "BEGIN CORVINT REPOSITORY DATA\n{\"receipt\":\"" + link + "\"}\nEND CORVINT REPOSITORY DATA"
		events := []Object{{"kind": "context", "system": []any{Object{"type": "text", "text": frame}}}}
		messages := []any{Object{"role": "system", "content": frame}, Object{"role": "user", "content": "task"}}
		if _, ok := hiddenPromptDelivery("task", events, messages, link); !ok {
			t.Fatal("frame carrying the query's link refused")
		}
		other := receiptLink("harness-receipt:sha256:" + strings.Repeat("cd", 32))
		if _, ok := hiddenPromptDelivery("task", events, messages, other); ok {
			t.Fatal("frame accepted for another query's link")
		}
	})
}
