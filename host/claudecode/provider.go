package claudecode

import (
	"github.com/dangernoodle-io/mcpkit/cli"
	"github.com/dangernoodle-io/mcpkit/host/claudecode/hooks"
	"github.com/spf13/cobra"
)

// NewProvider returns a cli.CommandProvider contributing the `claude` host
// namespace — "everything Claude Code's plugin protocol invokes against
// this binary." Today that's just `claude hooks`, built from reg.
//
// extra is a forward-compatible extension point for the `claude`
// namespace's other subtrees; a later PR adds `claude statusline` by
// passing statusline.Command(provider) here (or a subsequent signature
// grows a dedicated statusline parameter once that package exists) —
// either way this constructor's shape does not need to change to widen
// the `claude` tree. Pass nothing for hooks-only use.
func NewProvider(reg *hooks.Registry, extra ...*cobra.Command) cli.CommandProvider {
	return provider{reg: reg, extra: extra}
}

type provider struct {
	reg   *hooks.Registry
	extra []*cobra.Command
}

// Commands implements cli.CommandProvider: it returns the single `claude`
// namespace command, holding `hooks` (always) plus any extra subtrees
// passed to NewProvider.
func (p provider) Commands() []*cobra.Command {
	subs := make([]*cobra.Command, 0, 1+len(p.extra))
	subs = append(subs, hooks.Command(p.reg))
	subs = append(subs, p.extra...)

	return []*cobra.Command{cli.HostNamespaceCmd("claude", subs...)}
}
