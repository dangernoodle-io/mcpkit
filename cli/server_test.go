package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdapter is a minimal host.Adapter backed by an in-memory transport, so
// tests can build a real *mcpkit.App without a subprocess or real stdio.
type fakeAdapter struct {
	t mcpx.Transport
}

func (f fakeAdapter) Name() string { return "fake" }

func (f fakeAdapter) Transport() mcpx.Transport { return f.t }

func buildApp(t *testing.T) *mcpkit.App {
	t.Helper()

	serverT, _ := mcpx.InMemoryPair()

	app, err := mcpkit.New(mcpkit.Info{Name: "acme", Version: "1.0.0"}, fakeAdapter{t: serverT})
	require.NoError(t, err)

	return app
}

// TestAppRun_ReturnsContextCanceledOnCancel is the probe the spec calls for:
// confirms what mcpkit.App.Run (via mcpx -> go-sdk) actually returns when
// ctx is cancelled, so runLifecycle's errors.Is(err, context.Canceled)
// guard is verified load-bearing, not just belt-and-suspenders.
func TestAppRun_ReturnsContextCanceledOnCancel(t *testing.T) {
	app := buildApp(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunLifecycle_OrderStartRunShutdown(t *testing.T) {
	var order []string

	onStart := func(context.Context) error { order = append(order, "start"); return nil }
	run := func(context.Context) error { order = append(order, "run"); return nil }
	onShutdown := func(context.Context) error { order = append(order, "shutdown"); return nil }

	err := runLifecycle(context.Background(), run, onStart, onShutdown)

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "run", "shutdown"}, order)
}

func TestRunLifecycle_OnStartErrorAbortsRunAndShutdown(t *testing.T) {
	wantErr := errors.New("boom")
	ranRun := false
	ranShutdown := false

	onStart := func(context.Context) error { return wantErr }
	run := func(context.Context) error { ranRun = true; return nil }
	onShutdown := func(context.Context) error { ranShutdown = true; return nil }

	err := runLifecycle(context.Background(), run, onStart, onShutdown)

	assert.Equal(t, wantErr, err)
	assert.False(t, ranRun, "run must not execute after OnStart error")
	assert.False(t, ranShutdown, "OnShutdown must not execute after OnStart error")
}

func TestRunLifecycle_NilHooksAreSafe(t *testing.T) {
	run := func(context.Context) error { return nil }

	err := runLifecycle(context.Background(), run, nil, nil)

	assert.NoError(t, err)
}

func TestRunLifecycle_OnShutdownRunsEvenWhenRunErrors(t *testing.T) {
	runErr := errors.New("run failed")
	shutdownRan := false

	run := func(context.Context) error { return runErr }
	onShutdown := func(context.Context) error { shutdownRan = true; return nil }

	err := runLifecycle(context.Background(), run, nil, onShutdown)

	assert.ErrorIs(t, err, runErr)
	assert.True(t, shutdownRan, "OnShutdown must run even when run errors")
}

func TestRunLifecycle_CtxCanceledRunErrorIsClean(t *testing.T) {
	run := func(context.Context) error { return context.Canceled }

	err := runLifecycle(context.Background(), run, nil, nil)

	assert.NoError(t, err)
}

func TestRunLifecycle_CancelledCtxTreatsRunErrorAsClean(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := func(context.Context) error { return errors.New("transport teardown noise") }

	err := runLifecycle(ctx, run, nil, nil)

	assert.NoError(t, err)
}

func TestRunLifecycle_OnShutdownGetsNonCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var shutdownCtxErr error

	run := func(context.Context) error { return context.Canceled }
	onShutdown := func(sctx context.Context) error { shutdownCtxErr = sctx.Err(); return nil }

	err := runLifecycle(ctx, run, nil, onShutdown)

	require.NoError(t, err)
	assert.NoError(t, shutdownCtxErr, "OnShutdown must see a non-cancelled context")
}

