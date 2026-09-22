package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	protocolName  = "corvint-pulse-snapshot/0"
	maxFrameBytes = 1_048_576
	maxJSONDepth  = 256
	maxPaths      = 256
	maxPathBytes  = 1_024
	maxRequestIDs = 65_536
	writeTimeout  = 5 * time.Second
)

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)
var canonicalIntegerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

type request struct {
	body      map[string]any
	clientSeq uint64
	protocol  string
	requestID string
	typeName  string
}

type protocolError struct {
	code      string
	message   string
	requestID *string
}

func (e *protocolError) Error() string { return e.message }

func canonicalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	encoded := bytes.TrimSuffix(output.Bytes(), []byte{'\n'})
	return restoreJSONSeparators(encoded), nil
}

func restoreJSONSeparators(encoded []byte) []byte {
	output := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); {
		if encoded[index] != '\\' {
			output = append(output, encoded[index])
			index++
			continue
		}
		start := index
		for index < len(encoded) && encoded[index] == '\\' {
			index++
		}
		run := index - start
		var separator []byte
		if run%2 == 1 && index+5 <= len(encoded) {
			switch string(encoded[index : index+5]) {
			case "u2028":
				separator = []byte("\u2028")
			case "u2029":
				separator = []byte("\u2029")
			}
		}
		if separator == nil {
			output = append(output, encoded[start:index]...)
			continue
		}
		output = append(output, encoded[start:index-1]...)
		output = append(output, separator...)
		index += 5
	}
	return output
}

func decodeValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, errors.New("request exceeds its nesting limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, errors.New("request is not valid JSON")
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		if number, ok := token.(json.Number); ok &&
			(!canonicalIntegerPattern.MatchString(string(number)) || string(number) == "-0") {
			return nil, errors.New("request contains a noncanonical number")
		}
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, errors.New("request is not valid JSON")
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("request object key is invalid")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate object key: %s", key)
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, errors.New("request is not valid JSON")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, errors.New("request is not valid JSON")
		}
		return array, nil
	default:
		return nil, errors.New("request is not valid JSON")
	}
}

func decodeRequest(raw []byte) (request, *protocolError) {
	var empty request
	if len(raw)+1 > maxFrameBytes {
		return empty, &protocolError{code: "resource-exhausted", message: "request frame exceeds its limit"}
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return empty, &protocolError{code: "invalid-request", message: "request is not valid UTF-8 JSON"}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0)
	if err != nil {
		message := "request is not valid JSON"
		if strings.Contains(err.Error(), "duplicate object key") {
			message = "request contains a duplicate object key"
		} else if strings.Contains(err.Error(), "noncanonical number") {
			message = "request contains a noncanonical number"
		} else if strings.Contains(err.Error(), "nesting limit") {
			return empty, &protocolError{code: "resource-exhausted", message: "request exceeds its nesting limit"}
		}
		return empty, &protocolError{code: "invalid-request", message: message}
	}
	if _, err := decoder.Token(); err != io.EOF {
		return empty, &protocolError{code: "invalid-request", message: "request contains trailing data"}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return empty, &protocolError{code: "invalid-request", message: "request must be one object"}
	}
	canonical, err := canonicalJSON(object)
	if err != nil || !bytes.Equal(raw, canonical) {
		return empty, &protocolError{code: "invalid-request", message: "request is not canonical JSON"}
	}

	requestID, requestIDOK := object["requestId"].(string)
	var errorRequestID *string
	if requestIDOK && tokenPattern.MatchString(requestID) {
		errorRequestID = &requestID
	}
	expected := map[string]struct{}{
		"body": {}, "clientSeq": {}, "protocol": {}, "requestId": {}, "type": {},
	}
	if len(object) != len(expected) {
		return empty, &protocolError{
			code: "invalid-request", message: "request contains an unknown field", requestID: errorRequestID,
		}
	}
	for field := range object {
		if _, exists := expected[field]; !exists {
			return empty, &protocolError{
				code: "invalid-request", message: "request contains an unknown field", requestID: errorRequestID,
			}
		}
	}
	body, bodyOK := object["body"].(map[string]any)
	protocol, protocolOK := object["protocol"].(string)
	typeName, typeOK := object["type"].(string)
	sequence, sequenceOK := uintField(object["clientSeq"])
	if !bodyOK || !protocolOK || !typeOK || !sequenceOK || !requestIDOK || !tokenPattern.MatchString(requestID) {
		return empty, &protocolError{code: "invalid-request", message: "request has an invalid envelope", requestID: errorRequestID}
	}
	if protocol != protocolName {
		return empty, &protocolError{code: "invalid-protocol", message: "unsupported protocol", requestID: errorRequestID}
	}
	return request{
		body: body, clientSeq: sequence, protocol: protocol, requestID: requestID, typeName: typeName,
	}, nil
}

