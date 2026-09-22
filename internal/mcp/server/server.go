// Package server implements the local child-owned MCP 2026-07-28 stdio server.
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

const defaultMaxConcurrent = 64

type Config struct {
	ProtocolVersion string
	Name            string
	Version         string
	Description     string
	Instructions    string
	Capabilities    map[string]any
	DiscoveryTTL    time.Duration
	MaxConcurrent   int
	Handler         Handler
}

type Handler interface {
	Handle(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError)
}

type HandlerFunc func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError)

func (function HandlerFunc) Handle(ctx context.Context, request protocol.Request, notifier Notifier) (map[string]any, *protocol.RPCError) {
	return function(ctx, request, notifier)
}

type Notifier interface {
	Progress(progress float64, total *float64, message string) error
	Log(level, logger string, data any) error
}

type Server struct {
	config Config
}

func New(config Config) (*Server, error) {
	if config.ProtocolVersion == "" {
		config.ProtocolVersion = protocol.Version
	}
	if config.ProtocolVersion != protocol.Version && config.ProtocolVersion != protocol.LegacyVersion {
		return nil, fmt.Errorf("mcp protocol version is unsupported")
	}
	if config.Name == "" || config.Version == "" || config.Handler == nil {
		return nil, fmt.Errorf("mcp server requires identity and handler")
	}
	if config.DiscoveryTTL < 0 {
		return nil, fmt.Errorf("mcp discovery TTL must not be negative")
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = defaultMaxConcurrent
	}
	if config.MaxConcurrent < 1 || config.MaxConcurrent > 4096 {
		return nil, fmt.Errorf("mcp concurrency bound is invalid")
	}
	config.Capabilities = cloneMap(config.Capabilities)
	return &Server{config: config}, nil
}

type connection struct {
	config Config
	ctx    context.Context
	output io.Writer
	write  sync.Mutex
	active sync.Map
	limit  chan struct{}
	wait   sync.WaitGroup
	cancel context.CancelFunc

	legacyState legacyState
	legacyMeta  protocol.RequestMeta

	errorMu sync.Mutex
	first   error
}

type activeRequest struct {
	key    string
	cancel context.CancelFunc
	mu     sync.Mutex
	state  requestState
}

type requestState uint8

const (
	requestPending requestState = iota
	requestResponding
	requestCancelled
)

func (request *activeRequest) claimResponse() bool {
	request.mu.Lock()
	defer request.mu.Unlock()
	if request.state != requestPending {
		return false
	}
	request.state = requestResponding
	return true
}

func (request *activeRequest) cancelIfPending() {
	request.mu.Lock()
	defer request.mu.Unlock()
	if request.state != requestPending {
		return
	}
	request.state = requestCancelled
	request.cancel()
}

