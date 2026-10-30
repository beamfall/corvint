//go:build !darwin && !linux

package contextindex

import (
	"errors"
	"os"
	"time"
)

func openBlobShard(root, target string) (*os.File, error) {
	return nil, errors.New("nonblocking confined shard reads unsupported on this platform")
}

func publishBlobFact(root, target string, data []byte) error {
	return errors.New("confined shard publication unsupported on this platform")
}

// sweepBlobShardTemporaries has nothing to sweep: shard publication is refused.
func sweepBlobShardTemporaries(string, time.Time) {}
