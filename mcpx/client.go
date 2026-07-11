package mcpx

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProgressHandler receives progress notifications for a tracked token.
type ProgressHandler func(ctx context.Context, token any, message string, progress, total float64)

// ClientOptions configures a Client. The zero value is valid.
type ClientOptions struct {
	// OnProgress, if set, is invoked for every progress notification the
	// client receives.
	OnProgress ProgressHandler
}

// Client is a thin wrapper over mcp.Client, kept so callers never import
// go-sdk directly.
type Client struct {
	cli *mcp.Client
}

// NewClient constructs a Client advertising the given implementation.
func NewClient(impl Implementation, opts *ClientOptions) *Client {
	var co *mcp.ClientOptions
	if opts != nil && opts.OnProgress != nil {
		handler := opts.OnProgress
		co = &mcp.ClientOptions{
			ProgressNotificationHandler: func(ctx context.Context, req *mcp.ProgressNotificationClientRequest) {
				p := req.Params
				handler(ctx, p.ProgressToken, p.Message, p.Progress, p.Total)
			},
		}
	}
	return &Client{cli: mcp.NewClient(&impl, co)}
}

// ClientSession is a live client-side connection to a server.
type ClientSession struct {
	sess *mcp.ClientSession
}

// Connect connects c over t. The transport's server side must already be
// connected (see InMemoryPair).
func (c *Client) Connect(ctx context.Context, t Transport) (*ClientSession, error) {
	sess, err := c.cli.Connect(ctx, t.transport(), nil)
	if err != nil {
		return nil, err
	}
	return &ClientSession{sess: sess}, nil
}

// CallTool calls the named tool with args, which must be JSON-marshalable.
func (s *ClientSession) CallTool(ctx context.Context, name string, args any) (*CallToolResult, error) {
	return s.sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
}

// CallToolWithProgressToken calls the named tool with args and attaches
// token as the request's progress token, so the server can key progress
// notifications for this call to it (see NotifyProgress).
func (s *ClientSession) CallToolWithProgressToken(ctx context.Context, name string, args any, token any) (*CallToolResult, error) {
	params := &mcp.CallToolParams{Name: name, Arguments: args}
	params.SetProgressToken(token)
	return s.sess.CallTool(ctx, params)
}

// ListTools lists the tools the connected server advertises.
func (s *ClientSession) ListTools(ctx context.Context) (*ListToolsResult, error) {
	return s.sess.ListTools(ctx, &mcp.ListToolsParams{})
}

// InitializeResult returns the server's response to this session's
// initialize handshake, including any Instructions the server advertised.
func (s *ClientSession) InitializeResult() *InitializeResult {
	return s.sess.InitializeResult()
}

// Close closes the underlying session.
func (s *ClientSession) Close() error {
	return s.sess.Close()
}