func (server *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if input == nil || output == nil {
		return fmt.Errorf("mcp stdio requires input and output")
	}
	serveCtx, cancel := context.WithCancel(ctx)
	connection := &connection{
		config: server.config, ctx: serveCtx, output: output,
		limit: make(chan struct{}, server.config.MaxConcurrent), cancel: cancel,
	}
	restoreInput, restoreOutput := preserveBlockingMode(input), preserveBlockingMode(output)
	stopTransportWatch := watchTransportCancellation(serveCtx, input, output)
	defer func() {
		cancel()
		connection.active.Range(func(_, value any) bool {
			value.(*activeRequest).cancel()
			return true
		})
		connection.wait.Wait()
		stopTransportWatch()
		restoreOutput()
		restoreInput()
	}()

	reader := bufio.NewReaderSize(interruptibleReader(serveCtx, input), 64<<10)
	for {
		frame, state, err := readFrame(reader)
		if err != nil {
			if first := connection.err(); first != nil {
				return first
			}
			if serveCtx.Err() != nil {
				return serveCtx.Err()
			}
			return err
		}
		switch state {
		case frameEOF:
			// A clean stdin close stops admission but does not revoke requests
			// already accepted from the client. Let those responses flush.
			connection.wait.Wait()
			return connection.err()
		case frameUnterminated:
			// A torn final frame is still an EOF: report it once, then drain
			// requests already admitted exactly as a clean EOF does.
			if writeErr := connection.writeRPCError(nil, &protocol.RPCError{Code: protocol.CodeParseError, Message: "Parse error"}); writeErr != nil {
				return writeErr
			}
			connection.wait.Wait()
			return connection.err()
		case frameOversized:
			if writeErr := connection.writeRPCError(nil, &protocol.RPCError{Code: protocol.CodeParseError, Message: "Parse error"}); writeErr != nil {
				return writeErr
			}
			continue
		}
		message, decodeErr := protocol.Decode(frame)
		if decodeErr != nil {
			if writeErr := connection.writeRPCError(decodeErr.ID, &protocol.RPCError{Code: decodeErr.Code, Message: decodeErr.Message}); writeErr != nil {
				return writeErr
			}
			continue
		}
		if message.Notification {
			connection.handleNotification(message)
			continue
		}
		select {
		case connection.limit <- struct{}{}:
			requestCtx, requestCancel := context.WithCancel(serveCtx)
			active := &activeRequest{key: message.ID.Key(), cancel: requestCancel}
			if _, loaded := connection.active.LoadOrStore(message.ID.Key(), active); loaded {
				requestCancel()
				<-connection.limit
				// Omit the ID: echoing it would let the client correlate this
				// rejection with the request still in flight under that ID.
				if err := connection.writeRPCError(nil, &protocol.RPCError{Code: protocol.CodeInvalidRequest, Message: "Invalid Request"}); err != nil {
					return err
				}
				continue
			}
			connection.wait.Add(1)
			// Initialization commits at input admission, after its response is
			// written. Later notifications cannot race ahead of that write.
			ready := connection.legacyState == legacyReady
			if connection.config.ProtocolVersion == protocol.LegacyVersion && message.Method == "initialize" {
				connection.handleRequest(requestCtx, requestCancel, active, message, ready)
				continue
			}
			go connection.handleRequest(requestCtx, requestCancel, active, message, ready)
		default:
			if err := connection.writeRPCError(message.ID, &protocol.RPCError{Code: protocol.CodeInternalError, Message: "Internal error"}); err != nil {
				return err
			}
		}
	}
}

