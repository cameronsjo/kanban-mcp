// Package tools is the MCP layer. It ports the index.ts dispatch and the
// tools/*.ts composites: one AddTool per manager, each with a flat
// action-discriminated input struct, returning heterogeneous per-action results
// as a single JSON TextContent block.
package tools

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// jsonResult marshals v into a single TextContent block, mirroring the TS
// `{ content: [{ type: "text", text: JSON.stringify(result) }] }`.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// errResult renders err as a tool-error CallToolResult (IsError=true) with
// actionable text. Planka/validation failures surface this way so the model can
// see and self-correct — mirroring how zod failures surfaced in the TS server —
// rather than as protocol-level errors.
func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

// respond converts a dispatch's (result, error) into the SDK's tool-result
// triple. Expected failures (Planka API errors, validation) become IsError tool
// results; the Go error slot is reserved for unexpected faults (our own output
// failing to marshal).
func respond(res any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return errResult(err), nil, nil
	}
	out, marshalErr := jsonResult(res)
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	return out, nil, nil
}
