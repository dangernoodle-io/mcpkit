// Package testkit is mcpkit's shipped in-memory MCP test harness, built on
// mcpx.InMemoryPair (no subprocess). It is used by mcpkit's own tests and is
// intended for reuse by downstream consumers.
package testkit

import (
	"context"
	"sync"
	"testing"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/stretchr/testify/require"
)

// ProgressEvent is one progress notification received for a tracked token.
type ProgressEvent struct {
	Token    any
	Message  string
	Progress float64
	Total    float64
}

// Harness wires an in-memory MCP client to an *mcpkit.App for testing.
type Harness struct {
	t       testing.TB
	session *mcpx.ClientSession

	mu       sync.Mutex
	progress map[any][]ProgressEvent
}

// New composes app over an in-memory transport pair, connects a client, and
// returns a ready Harness. The client is closed automatically via
// t.Cleanup.
func New(t testing.TB, app *mcpkit.App) *Harness {
	t.Helper()

	ctx := context.Background()
	serverT, clientT := mcpx.InMemoryPair()

	// Servers must connect before clients (go-sdk requirement).
	srvSess, err := app.Connect(ctx, serverT)
	require.NoError(t, err, "connect app to in-memory transport")
	t.Cleanup(func() {
		_ = srvSess.Close()
	})

	h := &Harness{t: t, progress: make(map[any][]ProgressEvent)}

	client := mcpx.NewClient(mcpx.Implementation{Name: "testkit", Version: "0.0.0"}, &mcpx.ClientOptions{
		OnProgress: h.recordProgress,
	})

	sess, err := client.Connect(ctx, clientT)
	require.NoError(t, err, "connect testkit client")

	h.session = sess
	t.Cleanup(func() {
		_ = sess.Close()
	})

	return h
}

func (h *Harness) recordProgress(_ context.Context, token any, message string, progress, total float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.progress[token] = append(h.progress[token], ProgressEvent{
		Token:    token,
		Message:  message,
		Progress: progress,
		Total:    total,
	})
}

// CallTool calls the named tool with args, which must be JSON-marshalable.
func (h *Harness) CallTool(ctx context.Context, name string, args any) (*mcpx.CallToolResult, error) {
	return h.session.CallTool(ctx, name, args)
}

// CallToolWithProgressToken calls the named tool with args and attaches
// token as the request's progress token, so server-side progress
// notifications for this call can be retrieved via ProgressEvents(token).
func (h *Harness) CallToolWithProgressToken(ctx context.Context, name string, args any, token any) (*mcpx.CallToolResult, error) {
	return h.session.CallToolWithProgressToken(ctx, name, args, token)
}

// ListTools lists the tools the composed app advertises.
func (h *Harness) ListTools(ctx context.Context) (*mcpx.ListToolsResult, error) {
	return h.session.ListTools(ctx)
}

// ProgressEvents returns a snapshot of progress notifications recorded for
// token, in receipt order.
func (h *Harness) ProgressEvents(token any) []ProgressEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]ProgressEvent, len(h.progress[token]))
	copy(out, h.progress[token])
	return out
}
