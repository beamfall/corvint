package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNavigationFlagsCannotGrantLegacyObserver(t *testing.T) {
	for _, extra := range [][]string{{"--max-effect", "read"}, {"--timeout", "1s"}, {"--navigation", "packet.json"}} {
		t.Run(strings.Join(extra, " "), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			args := append([]string{"--experimental", "--trusted-local", "--observe", "--manifest", "manifest.json", "--assets", "assets"}, extra...)
			if code := run(context.Background(), args, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "navigation requires") || out.Len() != 0 {
				t.Fatalf("code=%d diagnostic=%s", code, diagnostic.String())
			}
		})
	}
}
