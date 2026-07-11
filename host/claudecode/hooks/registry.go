package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Handler processes one Claude Code hook invocation: ctx carries the
// command's context, in is the raw stdin bytes (already fully read and
// re-wrapped, so a handler needing fields Common doesn't expose can
// re-decode), and payload is the typed, already-decoded stdin JSON for
// this event. The returned Response is written to stdout by the built
// command (see Command); a Handler never writes stdout/stderr itself for
// control flow.
type Handler[P any] func(ctx context.Context, in io.Reader, payload P) Response

// Registry is a builder-style collection of per-event Handlers. Register
// only the events a consumer uses — Command builds a leaf subcommand only
// for a registered event, so an unregistered event has no `hooks <event>`
// command at all.
type Registry struct {
	stop             Handler[StopPayload]
	subagentStop     Handler[SubagentStopPayload]
	subagentStart    Handler[SubagentStartPayload]
	userPromptSubmit Handler[UserPromptSubmitPayload]
	preToolUse       Handler[PreToolUsePayload]
	postToolUse      Handler[PostToolUsePayload]
	preCompact       Handler[PreCompactPayload]
	sessionStart     Handler[SessionStartPayload]
}

// NewRegistry returns an empty Registry with no events registered.
func NewRegistry() *Registry {
	return &Registry{}
}

// Stop registers the Stop event handler and returns r for chaining.
func (r *Registry) Stop(h Handler[StopPayload]) *Registry {
	r.stop = h
	return r
}

// SubagentStop registers the SubagentStop event handler and returns r for
// chaining.
func (r *Registry) SubagentStop(h Handler[SubagentStopPayload]) *Registry {
	r.subagentStop = h
	return r
}

// SubagentStart registers the SubagentStart event handler and returns r
// for chaining.
func (r *Registry) SubagentStart(h Handler[SubagentStartPayload]) *Registry {
	r.subagentStart = h
	return r
}

// UserPromptSubmit registers the UserPromptSubmit event handler and
// returns r for chaining.
func (r *Registry) UserPromptSubmit(h Handler[UserPromptSubmitPayload]) *Registry {
	r.userPromptSubmit = h
	return r
}

// PreToolUse registers the PreToolUse event handler and returns r for
// chaining.
func (r *Registry) PreToolUse(h Handler[PreToolUsePayload]) *Registry {
	r.preToolUse = h
	return r
}

// PostToolUse registers the PostToolUse event handler and returns r for
// chaining.
func (r *Registry) PostToolUse(h Handler[PostToolUsePayload]) *Registry {
	r.postToolUse = h
	return r
}

// PreCompact registers the PreCompact event handler and returns r for
// chaining.
func (r *Registry) PreCompact(h Handler[PreCompactPayload]) *Registry {
	r.preCompact = h
	return r
}

// SessionStart registers the SessionStart event handler and returns r for
// chaining.
func (r *Registry) SessionStart(h Handler[SessionStartPayload]) *Registry {
	r.sessionStart = h
	return r
}

// FailOpen calls fn, recovering any panic and swallowing any returned
// error — logging either to stderr — so it always returns nil. A cobra
// leaf's RunE wraps its whole body in FailOpen so a hook's failure (decode
// error, handler panic, handler error) never surfaces as a non-zero exit,
// which would block the Claude Code session the hook is wired into.
func FailOpen(fn func() error) error {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "claude hooks: handler panicked: %v\n", p)
		}
	}()

	if err := fn(); err != nil {
		fmt.Fprintf(os.Stderr, "claude hooks: handler error: %v\n", err)
	}

	return nil
}
