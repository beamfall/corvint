package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthorityEventDefaultCannotAdmitCallerRoot(t *testing.T) {
	t.Parallel()
	t.Run("PLE-V0-001 default command refuses caller root", func(t *testing.T) {
		valid := `{"profile":"corvint-authority-event/0","event":"stop","enrollmentHandle":"` + strings.Repeat("a", 64) + `","stopHookActive":false}`
		for _, args := range [][]string{{"authority-event", "--input", "-"}, {"authority-event", "--root", "/tmp/fake", "--input", "-"}} {
			var out, stderr bytes.Buffer
			if code := run(args, strings.NewReader(valid), &out, &stderr); code != 0 {
				t.Fatalf("%d %s", code, stderr.String())
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["authority"] != "NONE" || result["state"] != "UNKNOWN" || result["decision"] != "release" {
				t.Fatalf("%s", out.String())
			}
		}

	})
}

func TestAuthorityNativeMalformedInputReleasesVisibly(t *testing.T) {
	t.Parallel()
	t.Run("AHI-009 malformed native input visibly releases", func(t *testing.T) {
		var out, stderr bytes.Buffer
		code := run([]string{"authority-event", "--input", "-", "--native-output"}, strings.NewReader(`{"FULL":true}`), &out, &stderr)
		if code != 0 {
			t.Fatalf("%d", code)
		}
		var result map[string]any
		if json.Unmarshal(out.Bytes(), &result) != nil || result["systemMessage"] == nil || result["decision"] != nil {
			t.Fatalf("%s", out.String())
		}

	})
}