func watchTransportCancellation(ctx context.Context, input io.Reader, output io.Writer) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-done:
			return
		case <-ctx.Done():
		}
		deadline := time.Now()
		if requiresExternalCancellation(input) {
			if setter, ok := input.(interface{ SetReadDeadline(time.Time) error }); ok {
				_ = setter.SetReadDeadline(deadline)
			}
			if closer, ok := input.(io.Closer); ok {
				_ = closer.Close()
			}
		}
		if requiresExternalCancellation(output) {
			if setter, ok := output.(interface{ SetWriteDeadline(time.Time) error }); ok {
				_ = setter.SetWriteDeadline(deadline)
			}
			if closer, ok := output.(io.Closer); ok {
				_ = closer.Close()
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

type frameState uint8

const (
	frameComplete frameState = iota
	frameOversized
	frameUnterminated
	frameEOF
)

func readFrame(reader *bufio.Reader) ([]byte, frameState, error) {
	frame := make([]byte, 0, 4096)
	oversized := false
	for {
		part, err := reader.ReadSlice('\n')
		if !oversized {
			remaining := protocol.MaxMessageBytes + 1 - len(frame)
			if remaining > 0 {
				if len(part) < remaining {
					frame = append(frame, part...)
				} else {
					frame = append(frame, part[:remaining]...)
					if len(part) > remaining {
						oversized = true
					}
				}
			}
			if len(frame) > protocol.MaxMessageBytes+1 {
				oversized = true
			}
		}
		if err == nil {
			if oversized {
				return nil, frameOversized, nil
			}
			frame = frame[:len(frame)-1]
			if len(frame) > protocol.MaxMessageBytes {
				return nil, frameOversized, nil
			}
			return frame, frameComplete, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			if len(frame) > protocol.MaxMessageBytes {
				oversized = true
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(frame) == 0 && len(part) == 0 {
				return nil, frameEOF, nil
			}
			return nil, frameUnterminated, nil
		}
		return nil, frameEOF, err
	}
}

// legacyMethods are the pre-2026-07-28 lifecycle requests MCPV0-006 answers with
// method-not-found. A legacy client never sends 2026-07-28 request metadata, so
// they are matched before that metadata is required.
var legacyMethods = map[string]bool{
	"initialize": true, "ping": true, "logging/setLevel": true,
	"resources/subscribe": true, "resources/unsubscribe": true,
}

func (connection *connection) handleRequest(requestCtx context.Context, cancel context.CancelFunc, active *activeRequest, inbound protocol.Inbound, legacyReady bool) {
	defer connection.wait.Done()
	defer func() { <-connection.limit }()
	defer func() {
		connection.active.CompareAndDelete(active.key, active)
		cancel()
	}()

	if connection.config.ProtocolVersion == protocol.LegacyVersion {
		connection.handleLegacyRequest(requestCtx, active, inbound, legacyReady)
		return
	}
	if legacyMethods[inbound.Method] {
		connection.respond(active, func() error {
			return connection.writeRPCErrorContext(requestCtx, inbound.ID, protocol.MethodNotFound())
		})
		return
	}
	meta, rpcErr := protocol.DecodeRequestMeta(inbound.Params)
	if rpcErr != nil {
		connection.respond(active, func() error { return connection.writeRPCErrorContext(requestCtx, inbound.ID, rpcErr) })
		return
	}
	if inbound.Method == "server/discover" && !onlyParams(inbound.Params, "_meta") {
		connection.respond(active, func() error {
			return connection.writeRPCErrorContext(requestCtx, inbound.ID, protocol.InvalidParams("Invalid params"))
		})
		return
	}
	if meta.ProtocolVersion != protocol.Version {
		connection.respond(active, func() error {
			return connection.writeRPCErrorContext(requestCtx, inbound.ID, &protocol.RPCError{
				Code: protocol.CodeUnsupportedProtocolVersion, Message: "Unsupported protocol version",
				Data: map[string]any{"requested": meta.ProtocolVersion, "supported": []string{protocol.Version}},
			})
		})
		return
	}

	request := protocol.Request{ID: *inbound.ID, Method: inbound.Method, Params: inbound.Params, Meta: meta}
	if request.Method == "server/discover" {
		connection.respond(active, func() error {
			return connection.writeResultContext(requestCtx, request.ID, connection.discoverResult())
		})
		return
	}
	notifier := &requestNotifier{connection: connection, ctx: requestCtx, meta: meta}
	result, rpcErr := connection.config.Handler.Handle(requestCtx, request, notifier)
	if rpcErr != nil {
		connection.respond(active, func() error {
			return connection.writeRPCErrorContext(requestCtx, &request.ID, sanitize(rpcErr))
		})
		return
	}
	if requestCtx.Err() != nil {
		return
	}
	connection.respond(active, func() error {
		return connection.writeResultContext(requestCtx, request.ID, connection.decorateResult(result))
	})
}

// respond takes the request out of flight before its frame can reach the client
// (MCPV0-004), so a client reusing the ID after reading the response is admitted.
func (connection *connection) respond(active *activeRequest, write func() error) {
	if !active.claimResponse() {
		return
	}
	connection.active.CompareAndDelete(active.key, active)
	connection.record(write())
}

func onlyParams(params map[string]any, fields ...string) bool {
	if len(params) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, present := params[field]; !present {
			return false
		}
	}
	return true
}

func (connection *connection) handleNotification(inbound protocol.Inbound) {
	if connection.config.ProtocolVersion == protocol.LegacyVersion && inbound.Method == "notifications/initialized" {
		if len(inbound.Params) == 0 && connection.legacyState == legacyInitialized {
			connection.legacyState = legacyReady
		}
		return
	}
	if inbound.Method != "notifications/cancelled" || inbound.Params == nil {
		return
	}
	id, ok := protocol.ParseIDValue(inbound.Params["requestId"])
	if !ok {
		return
	}
	if reason, present := inbound.Params["reason"]; present {
		if _, ok := reason.(string); !ok {
			return
		}
	}
	if value, exists := connection.active.Load(id.Key()); exists {
		// Arbitration is per request and never waits behind a blocked transport.
		// A response that claimed the request first wins; otherwise cancellation
		// prevents any later response or progress notification.
		value.(*activeRequest).cancelIfPending()
	}
}

func (connection *connection) discoverResult() map[string]any {
	result := map[string]any{
		"_meta":      map[string]any{"io.modelcontextprotocol/serverInfo": connection.serverInfo()},
		"cacheScope": "public", "capabilities": cloneMap(connection.config.Capabilities),
		"resultType": "complete", "supportedVersions": []string{protocol.Version},
		"ttlMs": connection.config.DiscoveryTTL.Milliseconds(),
	}
	if connection.config.Instructions != "" {
		result["instructions"] = connection.config.Instructions
	}
	return result
}

func (connection *connection) decorateResult(result map[string]any) map[string]any {
	if result == nil {
		result = make(map[string]any)
	} else {
		result = cloneMap(result)
	}
	if _, exists := result["resultType"]; !exists {
		result["resultType"] = "complete"
	}
	meta, _ := result["_meta"].(map[string]any)
	meta = cloneMap(meta)
	meta["io.modelcontextprotocol/serverInfo"] = connection.serverInfo()
	result["_meta"] = meta
	return result
}

func (connection *connection) serverInfo() map[string]any {
	info := map[string]any{"name": connection.config.Name, "version": connection.config.Version}
	if connection.config.Description != "" {
		info["description"] = connection.config.Description
	}
	return info
}

func sanitize(rpcErr *protocol.RPCError) *protocol.RPCError {
	if rpcErr == nil {
		return &protocol.RPCError{Code: protocol.CodeInternalError, Message: "Internal error"}
	}
	switch rpcErr.Code {
	case protocol.CodeInvalidRequest:
		return &protocol.RPCError{Code: rpcErr.Code, Message: "Invalid Request"}
	case protocol.CodeMethodNotFound:
		return &protocol.RPCError{Code: rpcErr.Code, Message: "Method not found"}
	case protocol.CodeInvalidParams:
		return &protocol.RPCError{Code: rpcErr.Code, Message: "Invalid params"}
	default:
		return &protocol.RPCError{Code: protocol.CodeInternalError, Message: "Internal error"}
	}
}

// Both are properties of one frame's value, never of the transport, so a
// result that hits either is answered per request instead of ending the session.
var (
	errUnencodable = errors.New("encode mcp output")
	errOutputLimit = errors.New("mcp output limit exceeded")
)

type response struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      *protocol.ID       `json:"id,omitempty"`
	Result  any                `json:"result,omitempty"`
	Error   *responseErrorBody `json:"error,omitempty"`
}

type responseErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (connection *connection) writeResult(id protocol.ID, result map[string]any) error {
	return connection.writeResultContext(nil, id, result)
}

func (connection *connection) writeResultContext(ctx context.Context, id protocol.ID, result map[string]any) error {
	value := response{JSONRPC: protocol.JSONRPCVersion, ID: &id, Result: result}
	if err := connection.writeValueContext(ctx, value); errors.Is(err, errOutputLimit) || errors.Is(err, errUnencodable) {
		return connection.writeRPCErrorContext(ctx, &id, &protocol.RPCError{Code: protocol.CodeInternalError, Message: "Internal error"})
	} else {
		return err
	}
}

func (connection *connection) writeRPCError(id *protocol.ID, rpcErr *protocol.RPCError) error {
	return connection.writeRPCErrorContext(nil, id, rpcErr)
}

func (connection *connection) writeRPCErrorContext(ctx context.Context, id *protocol.ID, rpcErr *protocol.RPCError) error {
	value := response{JSONRPC: protocol.JSONRPCVersion, ID: id, Error: &responseErrorBody{
		Code: rpcErr.Code, Message: rpcErr.Message, Data: rpcErr.Data,
	}}
	return connection.writeValueContext(ctx, value)
}

func (connection *connection) writeNotification(method string, params map[string]any) error {
	return connection.writeNotificationContext(nil, method, params)
}

func (connection *connection) writeNotificationContext(ctx context.Context, method string, params map[string]any) error {
	return connection.writeValueContext(ctx, map[string]any{"jsonrpc": protocol.JSONRPCVersion, "method": method, "params": params})
}

func (connection *connection) writeValue(value any) error {
	return connection.writeValueContext(nil, value)
}

func (connection *connection) writeValueContext(ctx context.Context, value any) error {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return errUnencodable
	}
	if body.Len()-1 > protocol.MaxMessageBytes {
		return errOutputLimit
	}
	connection.write.Lock()
	defer connection.write.Unlock()
	if ctx == nil {
		ctx = connection.ctx
	}
	if ctx != nil && ctx.Err() != nil {
		return nil
	}
	// The request context only decides whether a frame starts. Once bytes may
	// reach the transport, only connection teardown may interrupt the frame;
	// abandoning it midway would fuse the next frame onto a torn line.
	written, err := interruptibleWrite(connection.ctx, connection.output, body.Bytes())
	if err == nil && written != body.Len() {
		return io.ErrShortWrite
	}
	return err
}