func TestRunLifecycle_OnShutdownErrorPropagatesOnCleanRun(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")

	run := func(context.Context) error { return nil }
	onShutdown := func(context.Context) error { return shutdownErr }

	err := runLifecycle(context.Background(), run, nil, onShutdown)

	assert.ErrorIs(t, err, shutdownErr)
}

func TestRunLifecycle_DoubleFailureJoinsBothErrors(t *testing.T) {
	runErr := errors.New("run failed")
	shutdownErr := errors.New("shutdown failed")

	run := func(context.Context) error { return runErr }
	onShutdown := func(context.Context) error { return shutdownErr }

	err := runLifecycle(context.Background(), run, nil, onShutdown)

	require.Error(t, err)
	assert.ErrorIs(t, err, runErr, "double failure must still surface the run error")
	assert.ErrorIs(t, err, shutdownErr, "double failure must still surface the shutdown error")
}

func TestServerCmd_Defaults(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app})

	assert.Equal(t, "server", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestServerCmd_CustomUseAndShort(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app, Use: "run", Short: "custom short text"})

	assert.Equal(t, "run", cmd.Use)
	assert.Equal(t, "custom short text", cmd.Short)
}

func TestServerCmd_RegistersFlags(t *testing.T) {
	app := buildApp(t)
	called := false

	cmd := ServerCmd(Server{
		App: app,
		Flags: func(fs *pflag.FlagSet) {
			called = true
			fs.Bool("foo", false, "usage")
		},
	})

	assert.True(t, called)
	assert.NotNil(t, cmd.Flags().Lookup("foo"))
}

func TestServerCmd_NilFlagsIsSafe(t *testing.T) {
	app := buildApp(t)

	assert.NotPanics(t, func() {
		ServerCmd(Server{App: app})
	})
}

func TestServerCmd_AttachesSubcommands(t *testing.T) {
	app := buildApp(t)
	sub := &cobra.Command{Use: "sub", RunE: func(*cobra.Command, []string) error { return nil }}

	cmd := ServerCmd(Server{App: app, Subcommands: []*cobra.Command{sub}})

	found := false
	for _, c := range cmd.Commands() {
		if c == sub {
			found = true
		}
	}
	assert.True(t, found, "subcommand must be attached")
}

