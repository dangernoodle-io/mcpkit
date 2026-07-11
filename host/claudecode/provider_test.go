package claudecode_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dangernoodle-io/mcpkit/cli"
	"github.com/dangernoodle-io/mcpkit/host/claudecode"
	"github.com/dangernoodle-io/mcpkit/host/claudecode/hooks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewProvider_CommandsReturnsClaudeTree proves Commands() returns the
// single `claude` namespace command holding `hooks`.
func TestNewProvider_CommandsReturnsClaudeTree(t *testing.T) {
	reg := hooks.NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ hooks.StopPayload) hooks.Response {
		return hooks.Response{}
	})

	p := claudecode.NewProvider(reg)
	cmds := p.Commands()

	require.Len(t, cmds, 1)
	assert.Equal(t, "claude", cmds[0].Use)

	names := map[string]bool{}
	for _, c := range cmds[0].Commands() {
		names[c.Use] = true
	}
	assert.True(t, names["hooks"])
}

// TestNewProvider_ExtraSubtreesAppendUnderClaude proves the extension
// point: extra cobra.Command trees passed to NewProvider mount alongside
// hooks under the same `claude` namespace (the seam a later statusline PR
// uses).
func TestNewProvider_ExtraSubtreesAppendUnderClaude(t *testing.T) {
	extra := &cobra.Command{Use: "statusline"}

	p := claudecode.NewProvider(hooks.NewRegistry(), extra)
	cmds := p.Commands()

	require.Len(t, cmds, 1)

	names := map[string]bool{}
	for _, c := range cmds[0].Commands() {
		names[c.Use] = true
	}
	assert.True(t, names["hooks"])
	assert.True(t, names["statusline"])
}

// TestMountProviders_EndToEndDispatch proves the full mount path: root ->
// cli.MountProviders(root, claudecode.NewProvider(reg)) -> `root claude
// hooks stop` dispatches to the registered handler and its Response
// reaches stdout, fed via real stdin bytes.
func TestMountProviders_EndToEndDispatch(t *testing.T) {
	var gotSessionID string
	reg := hooks.NewRegistry().Stop(func(_ context.Context, _ io.Reader, p hooks.StopPayload) hooks.Response {
		gotSessionID = p.SessionID
		return hooks.Response{AdditionalContext: "seen"}
	})

	root := &cobra.Command{Use: "root"}
	cli.MountProviders(root, claudecode.NewProvider(reg))

	var out bytes.Buffer
	root.SetArgs([]string{"claude", "hooks", "stop"})
	root.SetIn(strings.NewReader(`{"session_id":"abc123"}`))
	root.SetOut(&out)

	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "abc123", gotSessionID)
	assert.Contains(t, out.String(), `"hookEventName":"Stop"`)
	assert.Contains(t, out.String(), `"additionalContext":"seen"`)
}
