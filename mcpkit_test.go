package mcpkit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/dangernoodle-io/mcpkit/testkit"
	"github.com/stretchr/testify/require"
)

type helloIn struct {
	Name string `json:"name" jsonschema:"name to greet"`
}

type helloOut struct {
	Greeting string `json:"greeting"`
}

type helloCap struct{}

func (helloCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{
		Name:        "hello",
		Description: "greets the caller by name",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, in helloIn) (*mcpx.CallToolResult, helloOut, error) {
		name := in.Name
		if name == "" {
			name = "world"
		}
		return nil, helloOut{Greeting: fmt.Sprintf("hello, %s!", name)}, nil
	})
	return nil
}

// TestEndToEnd proves the mcpx seam, capability wiring, and testkit harness
// work together: compose an App, list its tools, call one, and decode the
// result.
func TestEndToEnd(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "hello")

	res, err := h.CallTool(context.Background(), "hello", map[string]any{"name": "mcpkit"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[helloOut](t, res)
	require.Equal(t, "hello, mcpkit!", out.Greeting)
}

// TestNewNilHost proves New rejects a nil host adapter rather than panicking
// later.
func TestNewNilHost(t *testing.T) {
	_, err := mcpkit.New(mcpkit.Info{Name: "e2e", Version: "0.0.1"}, nil)
	require.Error(t, err)
}

// TestAppHTTPHandler proves App.HTTPHandler delegates to the composed
// server's real streamable-HTTP handler (the mcpx protocol round trip is
// covered in mcpx/http_test.go, which alone is allowed to import go-sdk).
// A bare GET without the streamable-HTTP Accept header is go-sdk's
// documented rejection for a malformed streamable request: 400 with a
// text/event-stream mention in the body. Asserting that exact behavior
// (rather than just a non-zero status) catches a broken/no-op delegate.
func TestAppHTTPHandler(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "http-e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := app.HTTPHandler()
	require.NotNil(t, h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "text/event-stream")
}