func TestServerCmd_RunE_CleanExitOnSignalContextCancel(t *testing.T) {
	app := buildApp(t)

	var started, stopped []string
	cmd := ServerCmd(Server{
		App:        app,
		OnStart:    func(context.Context) error { started = append(started, "start"); return nil },
		OnShutdown: func(context.Context) error { stopped = append(stopped, "stop"); return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := cmd.RunE(cmd, nil)

	assert.NoError(t, err)
	assert.Equal(t, []string{"start"}, started)
	assert.Equal(t, []string{"stop"}, stopped)
}

func TestServerCmd_RunE_OnStartErrorAborts(t *testing.T) {
	app := buildApp(t)
	wantErr := errors.New("db unavailable")

	ranShutdown := false
	cmd := ServerCmd(Server{
		App:        app,
		OnStart:    func(context.Context) error { return wantErr },
		OnShutdown: func(context.Context) error { ranShutdown = true; return nil },
	})

	ctx := context.Background()
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)

	assert.Equal(t, wantErr, err)
	assert.False(t, ranShutdown)
}

func TestUseAsDefault(t *testing.T) {
	app := buildApp(t)
	cmd := ServerCmd(Server{App: app})
	root := &cobra.Command{Use: "root"}

	UseAsDefault(root, cmd)

	require.NotNil(t, root.RunE)

	ctx, cancel := context.WithCancel(context.Background())
	root.SetContext(ctx)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := root.RunE(root, nil)
	assert.NoError(t, err)
}

// TestUseAsDefault_CopiesFlagsForBareInvocation mirrors the HIGH-severity
// repro: a flag registered via Server.Flags must be accepted on BARE
// invocation of root, not just on the explicit `server` subcommand, because
// cobra parses argv against the executing command's own FlagSet (root's on
// bare invocation, not cmd's).
func TestUseAsDefault_CopiesFlagsForBareInvocation(t *testing.T) {
	app := buildApp(t)

	started := false
	sc := ServerCmd(Server{
		App: app,
		Flags: func(fs *pflag.FlagSet) {
			fs.String("foo", "", "usage")
		},
		OnStart: func(context.Context) error { started = true; return nil },
	})

	root := &cobra.Command{Use: "root"}
	root.AddCommand(sc)
	UseAsDefault(root, sc)

	ctx, cancel := context.WithCancel(context.Background())
	root.SetContext(ctx)
	root.SetArgs([]string{"--foo", "x"})

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := root.Execute()

	require.NoError(t, err, "bare invocation must not reject a Server.Flags flag as unknown")
	assert.True(t, started)
	assert.Equal(t, "x", sc.Flags().Lookup("foo").Value.String())

	// the explicit `server` subcommand must remain reachable too.
	found := false
	for _, c := range root.Commands() {
		if c == sc {
			found = true
		}
	}
	assert.True(t, found, "server subcommand must stay attached alongside UseAsDefault")
}

// TestUseAsDefault_HTTPMode proves --http/--stateless reach root's FlagSet
// through UseAsDefault's flag copy and that HTTP serving is reachable on a
// BARE root invocation (no "server" token) — the flags a consumer wires via
// Server.HTTP must work identically whether the app is invoked as `myapp
// server --http :0` or, via UseAsDefault, bare `myapp --http :0`.
func TestUseAsDefault_HTTPMode(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	var started, stopped []string
	sc := ServerCmd(Server{
		App:        app,
		HTTP:       &ServerHTTP{},
		OnStart:    func(context.Context) error { started = append(started, "start"); return nil },
		OnShutdown: func(context.Context) error { stopped = append(stopped, "stop"); return nil },
	})

	root := &cobra.Command{Use: "root"}
	root.AddCommand(sc)
	UseAsDefault(root, sc)

	ctx, cancel := context.WithCancel(context.Background())
	root.SetContext(ctx)
	root.SetArgs([]string{"--http", addr})

	errCh := make(chan error, 1)
	go func() { errCh <- root.Execute() }()

	status, _ := postInitialize(t, addr, "/mcp")
	assert.Equal(t, http.StatusOK, status)

	cancel()
	require.NoError(t, <-errCh)

	assert.Equal(t, []string{"start"}, started)
	assert.Equal(t, []string{"stop"}, stopped)
}

// freeAddr picks an ephemeral loopback address by binding to port 0 and
// releasing it immediately, so httpx.Serve (which only takes an addr
// string, not a pre-bound listener) can bind it. The release-then-rebind
// window is negligible in practice for a same-process test.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	return addr
}

// postInitialize POSTs a bare JSON-RPC initialize request to url/mcpPath
// and returns the response's status code and Content-Type header, retrying
// briefly since the server under test may still be coming up.
func postInitialize(t *testing.T, addr, mcpPath string) (int, string) {
	t.Helper()

	url := "http://" + addr + mcpPath
	body := `{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}`

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(10 * time.Millisecond)
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		return resp.StatusCode, resp.Header.Get("Content-Type")
	}

	t.Fatalf("server never became reachable: %v", lastErr)
	return 0, ""
}

func TestServerCmd_HTTPFlagsRegisteredWhenHTTPSet(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{}})

	assert.NotNil(t, cmd.Flags().Lookup("http"))
	assert.NotNil(t, cmd.Flags().Lookup("stateless"))
}

func TestServerCmd_HTTPFlagsAbsentWhenHTTPNil(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app})

	assert.Nil(t, cmd.Flags().Lookup("http"))
	assert.Nil(t, cmd.Flags().Lookup("stateless"))
}

func TestServerCmd_StatelessFlagDefaultsFromServerHTTP(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{Stateless: true}})

	assert.Equal(t, "true", cmd.Flags().Lookup("stateless").DefValue)
}

func TestServerCmd_RunE_HTTPAbsent_UsesStdio(t *testing.T) {
	app := buildApp(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{}})

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := cmd.RunE(cmd, nil)

	assert.NoError(t, err)
}

