package mcpx

import "github.com/modelcontextprotocol/go-sdk/mcp"

// TextResult builds a CallToolResult carrying a single text-content block.
// It is the mcpx-owned counterpart to ResultText, letting callers construct
// results without importing go-sdk directly.
func TextResult(text string) *CallToolResult {
	return &CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// ErrorResult builds a CallToolResult carrying a single text-content block
// and marked IsError, for a caller that must surface a tool-level error as
// a *result value* rather than returning a Go error.
//
// Note: a typed handler that returns a Go error (return nil, zero, err)
// already yields an IsError result via go-sdk's own error handling — see
// the probe in result_test.go — so ErrorResult is for building such a
// result explicitly, where no error value is being returned.
func ErrorResult(text string) *CallToolResult {
	return &CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
}
