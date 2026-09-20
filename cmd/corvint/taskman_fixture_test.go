package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNTPV0008CommandBoundary(t *testing.T) {
	t.Run("NTP-V0-008 explicit fixture command", func(t *testing.T) {
		for _, args := range [][]string{
			{"plan-fixture"},
			{"plan-fixture", "--executor", "relative", "--observations", "missing"},
			{"plan-fixture", "--executor", "/missing", "--executor", "/another"},
			{"plan-fixture", "--unknown", "value"},
		} {
			var out, err bytes.Buffer
			if code := runWork(context.Background(), t.TempDir(), args, &out, &err); code != 2 || out.Len() != 0 || !strings.HasPrefix(err.String(), "taskman fixture:") {
				t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), err.String())
			}
		}
		if _, err := parseWorkOptions([]string{"observe"}); err != nil {
			t.Fatal(err)
		}
		if _, err := parseWorkOptions([]string{"observe", "--executor", "/fixture"}); err == nil {
			t.Fatal("native fixture flags admitted to WQO")
		}
	})
}
