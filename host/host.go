// Package host defines the seam mcpkit uses to adapt a server to a specific
// MCP host (generic stdio, Claude Code, Cursor, ...). Subpackages provide
// separable stub implementations so a consumer imports only the host it
// targets.
package host

import "github.com/dangernoodle-io/mcpkit/mcpx"

// Adapter binds mcpkit's composition root to a specific MCP host.
type Adapter interface {
	// Name identifies the host, e.g. "generic", "claude-code", "cursor".
	Name() string
	// Transport returns the transport the app should serve over for this
	// host.
	Transport() mcpx.Transport
}
