package mcpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type httpEchoIn struct {
	Text string `json:"text"`
}

type httpEchoOut struct {
	Text string `json:"text"`
}

func newHTTPEchoServer(t *testing.T) *Server {
	t.Helper()
	srv := NewServer(Implementation{Name: "http-test-server", Version: "0.0.1"}, "", 0)
	AddTool(srv, &Tool{
		Name:        "echo",
		Description: "echoes text back",
	}, func(_ context.Context, _ *CallToolRequest, in httpEchoIn) (*CallToolResult, httpEchoOut, error) {
		return nil, httpEchoOut(in), nil
	})
	return srv
}

// TestHTTPHandlerRoundTrip proves HTTPHandler serves a real MCP session over
// HTTP: a go-sdk client connects via StreamableClientTransport, lists the
// registered tool, and round-trips a CallTool through it.
func TestHTTPHandlerRoundTrip(t *testing.T) {
	srv := newHTTPEchoServer(t)

	ts := httptest.NewServer(srv.HTTPHandler())
	defer ts.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	tools, err := sess.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "echo", tools.Tools[0].Name)

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"text": "hi"}})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.JSONEq(t, `{"text":"hi"}`, ResultText(res))
}

// TestHTTPHandlerStateless proves WithStateless actually reaches go-sdk:
// even without server-side session state, a plain tool call still round
// trips end to end.
func TestHTTPHandlerStateless(t *testing.T) {
	srv := newHTTPEchoServer(t)

	ts := httptest.NewServer(srv.HTTPHandler(WithStateless(true)))
	defer ts.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"text": "stateless"}})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.JSONEq(t, `{"text":"stateless"}`, ResultText(res))
}

// TestHTTPHandlerJSONResponse proves WithJSONResponse reaches go-sdk: a
// stateful session using application/json responses instead of SSE still
// round trips a tool call.
func TestHTTPHandlerJSONResponse(t *testing.T) {
	srv := newHTTPEchoServer(t)

	ts := httptest.NewServer(srv.HTTPHandler(WithJSONResponse(true)))
	defer ts.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"text": "json"}})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.JSONEq(t, `{"text":"json"}`, ResultText(res))
}

// postInitializeContentType POSTs a bare JSON-RPC initialize request
// directly (bypassing the go-sdk client, which abstracts the raw response
// away) and returns the response's Content-Type header, so callers can
// verify which wire format the handler actually chose.
func postInitializeContentType(t *testing.T, url string) string {
	t.Helper()
	body := `{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}`
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	return resp.Header.Get("Content-Type")
}

// TestHTTPHandlerContentTypeMapping proves WithJSONResponse's mapping onto
// go-sdk's StreamableHTTPOptions.JSONResponse actually reaches the wire: a
// swapped Stateless/JSONResponse field mapping would make this fail. The
// default handler responds SSE (text/event-stream); WithJSONResponse(true)
// switches it to application/json.
func TestHTTPHandlerContentTypeMapping(t *testing.T) {
	sseServer := newHTTPEchoServer(t)
	sseTS := httptest.NewServer(sseServer.HTTPHandler())
	defer sseTS.Close()
	require.Contains(t, postInitializeContentType(t, sseTS.URL), "text/event-stream")

	jsonServer := newHTTPEchoServer(t)
	jsonTS := httptest.NewServer(jsonServer.HTTPHandler(WithJSONResponse(true)))
	defer jsonTS.Close()
	require.Contains(t, postInitializeContentType(t, jsonTS.URL), "application/json")
}
