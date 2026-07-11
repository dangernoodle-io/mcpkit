// Package mcpkit is the composition root of a host-agnostic MCP server:
// compile-time plugin-style composition of Capabilities over a pluggable
// host.Adapter, built on the mcpx protocol seam.
package mcpkit

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dangernoodle-io/mcpkit/host"
	"github.com/dangernoodle-io/mcpkit/mcpx"
)

// Info identifies the server being composed.
type Info struct {
	Name    string
	Version string
}

// Registrar is what a Capability's Attach method uses to register itself
// against the underlying server and inspect the target host. Capabilities
// register tools through the package-level AddTool, not against mcpx
// directly, so mcpkit owns a single tool-registration chokepoint.
type Registrar struct {
	server *mcpx.Server
	host   host.Adapter
}

// Host returns the host.Adapter the app is composed for.
func (r *Registrar) Host() host.Adapter {
	return r.host
}

// AddTool registers a typed tool handler through r. This is the only
// tool-registration chokepoint capabilities should use; MC-8 fills in
// annotations/risk/recover behind this same signature additively.
func AddTool[In, Out any](r *Registrar, t *mcpx.Tool, h mcpx.Handler[In, Out]) {
	// MC-8: annotations/risk/recover hook here
	mcpx.AddTool(r.server, t, h)
}

// Capability is a self-contained unit of server functionality, attached to
// the composition root at build time.
type Capability interface {
	Attach(r *Registrar) error
}

// App is a fully composed MCP server, ready to run.
type App struct {
	server *mcpx.Server
	host   host.Adapter
}

// New composes an App from a host.Adapter and zero or more Capabilities,
// attaching each capability in order.
func New(info Info, h host.Adapter, caps ...Capability) (*App, error) {
	if h == nil {
		return nil, fmt.Errorf("mcpkit: host adapter must not be nil")
	}

	srv := mcpx.NewServer(mcpx.Implementation{Name: info.Name, Version: info.Version})
	r := &Registrar{server: srv, host: h}

	for _, c := range caps {
		if err := c.Attach(r); err != nil {
			return nil, fmt.Errorf("mcpkit: attach capability: %w", err)
		}
	}

	return &App{server: srv, host: h}, nil
}

// Run serves the app over its host's transport until the client disconnects
// or ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	return a.server.Run(ctx, a.host.Transport())
}

// Connect connects the app over t without blocking, for use by testkit and
// other in-process harnesses.
func (a *App) Connect(ctx context.Context, t mcpx.Transport) (*mcpx.Session, error) {
	return a.server.Connect(ctx, t)
}

// HTTPHandler exposes the composed server over streamable-HTTP for the
// consumer to mount. mcpkit is path-agnostic: the returned handler is bare
// and MCP-over-HTTP is entirely opt-in — the consumer decides whether and
// where to mount it.
func (a *App) HTTPHandler(opts ...mcpx.HTTPOption) http.Handler {
	return a.server.HTTPHandler(opts...)
}
