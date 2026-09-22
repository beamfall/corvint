package main

import (
	"bytes"
	"context"
	"testing"
)

func TestRemoteCLI(t *testing.T) {
	t.Run("EEP-REMOTE-001 explicit consent", func(t *testing.T) {
		for _, args := range [][]string{nil, {"--config", "missing"}, {"--allow-network"}, {"--allow-network", "--config", "missing"}} {
			var stdout, stderr bytes.Buffer
			if run(context.Background(), args, &stdout, &stderr) == 0 || stdout.Len() != 0 {
				t.Fatal("invalid invocation published stdout")
			}
		}
	})
}
