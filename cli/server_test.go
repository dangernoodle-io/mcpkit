package cli

import (
	"context"
	"errors"
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