func uintField(value any) (uint64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseUint(string(number), 10, 64)
	return parsed, err == nil && parsed > 0 && parsed < 1<<53
}

func exactFields(value map[string]any, fields ...string) bool {
	if len(value) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, exists := value[field]; !exists {
			return false
		}
	}
	return true
}

func normalizedPaths(value any) ([]string, bool, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false, false
	}
	if len(items) > maxPaths {
		return nil, false, true
	}
	paths := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		path, ok := item.(string)
		if !ok || path == "" {
			return nil, false, false
		}
		if len(path) > maxPathBytes {
			return nil, false, true
		}
		if strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || driveAbsolutePath(path) || hasControl(path) {
			return nil, false, false
		}
		for _, part := range strings.Split(path, "/") {
			if part == "" || part == "." || part == ".." {
				return nil, false, false
			}
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, false, false
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths, true, false
}

func driveAbsolutePath(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	first := value[0]
	return first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return character < 0x20 || character == 0x7f
	}) >= 0
}

func freshSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

type workspaceActor interface {
	Invalidate(context.Context, uint64) (uint64, bool, error)
	Shutdown(context.Context) error
}

type actorFactory func(string, string) (workspaceActor, error)

type server struct {
	actor            workspaceActor
	actorFactory     actorFactory
	clientSequence   uint64
	initialized      bool
	serverSequence   uint64
	seenRequestIDs   map[string]struct{}
	sessionID        string
	sessionGenerator func() (string, error)
	unknown          bool
	workspaceGen     uint64
	output           io.Writer
	root             string
}

func (s *server) nextSequence() (uint64, error) {
	if s.serverSequence == ^uint64(0) {
		return 0, errors.New("server sequence exhausted")
	}
	s.serverSequence++
	return s.serverSequence, nil
}

