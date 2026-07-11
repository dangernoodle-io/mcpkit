package cli

import "github.com/spf13/cobra"

// CommandProvider is implemented by anything that contributes cobra commands
// to a consumer's root command — e.g. a host adapter (like the Claude Code
// adapter) or, in the future, a capability. Consumers declare providers and
// call MountProviders instead of hand-wiring AddCommand.
type CommandProvider interface {
	// Commands returns the top-level commands this provider contributes.
	// Each returned command may carry its own subtree of any depth — cobra's
	// AddCommand nests freely, so a provider is free to build a multi-level
	// command tree (e.g. a host namespace via HostNamespaceCmd) before
	// returning it here.
	Commands() []*cobra.Command
}

// MountProviders mounts every provider's Commands() onto root via
// AddCommand, in order. A nil provider is skipped safely. Command-name
// collisions across providers are NOT de-duplicated — cobra's AddCommand
// appends both — so callers should compose distinct namespaces or
// otherwise non-colliding command names.
func MountProviders(root *cobra.Command, providers ...CommandProvider) {
	for _, p := range providers {
		if p == nil {
			continue
		}

		root.AddCommand(p.Commands()...)
	}
}

// HostNamespaceCmd builds a run-less parent command named name with subs
// attached via AddCommand — the shape used to group a host's commands under
// one namespace (e.g. a `claude` command holding `hooks`/`statusline`).
// Invoking name alone (with no further subcommand) shows cobra's default
// help; there is no RunE.
func HostNamespaceCmd(name string, subs ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Commands for the " + name + " host",
	}

	cmd.AddCommand(subs...)

	return cmd
}
