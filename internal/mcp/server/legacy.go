package server

import (
	"context"
	"errors"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

type legacyState uint8

const (
	legacyFresh legacyState = iota
	legacyInitialized
	legacyReady
)

func (connection *connection) handleLegacyRequest(ctx context.Context, active *activeRequest, inbound protocol.Inbound, ready bool) {
	if inbound.Method == "initialize" {
		connection.initializeLegacy(ctx, active, inbound)
		return
	}
	meta, rpcErr := protocol.DecodeLegacyRequestMeta(inbound.Params)
	if rpcErr == nil {
		switch inbound.Method {
		case "ping":
			if len(inbound.Params) != 0 {
				rpcErr = protocol.InvalidParams("Invalid params")
			}
		case "tools/list", "tools/call":
			if !ready {
				rpcErr = protocol.NewError(protocol.CodeInvalidRequest, "Invalid Request")
			}
		default:
			rpcErr = protocol.MethodNotFound()
		}
	}
	if rpcErr != nil {
		connection.respond(active, func() error { return connection.writeRPCErrorContext(ctx, inbound.ID, rpcErr) })
		return
	}
	if inbound.Method == "ping" {
		connection.respond(active, func() error { return connection.writeResultContext(ctx, *inbound.ID, map[string]any{}) })
		return
	}
	// Session declarations are data only. Each handler gets an independent copy.
	meta.ClientCapabilities = cloneLegacyObject(connection.legacyMeta.ClientCapabilities)
	meta.ClientInfo = cloneLegacyObject(connection.legacyMeta.ClientInfo)
	if inbound.Params == nil {
		inbound.Params = map[string]any{}
	}
	request := protocol.Request{ID: *inbound.ID, Method: inbound.Method, Params: inbound.Params, Meta: meta}
	notifier := &requestNotifier{connection: connection, ctx: ctx, meta: meta}
	result, rpcErr := connection.config.Handler.Handle(ctx, request, notifier)
	if rpcErr != nil {
		connection.respond(active, func() error { return connection.writeRPCErrorContext(ctx, inbound.ID, sanitize(rpcErr)) })
		return
	}
	if ctx.Err() != nil {
		return
	}
	result = cloneMap(result)
	delete(result, "cacheScope")
	delete(result, "ttlMs")
	delete(result, "resultType")
	connection.respond(active, func() error { return connection.writeResultContext(ctx, request.ID, result) })
}

func (connection *connection) initializeLegacy(ctx context.Context, active *activeRequest, inbound protocol.Inbound) {
	if connection.legacyState != legacyFresh {
		connection.respond(active, func() error {
			return connection.writeRPCErrorContext(ctx, inbound.ID, protocol.NewError(protocol.CodeInvalidRequest, "Invalid Request"))
		})
		return
	}
	meta, rpcErr := protocol.DecodeLegacyInitialize(inbound.Params)
	if rpcErr != nil {
		connection.respond(active, func() error { return connection.writeRPCErrorContext(ctx, inbound.ID, rpcErr) })
		return
	}
	capabilities := map[string]any{}
	if tools, ok := connection.config.Capabilities["tools"]; ok {
		capabilities["tools"] = tools
	}
	result := map[string]any{"protocolVersion": protocol.LegacyVersion, "serverInfo": connection.serverInfo(), "capabilities": capabilities}
	if connection.config.Instructions != "" {
		result["instructions"] = connection.config.Instructions
	}
	connection.respond(active, func() error {
		// Use the raw bounded writer: a fallback error response is not a
		// successful initialize and must leave the lifecycle fresh.
		err := connection.writeValueContext(ctx, response{JSONRPC: protocol.JSONRPCVersion, ID: inbound.ID, Result: result})
		if errors.Is(err, errOutputLimit) || errors.Is(err, errUnencodable) {
			return connection.writeRPCErrorContext(ctx, inbound.ID, protocol.NewError(protocol.CodeInternalError, "Internal error"))
		}
		if err == nil && ctx.Err() == nil {
			connection.legacyMeta = meta
			connection.legacyState = legacyInitialized
		}
		return err
	})
}

func cloneLegacyObject(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneLegacyValue(value)
	}
	return result
}

func cloneLegacyValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneLegacyObject(value)
	case []any:
		result := make([]any, len(value))
		for index, item := range value {
			result[index] = cloneLegacyValue(item)
		}
		return result
	default:
		return value
	}
}
