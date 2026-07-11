// Package cli provides cobra.Command factories for mounting an *mcpkit.App
// into a consumer's own root command. mcpkit does not own the root: the
// consumer builds its own cobra root and mounts ServerCmd (and, optionally,
// VersionCmd) as subcommands of it.
//
// mcpkit owns transport selection, running the server, and graceful
// shutdown; the consumer extends the server command via Server.App plus the
// OnStart/OnShutdown hooks, Flags, and Subcommands (arbitrary cobra
// subtrees — see CommandProvider/MountProviders for the uniform mounting
// API). Use UseAsDefault to make a bare invocation of the consumer's binary
// (no subcommand given) run the server command directly — useful for a
// Claude Code plugin launched without an explicit subcommand.
//
// cobra auto-provides a `completion` command on any root with subcommands,
// so this package does not add one.
//
// Serving over HTTP instead of stdio (via App.HTTPHandler) is a future seam
// — e.g. a `--http` flag on the built command — and is not implemented yet.
package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Server configures the command ServerCmd builds. Only App is required.
type Server struct {
	// App is the composed server to run. Required.
	App *mcpkit.App

	// Use is the command name. Defaults to "server".
	Use string

	// Short is the one-line help text. A sensible default is used if empty.
	Short string

	// OnStart runs before the transport starts (e.g. open a DB connection).
	// Nil-safe. An error aborts before the server runs and before
	// OnShutdown runs.
	OnStart func(context.Context) error

	// OnShutdown runs after the server stops, on every path where the
	// server actually started. Nil-safe.
	OnShutdown func(context.Context) error

	// Flags registers consumer flags on the built command. Nil-safe.
	Flags func(*pflag.FlagSet)

	// Subcommands are attached under the built command via AddCommand.
	// Each entry may carry its own subtree of any depth — cobra's
	// AddCommand nests freely, so grandchildren (and deeper) work the same
	// as leaves; nothing here flattens them.
	Subcommands []*cobra.Command
}

// ServerCmd builds the server command: runs App.Run(ctx) with signal-driven
// graceful shutdown, calling OnStart before and OnShutdown after.
func ServerCmd(s Server) *cobra.Command {
	use := s.Use
	if use == "" {
		use = "server"
	}

	short := s.Short
	if short == "" {
		short = "Run the MCP server over the configured transport"
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runLifecycle(ctx, s.App.Run, s.OnStart, s.OnShutdown)
		},
	}

	if s.Flags != nil {
		s.Flags(cmd.Flags())
	}

	for _, sub := range s.Subcommands {
		cmd.AddCommand(sub)
	}

	return cmd
}

// runLifecycle drives the start->run->shutdown sequence, App-free so tests
// can exercise it with fake funcs. Order: onStart (if non-nil; an error
// aborts here, skipping run and onShutdown) -> run -> onShutdown (if
// non-nil, always, once run has started, with a context that outlives ctx's
// cancellation).
//
// A ctx-cancellation exit is treated as clean: if run's error is (or wraps)
// context.Canceled, or ctx was already cancelled by the time run returns,
// the run is not treated as a failure. The result joins (via errors.Join)
// the run failure, if any real one occurred, with the onShutdown error, if
// any — so a double failure (run fails for real AND onShutdown fails)
// surfaces both instead of one silently swallowing the other. A clean run
// with a nil onShutdown yields a nil result; errors.Is still matches either
// wrapped error individually.
func runLifecycle(ctx context.Context, run, onStart, onShutdown func(context.Context) error) error {
	if onStart != nil {
		if err := onStart(ctx); err != nil {
			return err
		}
	}

	runErr := run(ctx)

	var shutdownErr error
	if onShutdown != nil {
		shutdownErr = onShutdown(context.WithoutCancel(ctx))
	}

	var runFail error
	if runErr != nil && !errors.Is(runErr, context.Canceled) && ctx.Err() == nil {
		runFail = runErr
	}

	return errors.Join(runFail, shutdownErr)
}

// UseAsDefault makes bare invocation of root (no subcommand) run cmd: it
// sets root.RunE = cmd.RunE and copies cmd's flags onto root's local
// FlagSet. The flag copy matters because cobra parses argv against the
// FlagSet of whichever command is actually executing — on bare invocation
// that's root, not cmd — so without it any flag registered via Server.Flags
// (e.g. a future --http) would be rejected as unknown on bare invocation.
//
// Call UseAsDefault AFTER ServerCmd, once cmd.Flags() already carries the
// flags Server.Flags registered. The normal pattern keeps the server
// reachable both ways:
//
//	sc := cli.ServerCmd(s)
//	root.AddCommand(sc)
//	cli.UseAsDefault(root, sc)
//
// so `myapp server --foo` and bare `myapp --foo` both work. Without
// UseAsDefault, bare invocation shows help (cobra's default).
func UseAsDefault(root, cmd *cobra.Command) {
	root.RunE = cmd.RunE
	root.Flags().AddFlagSet(cmd.Flags())
}