func (s *server) emit(typeName string, requestID any, body map[string]any) error {
	sequence, err := s.nextSequence()
	if err != nil {
		return err
	}
	session := any(nil)
	if s.initialized {
		session = s.sessionID
	}
	envelope := map[string]any{
		"body": body, "protocol": protocolName, "requestId": requestID,
		"serverSeq": sequence, "sessionId": session, "type": typeName,
	}
	raw, err := canonicalJSON(envelope)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	written, err := s.output.Write(raw)
	if err == nil && written != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

func (s *server) emitProtocolError(protocolErr *protocolError) error {
	requestID := any(nil)
	if protocolErr.requestID != nil {
		requestID = *protocolErr.requestID
	}
	return s.emit("error", requestID, map[string]any{
		"code": protocolErr.code, "message": protocolErr.message,
	})
}

func (s *server) validateClientSequence(value uint64) *protocolError {
	if value <= s.clientSequence {
		return &protocolError{code: "client-sequence-regression", message: "client sequence did not advance"}
	}
	if value != s.clientSequence+1 {
		s.unknown = true
		return &protocolError{code: "client-sequence-gap", message: "client sequence contains a gap"}
	}
	s.clientSequence = value
	return nil
}

func (s *server) handle(ctx context.Context, req request) (bool, error) {
	if s.seenRequestIDs == nil {
		s.seenRequestIDs = make(map[string]struct{})
	}
	if _, duplicate := s.seenRequestIDs[req.requestID]; duplicate {
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "duplicate-request-id", "message": "request ID was already used",
		})
	}
	if len(s.seenRequestIDs) >= maxRequestIDs {
		if s.initialized && req.typeName == "shutdown" && exactFields(req.body) {
			return s.shutdown(ctx, req.requestID)
		}
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "resource-exhausted", "message": "request ID capacity was exhausted",
		})
	}
	s.seenRequestIDs[req.requestID] = struct{}{}

	if !s.initialized {
		if req.typeName != "initialize" {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "session-not-initialized", "message": "initialize must be the first request",
			})
		}
		if req.clientSeq != 1 || !exactFields(req.body, "clientBootId", "clientId") {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "invalid-request", "message": "initialize request is invalid",
			})
		}
		clientID, clientOK := req.body["clientId"].(string)
		bootID, bootOK := req.body["clientBootId"].(string)
		if !clientOK || !bootOK || !tokenPattern.MatchString(clientID) || !tokenPattern.MatchString(bootID) {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "invalid-request", "message": "initialize request is invalid",
			})
		}
		sessionID, err := s.sessionGenerator()
		if err != nil || !tokenPattern.MatchString(sessionID) || len(sessionID) < 16 {
			return false, errors.New("cannot create session")
		}
		actor, err := s.actorFactory(s.root, sessionID)
		if err != nil {
			return false, fmt.Errorf("cannot start actor: %w", err)
		}
		s.actor = actor
		s.sessionID = sessionID
		s.initialized = true
		s.clientSequence = 1
		capabilities := map[string]any{
			"overlay.close": false, "overlay.replace": false, "shutdown": true,
			"snapshot.acquire": false, "workspace.invalidate": true,
		}
		return false, s.emit("initialized", req.requestID, map[string]any{
			"capabilities": capabilities, "workspaceGeneration": uint64(0),
		})
	}
	if s.unknown {
		if req.typeName == "shutdown" && exactFields(req.body) {
			return s.shutdown(ctx, req.requestID)
		}
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "session-unknown", "message": "session cannot continue after a sequence gap",
		})
	}

	sequenceErr := s.validateClientSequence(req.clientSeq)
	if sequenceErr != nil {
		sequenceErr.requestID = &req.requestID
		if sequenceErr.code == "client-sequence-gap" {
			if err := s.emit("workspace.unknown", nil, map[string]any{
				"reason": "client-sequence-gap", "workspaceGeneration": s.workspaceGen,
			}); err != nil {
				return false, err
			}
		}
		return false, s.emitProtocolError(sequenceErr)
	}

	switch req.typeName {
	case "initialize":
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "session-already-initialized", "message": "session is already initialized",
		})
	case "workspace.invalidate":
		return false, s.invalidate(ctx, req)
	case "snapshot.acquire":
		if !exactFields(req.body) {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "invalid-request", "message": "snapshot.acquire request is invalid",
			})
		}
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "identity-incomplete", "message": "complete WSI capture is not implemented in P0-A",
			"snapshotState": "UNKNOWN",
		})
	case "overlay.replace", "overlay.close":
		if !exactFields(req.body) {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "invalid-request", "message": req.typeName + " request is invalid",
			})
		}
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "unsupported-operation", "message": req.typeName + " is not implemented in P0-A",
		})
	case "shutdown":
		if !exactFields(req.body) {
			return false, s.emit("error", req.requestID, map[string]any{
				"code": "invalid-request", "message": "shutdown request is invalid",
			})
		}
		return s.shutdown(ctx, req.requestID)
	default:
		return false, s.emit("error", req.requestID, map[string]any{
			"code": "unsupported-operation", "message": "operation is not implemented in P0-A",
		})
	}
}

func (s *server) shutdown(ctx context.Context, requestID string) (bool, error) {
	if err := s.actor.Shutdown(ctx); err != nil {
		return false, err
	}
	return true, s.emit("shutdown.complete", requestID, map[string]any{})
}

