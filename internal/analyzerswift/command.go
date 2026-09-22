package analyzerswift

import (
	"bytes"
	"errors"
	"io"
)

// Run is the public command boundary. It accepts only one descriptor-backed
// request file so every public execution receives the same no-follow identity
// checks before the pure envelope analyzer runs.
func Run(args []string, writer io.Writer) int {
	response := sentinelBytes()
	if len(args) == 2 && args[0] == "--request-file" {
		response = AnalyzeDescriptor(args[1])
	}
	if writeFull(writer, response) != nil {
		return 1
	}
	if bytes.Contains(response, []byte(`"status":"CANDIDATE"`)) {
		return 0
	}
	return 1
}

func writeFull(writer io.Writer, value []byte) error {
	written, err := writer.Write(value)
	if err != nil {
		return err
	}
	if written != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

// AnalyzeDescriptor safely acquires one request descriptor before delegating
// only immutable bytes to Analyze.
func AnalyzeDescriptor(path string) []byte {
	value, err := readDescriptor(path)
	if err != nil {
		return sentinelBytes()
	}
	return analyzeWire(value)
}

var errDescriptorUnsafe = errors.New("unsafe request descriptor")
