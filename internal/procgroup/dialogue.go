package procgroup

import (
	"io"
	"os"
	"time"
)

type processDialogue struct {
	inputReader, inputWriter *os.File
	outputReader             *io.PipeReader
	outputWriter             *io.PipeWriter
	done                     chan error
	failure                  chan struct{}
}

func newProcessDialogue(exchange func(io.Reader, io.WriteCloser) error) (*processDialogue, error) {
	if exchange == nil {
		return nil, nil
	}
	in, out, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	reader, writer := io.Pipe()
	return &processDialogue{in, out, reader, writer, make(chan error, 1), make(chan struct{}, 1)}, nil
}

func (d *processDialogue) start(exchange func(io.Reader, io.WriteCloser) error) {
	if d == nil {
		return
	}
	go func() {
		err := exchange(d.outputReader, d.inputWriter)
		_ = d.inputWriter.Close()
		if err != nil {
			d.failure <- struct{}{}
		}
		d.done <- err
	}()
}

func (d *processDialogue) destination(capture io.Writer) io.Writer {
	if d == nil {
		return capture
	}
	return &dialogueOutput{capture, d.outputWriter}
}

// Closing the forwarded pipe on EOF wakes a dialogue waiting for clean EOF.
type dialogueOutput struct {
	capture io.Writer
	pipe    *io.PipeWriter
}

func (d *dialogueOutput) Write(p []byte) (int, error) {
	if _, err := d.capture.Write(p); err != nil {
		return 0, err
	}
	return d.pipe.Write(p)
}
func (d *dialogueOutput) Close() error { return d.pipe.Close() }

func (d *processDialogue) failed() <-chan struct{} {
	if d == nil {
		return nil
	}
	return d.failure
}
func (d *processDialogue) finish(exited bool, timeout time.Duration) error {
	if exited {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-d.done:
			d.close()
			return err
		case <-timer.C:
		}
	}
	d.close()
	return <-d.done
}
func (d *processDialogue) close() {
	if d == nil {
		return
	}
	_ = d.inputReader.Close()
	_ = d.inputWriter.Close()
	_ = d.outputReader.Close()
	_ = d.outputWriter.Close()
}