func (s *server) invalidate(ctx context.Context, req request) error {
	if !exactFields(req.body, "expectedGeneration", "paths", "reason") {
		return s.emit("error", req.requestID, map[string]any{
			"code": "invalid-request", "message": "workspace.invalidate request is invalid",
		})
	}
	expected, ok := uintFieldAllowZero(req.body["expectedGeneration"])
	paths, pathsOK, pathsExhausted := normalizedPaths(req.body["paths"])
	reason, reasonOK := req.body["reason"].(string)
	if pathsExhausted {
		return s.emit("error", req.requestID, map[string]any{
			"code": "resource-exhausted", "message": "workspace.invalidate paths exceed their limit",
		})
	}
	if !ok || !pathsOK || !reasonOK || !tokenPattern.MatchString(reason) {
		return s.emit("error", req.requestID, map[string]any{
			"code": "invalid-request", "message": "workspace.invalidate request is invalid",
		})
	}
	newGeneration, accepted, err := s.actor.Invalidate(ctx, expected)
	if err != nil {
		return err
	}
	if !accepted {
		return s.emit("error", req.requestID, map[string]any{
			"code": "generation-mismatch", "message": "workspace generation does not match",
		})
	}
	s.workspaceGen = newGeneration
	if err := s.emit("workspace.invalidated", nil, map[string]any{
		"paths": paths, "priorWsi": nil, "reason": reason, "workspaceGeneration": newGeneration,
	}); err != nil {
		return err
	}
	return s.emit("workspace.invalidate.ack", req.requestID, map[string]any{
		"workspaceGeneration": newGeneration,
	})
}

func uintFieldAllowZero(value any) (uint64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseUint(string(number), 10, 64)
	return parsed, err == nil && parsed < 1<<53
}

type frameResult struct {
	line []byte
	err  error
}

func readFrames(ctx context.Context, input io.Reader) <-chan frameResult {
	frames := make(chan frameResult)
	go func() {
		defer close(frames)
		reader := bufio.NewReaderSize(input, maxFrameBytes)
		for {
			line, err := reader.ReadSlice('\n')
			result := frameResult{line: bytes.Clone(line), err: err}
			select {
			case frames <- result:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return frames
}

type boundedWriter struct {
	ctx    context.Context
	output io.Writer
}

type writeResult struct {
	written int
	err     error
}

func (w boundedWriter) Write(value []byte) (int, error) {
	result := make(chan writeResult, 1)
	owned := bytes.Clone(value)
	go func() {
		written, err := w.output.Write(owned)
		result <- writeResult{written: written, err: err}
	}()
	timer := time.NewTimer(writeTimeout)
	defer timer.Stop()
	select {
	case completed := <-result:
		return completed.written, completed.err
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	case <-timer.C:
		return 0, errors.New("response write timed out")
	}
}

func serve(ctx context.Context, root string, input io.Reader, output io.Writer, factory actorFactory, sessions func() (string, error)) error {
	s := &server{
		actorFactory:     factory,
		output:           boundedWriter{ctx: ctx, output: output},
		root:             root,
		sessionGenerator: sessions,
	}
	readerCtx, stopReader := context.WithCancel(ctx)
	defer stopReader()
	defer func() {
		if s.actor != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.actor.Shutdown(shutdownCtx)
		}
	}()
	frames := readFrames(readerCtx, input)
	for {
		var result frameResult
		select {
		case <-ctx.Done():
			return ctx.Err()
		case next, open := <-frames:
			if !open {
				return nil
			}
			result = next
		}
		line, err := result.line, result.err
		if errors.Is(err, io.EOF) && len(line) == 0 {
			return nil
		}
		if errors.Is(err, bufio.ErrBufferFull) || len(line) > maxFrameBytes {
			return s.emitProtocolError(&protocolError{
				code: "resource-exhausted", message: "request frame exceeds its limit",
			})
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("request stream has an incomplete frame")
		}
		raw := line[:len(line)-1]
		if bytes.HasSuffix(raw, []byte{'\r'}) {
			if emitErr := s.emitProtocolError(&protocolError{
				code: "invalid-request", message: "request framing must use one LF",
			}); emitErr != nil {
				return emitErr
			}
			continue
		}
		req, protocolErr := decodeRequest(raw)
		if protocolErr != nil {
			if err := s.emitProtocolError(protocolErr); err != nil {
				return err
			}
			continue
		}
		stop, err := s.handle(ctx, req)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}
