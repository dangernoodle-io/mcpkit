package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Mount describes where a command subtree attaches: Cmd mounts as a direct
// child of the command reached by walking Under, a parent path from root
// given by command name (cobra.Command.Name(), the first whitespace-
// delimited token of Use — not the full Use string). A nil or empty Under
// mounts Cmd directly under root.
//
// Example: Under: []string{"claude"} attaches Cmd under whatever command
// named "claude" already exists on root (built earlier, e.g. via
// HostNamespaceCmd) — the shape a provider uses to extend a tree it doesn't
// own without hand-walking cobra itself.
type Mount struct {
	// Under is the parent path from root, by command name, each segment
	// resolved via cobra's Command.Commands()/Name(). Nil/empty = root.
	Under []string

	// Cmd is the subtree attached under the resolved parent. A nil Cmd is
	// skipped safely — consistent with MountProviders' nil-provider skip —
	// so a conditional mount can pass nil rather than branch at the call
	// site.
	Cmd *cobra.Command
}

// CommandProvider is implemented by anything that contributes cobra commands
// to a consumer's root command — e.g. a host adapter (like the Claude Code
// adapter) or, in the future, a capability. Consumers declare providers and
// call MountProviders instead of hand-wiring AddCommand.
type CommandProvider interface {
	// Mounts returns the subtrees this provider contributes, each
	// self-declaring where it attaches via Mount.Under. A provider that only
	// ever mounts at root returns Mount{Cmd: ...} (nil Under).
	Mounts() []Mount
}

// MountAll resolves each mount's Under path by walking root's tree by
// command name and attaches Cmd under the resolved parent via AddCommand, in
// argument order. An Under segment that names no existing child returns an
// error immediately — a typo'd or premature mount fails loudly at startup
// rather than silently vanishing. A nil Cmd is skipped safely.
//
// Command-name collisions across mounts attached at the same parent are NOT
// de-duplicated — cobra's AddCommand appends both — so callers should
// compose distinct namespaces or otherwise non-colliding command names.
func MountAll(root *cobra.Command, mounts ...Mount) error {
	for _, m := range mounts {
		if m.Cmd == nil {
			continue
		}

		parent, err := resolveParent(root, m.Under)
		if err != nil {
			return err
		}

		parent.AddCommand(m.Cmd)
	}

	return nil
}

// resolveParent walks root by command name, one segment of under at a time,
// returning the command reached. An empty/nil under returns root unchanged.
func resolveParent(root *cobra.Command, under []string) (*cobra.Command, error) {
	parent := root
	walked := make([]string, 0, len(under))

	for _, seg := range under {
		var next *cobra.Command
		for _, c := range parent.Commands() {
			if c.Name() == seg {
				next = c
				break
			}
		}

		if next == nil {
			loc := "root"
			if len(walked) > 0 {
				loc = fmt.Sprintf("%q", strings.Join(walked, " "))
			}
			return nil, fmt.Errorf("cli: MountAll: no command %q under %s", seg, loc)
		}

		parent = next
		walked = append(walked, seg)
	}

	return parent, nil
}

// MountProviders mounts every provider's Mounts() onto root via MountAll, in
// order. A nil provider is skipped safely. Returns the first MountAll error
// encountered (an unresolved Under path).
func MountProviders(root *cobra.Command, providers ...CommandProvider) error {
	var mounts []Mount

	for _, p := range providers {
		if p == nil {
			continue
		}

		mounts = append(mounts, p.Mounts()...)
	}

	return MountAll(root, mounts...)
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
