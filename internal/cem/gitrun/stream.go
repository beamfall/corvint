package gitrun

import (
	"errors"
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// A refused consumer keeps draining until the runner contains the process.
// Returning its error to os/exec would race a copy error into Git's exit code.
type streamOutput struct {
	consumer io.Writer
	abort    func()
	mu       sync.Mutex
	err      error
}

func (s *streamOutput) Write(data []byte) (int, error) {
	if s.failure() != nil {
		return len(data), nil
	}
	n, err := s.consumer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		var typed *cemcode.Error
		if !errors.As(err, &typed) {
			err = cemcode.New(cemcode.GitReadFailed, "Git stdout consumer failed: %v", err)
		}
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		s.abort()
	}
	return len(data), nil
}

func (s *streamOutput) failure() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
