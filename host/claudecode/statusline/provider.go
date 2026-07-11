package statusline

import "context"

// StatuslineProvider is implemented by a consumer to supply the Segments a
// `claude statusline` invocation renders. Command decodes the Claude Code
// statusLine stdin contract, resolves session identity via Resolve, then
// calls Statusline — the provider stays DB/domain-agnostic to this
// package: it returns Segments and Render (not the provider) owns all
// color-profile degradation, so a provider never has to think about
// termenv, TTY detection, or NO_COLOR.
//
// A provider returning (nil, nil) — or any error — renders nothing: the
// shape both known consumers need for their "print nothing" cases
// (ouroboros: KB+backlog both zero; pogopin: no live ports outside
// ModeAlways).
type StatuslineProvider interface {
	Statusline(ctx context.Context, payload Payload, sessionID string) ([]Segment, error)
}

// StatuslineProviderFunc adapts a plain function to StatuslineProvider, the
// way http.HandlerFunc adapts a function to http.Handler.
type StatuslineProviderFunc func(ctx context.Context, payload Payload, sessionID string) ([]Segment, error)

// Statusline implements StatuslineProvider.
func (f StatuslineProviderFunc) Statusline(
	ctx context.Context, payload Payload, sessionID string,
) ([]Segment, error) {
	return f(ctx, payload, sessionID)
}
