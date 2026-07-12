// Package mcpkit is the composition root of a host-agnostic MCP server:
// compile-time plugin-style composition of Capabilities over a pluggable
// host.Adapter, built on the mcpx protocol seam.
package mcpkit

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/dangernoodle-io/mcpkit/host"
	"github.com/dangernoodle-io/mcpkit/mcpx"
)

// Info identifies the server being composed.
type Info struct {
	Name    string
	Version string

	// Instructions is optional MCP server instructions advertised to
	// clients (tool-selection guidance); empty = none.
	Instructions string
}

// Risk classifies a tool's blast radius, from a client's perspective, for
// gating and annotation purposes. Every AddTool call requires one
// (fail-closed): a caller can't omit risk classification and accidentally
// ship a write/destructive tool unannotated.
type Risk int

const (
	// ReadOnly tools only observe state; they never mutate anything a
	// client cares about.
	ReadOnly Risk = iota
	// Write tools mutate state, but the mutation is reversible/benign
	// enough not to warrant a destructive warning.
	Write
	// Destructive tools perform an irreversible or high-blast-radius
	// mutation (data loss, external side effects that can't be undone).
	Destructive
)

// ToolOption configures optional per-tool metadata at AddTool time.
type ToolOption interface {
	applyTool(*toolMeta)
}

// toolMeta is the mutable state ToolOption values apply to.
type toolMeta struct {
	group string
}

// groupOption is the ToolOption Group returns.
type groupOption string

func (g groupOption) applyTool(m *toolMeta) {
	m.group = string(g)
}

// Group tags a tool with an arbitrary consumer-defined group name, recorded
// in the registry's byGroup bookkeeping post-registration. mcpkit imposes no
// meaning on the group string; a consumer's own gating (MC-44/MC-45) is what
// interprets it.
func Group(name string) ToolOption {
	return groupOption(name)
}

// Registrar is what a Capability's Attach method uses to register itself
// against the underlying server and inspect the target host. Capabilities
// register tools through the package-level AddTool, not against mcpx
// directly, so mcpkit owns a single tool-registration chokepoint.
type Registrar struct {
	server *mcpx.Server
	host   host.Adapter
	reg    *registry
}

// Host returns the host.Adapter the app is composed for.
func (r *Registrar) Host() host.Adapter {
	return r.host
}

// AddTool captures a typed tool handler through r for deferred registration
// against the underlying server. This is the only tool-registration
// chokepoint capabilities should use; MC-8 wraps every handler in a
// panic-recover here so a panicking tool surfaces as an IsError result
// instead of crashing the server process. Registration itself is deferred
// until the App's finalize runs (MC-43) so a later gate (MC-44) can filter
// which pending tools actually register before anything is exposed to a
// client.
//
// risk is required (fail-closed): a caller cannot omit a tool's risk
// classification. When t.Annotations is nil, AddTool derives it from risk
// via mcpx.RiskAnnotations; an explicitly-set Annotations is left untouched.
func AddTool[In, Out any](r *Registrar, t *mcpx.Tool, risk Risk, h mcpx.Handler[In, Out], opts ...ToolOption) {
	if t.Annotations == nil {
		t.Annotations = mcpx.RiskAnnotations(risk == ReadOnly, risk == Destructive)
	}

	meta := toolMeta{}
	for _, opt := range opts {
		opt.applyTool(&meta)
	}

	wrapped := func(ctx context.Context, req *mcpx.CallToolRequest, in In) (res *mcpx.CallToolResult, out Out, err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("tool %q panicked: %v", t.Name, p)
			}
		}()
		return h(ctx, req, in)
	}

	r.reg.add(pendingTool{
		name:  t.Name,
		group: meta.group,
		risk:  risk,
		register: func(s *mcpx.Server) {
			mcpx.AddTool(s, t, wrapped)
		},
	})
}

// pendingTool is one captured-but-not-yet-registered AddTool call. register
// closes over the tool's In/Out type parameters (erased here) and the
// panic-recover-wrapped handler; calling it performs the actual
// mcpx.AddTool registration against a live server.
type pendingTool struct {
	name, group string
	risk        Risk
	register    func(*mcpx.Server)
}

// registry accumulates pending tool registrations shared between a
// Registrar (during composition, via AddTool) and its App (at finalize
// time). Deferring registration out of AddTool is what lets a later gate
// (MC-44) filter which pending tools actually register before finalize ever
// touches the live server.
type registry struct {
	mu      sync.Mutex
	pending []pendingTool
	byGroup map[string][]string
	started bool
}

// add appends t to the pending set. Safe for concurrent Attach calls.
func (reg *registry) add(t pendingTool) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.pending = append(reg.pending, t)
}

// finalize registers every pending tool against srv exactly once. It is
// idempotent: a second call (e.g. Run then Connect, or Run called twice) is
// a guarded no-op rather than double-registering or panicking.
func (reg *registry) finalize(srv *mcpx.Server) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.started {
		return
	}
	reg.started = true

	if reg.byGroup == nil {
		reg.byGroup = make(map[string][]string)
	}

	for _, t := range reg.pending {
		// MC-44 gate predicate slots in here (a per-tool allow check that may `continue`).
		t.register(srv)
		reg.byGroup[t.group] = append(reg.byGroup[t.group], t.name)
	}
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
	reg    *registry
}

// New composes an App from a host.Adapter and zero or more Capabilities,
// attaching each capability in order. Tool registration is deferred: no
// tool is registered against the underlying server until Run, Connect, or
// HTTPHandler calls finalize.
func New(info Info, h host.Adapter, caps ...Capability) (*App, error) {
	if h == nil {
		return nil, fmt.Errorf("mcpkit: host adapter must not be nil")
	}

	srv := mcpx.NewServer(mcpx.Implementation{Name: info.Name, Version: info.Version}, info.Instructions)
	reg := &registry{}
	r := &Registrar{server: srv, host: h, reg: reg}

	for _, c := range caps {
		if err := c.Attach(r); err != nil {
			return nil, fmt.Errorf("mcpkit: attach capability: %w", err)
		}
	}

	return &App{server: srv, host: h, reg: reg}, nil
}

// finalize registers every pending tool against a's server exactly once,
// regardless of how many of Run/Connect/HTTPHandler trigger it or in what
// order.
func (a *App) finalize() {
	a.reg.finalize(a.server)
}

// Run serves the app over its host's transport until the client disconnects
// or ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.finalize()
	return a.server.Run(ctx, a.host.Transport())
}

// Connect connects the app over t without blocking, for use by testkit and
// other in-process harnesses.
func (a *App) Connect(ctx context.Context, t mcpx.Transport) (*mcpx.Session, error) {
	a.finalize()
	return a.server.Connect(ctx, t)
}

// HTTPHandler exposes the composed server over streamable-HTTP for the
// consumer to mount. mcpkit is path-agnostic: the returned handler is bare
// and MCP-over-HTTP is entirely opt-in — the consumer decides whether and
// where to mount it. finalize runs here too (not just Run/Connect) because
// an HTTP-only consumer (see cli.ServerCmd's --http path and
// examples/http) never calls Run or Connect at all — without this,
// deferred registration would silently ship zero tools over HTTP,
// regressing MC-43's behavior-preservation goal.
func (a *App) HTTPHandler(opts ...mcpx.HTTPOption) http.Handler {
	a.finalize()
	return a.server.HTTPHandler(opts...)
}