// TestServerCmd_RunE_HTTPMode drives the real RunE (flag parsing included)
// with --http set to an ephemeral address, then round-trips a JSON-RPC
// initialize request against the running handler and asserts OnStart/
// OnShutdown both fire around the HTTP transport exactly as they do around
// stdio.
func TestServerCmd_RunE_HTTPMode(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	var started, stopped []string
	cmd := ServerCmd(Server{
		App:        app,
		HTTP:       &ServerHTTP{},
		OnStart:    func(context.Context) error { started = append(started, "start"); return nil },
		OnShutdown: func(context.Context) error { stopped = append(stopped, "stop"); return nil },
	})

	require.NoError(t, cmd.Flags().Set("http", addr))

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.RunE(cmd, nil) }()

	status, _ := postInitialize(t, addr, "/mcp")
	assert.Equal(t, http.StatusOK, status)

	cancel()
	require.NoError(t, <-errCh)

	assert.Equal(t, []string{"start"}, started)
	assert.Equal(t, []string{"stop"}, stopped)
}

// TestServerCmd_RunE_HTTPMode_CustomMCPPath proves ServerHTTP.MCPPath
// overrides the "/mcp" default.
func TestServerCmd_RunE_HTTPMode_CustomMCPPath(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{MCPPath: "/custom"}})
	require.NoError(t, cmd.Flags().Set("http", addr))

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.RunE(cmd, nil) }()

	status, _ := postInitialize(t, addr, "/custom")
	assert.Equal(t, http.StatusOK, status)

	cancel()
	require.NoError(t, <-errCh)
}

// TestServerCmd_RunE_HTTPMode_Routes proves ServerHTTP.Routes mounts
// consumer routes on the same mux the MCP endpoint is served from.
func TestServerCmd_RunE_HTTPMode_Routes(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	cmd := ServerCmd(Server{
		App: app,
		HTTP: &ServerHTTP{
			Routes: func(mux *http.ServeMux) {
				mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
		},
	})
	require.NoError(t, cmd.Flags().Set("http", addr))

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.RunE(cmd, nil) }()

	// Poll /healthz until the server is up (also proves the consumer
	// route was actually mounted, since a 404 would mean it was not).
	deadline := time.Now().Add(2 * time.Second)
	var status int
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			status = resp.StatusCode
			_ = resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, http.StatusOK, status)

	cancel()
	require.NoError(t, <-errCh)
}

// TestServerCmd_RunE_HTTPMode_Stateless proves --stateless flips the
// handler to stateless + JSON-response mode: the observable difference is
// the initialize response's Content-Type, application/json instead of the
// default text/event-stream (mirrors mcpx's
// TestHTTPHandlerContentTypeMapping, which proves WithJSONResponse reaches
// the wire).
func TestServerCmd_RunE_HTTPMode_Stateless(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{}})
	require.NoError(t, cmd.Flags().Set("http", addr))
	require.NoError(t, cmd.Flags().Set("stateless", "true"))

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.RunE(cmd, nil) }()

	_, contentType := postInitialize(t, addr, "/mcp")
	assert.Contains(t, contentType, "application/json")

	cancel()
	require.NoError(t, <-errCh)
}

// TestServerCmd_RunE_HTTPMode_DefaultIsStateful proves the default (no
// --stateless) initialize response stays SSE, i.e. stateless mode is
// opt-in, not accidentally always-on.
func TestServerCmd_RunE_HTTPMode_DefaultIsStateful(t *testing.T) {
	app := buildApp(t)
	addr := freeAddr(t)

	cmd := ServerCmd(Server{App: app, HTTP: &ServerHTTP{}})
	require.NoError(t, cmd.Flags().Set("http", addr))

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.RunE(cmd, nil) }()

	_, contentType := postInitialize(t, addr, "/mcp")
	assert.Contains(t, contentType, "text/event-stream")

	cancel()
	require.NoError(t, <-errCh)
}
