package statusline

import (
	"fmt"
	"io"
	"os"

	"github.com/dangernoodle-io/mcpkit/jsonutil"
	"github.com/spf13/cobra"
)

// Option configures Command.
type Option func(*options)

type options struct {
	appPrefix string
}

// WithAppPrefix sets the consumer's env-var prefix (e.g. "OUROBOROS",
// "POGOPIN") used by Resolve's highest-precedence session-id override
// (<appPrefix>_SESSION_ID). Omit it to skip that tier — Resolve then falls
// back to the stdin payload's session_id, then CLAUDE_CODE_SESSION_ID.
func WithAppPrefix(prefix string) Option {
	return func(o *options) { o.appPrefix = prefix }
}

// Command builds the `statusline` leaf: reads the Claude Code statusLine
// stdin JSON, resolves session identity (Resolve), calls provider, renders
// the returned Segments via Render, and writes the single rendered line to
// stdout. Fully fail-open — mirrors pogopin's posture (BR-76): bad/empty
// stdin, an unresolved session, or a provider error all render nothing
// rather than a non-zero exit, since a non-nil cobra error here would print
// a usage line, never appropriate for a statusline widget. RunE always
// returns nil.
func Command(provider StatuslineProvider, opts ...Option) *cobra.Command {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	var plain bool

	cmd := &cobra.Command{
		Use:   "statusline",
		Short: "Render the Claude Code status line",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failOpen(func() error {
				return run(cmd, provider, cfg, plain)
			})
		},
	}

	cmd.Flags().BoolVar(&plain, "plain", false, "render without ANSI color escapes")

	return cmd
}

func run(cmd *cobra.Command, provider StatuslineProvider, cfg options, plain bool) error {
	payload := decodeStdin(cmd.InOrStdin())
	sessionID := Resolve(payload, cfg.appPrefix)

	segments, err := provider.Statusline(cmd.Context(), payload, sessionID)
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		return nil
	}

	line := Render(segments, RenderOptions{Plain: plain})
	if line == "" {
		return nil
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), line)
	return err
}

// decodeStdin reads and decodes the Claude Code statusLine stdin contract
// from r. Fail-open: empty stdin or unparseable JSON both yield a
// zero-value Payload rather than an error — Resolve and providers all
// degrade gracefully from an empty Payload.
func decodeStdin(r io.Reader) Payload {
	var payload Payload

	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return payload
	}

	if err := jsonutil.Unmarshal(data, &payload); err != nil {
		return Payload{}
	}

	return payload
}

// failOpen calls fn, recovering any panic and swallowing any returned
// error — logging either to stderr — so it always returns nil. Command's
// RunE wraps its whole body in failOpen so a statusline failure (decode
// error, provider panic, provider error) never surfaces as a non-zero
// exit or cobra usage text.
func failOpen(fn func() error) error {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "claude statusline: panicked: %v\n", p)
		}
	}()

	if err := fn(); err != nil {
		fmt.Fprintf(os.Stderr, "claude statusline: error: %v\n", err)
	}

	return nil
}
