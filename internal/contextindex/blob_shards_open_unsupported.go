//go:build !darwin && !linux

package contextindex

import (
	"errors"
	"os"
)

func openBlobShard(root, target string) (*os.File, error) {
	return nil, errors.New("nonblocking confined shard reads unsupported on this platform")
}

func publishBlobFact(root, target string, data []byte) error {
	return errors.New("confined shard publication unsupported on this platform")
}