func (connection *connection) record(err error) {
	if err == nil {
		return
	}
	connection.errorMu.Lock()
	if connection.first == nil {
		connection.first = err
		connection.cancel()
	}
	connection.errorMu.Unlock()
}

func (connection *connection) err() error {
	connection.errorMu.Lock()
	defer connection.errorMu.Unlock()
	return connection.first
}

type requestNotifier struct {
	connection *connection
	ctx        context.Context
	meta       protocol.RequestMeta
	mu         sync.Mutex
	last       float64
	started    bool
}

func (notifier *requestNotifier) Progress(progress float64, total *float64, message string) error {
	if notifier.ctx.Err() != nil || notifier.meta.ProgressToken == nil {
		return nil
	}
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		return fmt.Errorf("invalid progress")
	}
	if total != nil && (math.IsNaN(*total) || math.IsInf(*total, 0) || *total < progress) {
		return fmt.Errorf("invalid progress total")
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if notifier.started && progress < notifier.last {
		return fmt.Errorf("progress regressed")
	}
	notifier.started = true
	notifier.last = progress
	params := map[string]any{"progress": progress, "progressToken": notifier.meta.ProgressToken}
	if total != nil {
		params["total"] = *total
	}
	if message != "" {
		params["message"] = message
	}
	return notifier.connection.writeNotificationContext(notifier.ctx, "notifications/progress", params)
}

func (notifier *requestNotifier) Log(level, logger string, data any) error {
	if notifier.ctx.Err() != nil || notifier.meta.LogLevel == "" || !loggingEnabled(notifier.connection.config.Capabilities) || !logAdmitted(notifier.meta.LogLevel, level) {
		return nil
	}
	params := map[string]any{"data": data, "level": level}
	if logger != "" {
		params["logger"] = logger
	}
	return notifier.connection.writeNotificationContext(notifier.ctx, "notifications/message", params)
}

func loggingEnabled(capabilities map[string]any) bool {
	_, ok := capabilities["logging"]
	return ok
}

func logAdmitted(requested, actual string) bool {
	severity := map[string]int{
		"emergency": 0, "alert": 1, "critical": 2, "error": 3,
		"warning": 4, "notice": 5, "info": 6, "debug": 7,
	}
	actualValue, actualOK := severity[actual]
	requestedValue, requestedOK := severity[requested]
	return actualOK && requestedOK && actualValue <= requestedValue
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
