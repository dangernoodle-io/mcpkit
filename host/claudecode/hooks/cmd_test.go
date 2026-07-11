package hooks

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errReader always fails, to exercise leaf's io.ReadAll error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestCommand_OnlyRegisteredEventsGetLeaves(t *testing.T) {
	reg := NewRegistry().
		Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response { return Response{} }).
		PreCompact(func(_ context.Context, _ io.Reader, _ PreCompactPayload) Response { return Response{} })

	cmd := Command(reg)

	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Use] = true
	}

	assert.True(t, names["stop"])
	assert.True(t, names["pre-compact"])
	assert.False(t, names["subagent-stop"], "unregistered event must have no leaf")
	assert.False(t, names["subagent-start"])
	assert.False(t, names["user-prompt-submit"])
	assert.False(t, names["pre-tool-use"])
	assert.False(t, names["post-tool-use"])
	assert.False(t, names["session-start"])
	assert.Len(t, cmd.Commands(), 2)
}

func TestCommand_NoEventsRegisteredHasNoLeaves(t *testing.T) {
	cmd := Command(NewRegistry())
	assert.Empty(t, cmd.Commands())
}

// runLeaf executes cmd's `hooks <use>` subcommand with stdin and returns
// stdout.
func runLeaf(t *testing.T, cmd *cobra.Command, use, stdin string) string {
	t.Helper()

	var out bytes.Buffer
	cmd.SetArgs([]string{use})
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)

	err := cmd.Execute()
	require.NoError(t, err)

	return out.String()
}

func TestCommand_StopDispatchesAndWritesResponse(t *testing.T) {
	var got StopPayload
	reg := NewRegistry().Stop(func(_ context.Context, _ io.Reader, p StopPayload) Response {
		got = p
		return Response{AdditionalContext: "hi"}
	})

	cmd := Command(reg)
	out := runLeaf(t, cmd, "stop", `{"session_id":"s1","stop_hook_active":true}`)

	assert.Equal(t, "s1", got.SessionID)
	assert.True(t, got.StopHookActive)
	assert.Contains(t, out, `"hookEventName":"Stop"`)
	assert.Contains(t, out, `"additionalContext":"hi"`)
}

func TestCommand_AllEventsDecodeAndDispatch(t *testing.T) {
	reg := NewRegistry().
		Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response { return Response{} }).
		SubagentStop(func(_ context.Context, _ io.Reader, p SubagentStopPayload) Response {
			assert.Equal(t, "agent-1", p.AgentID)
			return Response{}
		}).
		SubagentStart(func(_ context.Context, _ io.Reader, p SubagentStartPayload) Response {
			assert.Equal(t, "worker", p.AgentType)
			return Response{}
		}).
		UserPromptSubmit(func(_ context.Context, _ io.Reader, p UserPromptSubmitPayload) Response {
			assert.Equal(t, "hello", p.Prompt)
			return Response{}
		}).
		PreToolUse(func(_ context.Context, _ io.Reader, p PreToolUsePayload) Response {
			assert.Equal(t, "Edit", p.ToolName)
			return Response{}
		}).
		PostToolUse(func(_ context.Context, _ io.Reader, p PostToolUsePayload) Response {
			assert.Equal(t, "Bash", p.ToolName)
			return Response{}
		}).
		PreCompact(func(_ context.Context, _ io.Reader, p PreCompactPayload) Response {
			assert.Equal(t, "auto", p.Trigger)
			return Response{}
		}).
		SessionStart(func(_ context.Context, _ io.Reader, p SessionStartPayload) Response {
			assert.Equal(t, "startup", p.Source)
			return Response{}
		})

	cases := []struct {
		use   string
		stdin string
	}{
		{"stop", `{"session_id":"s"}`},
		{"subagent-stop", `{"agent_id":"agent-1"}`},
		{"subagent-start", `{"agent_type":"worker"}`},
		{"user-prompt-submit", `{"prompt":"hello"}`},
		{"pre-tool-use", `{"tool_name":"Edit"}`},
		{"post-tool-use", `{"tool_name":"Bash"}`},
		{"pre-compact", `{"trigger":"auto"}`},
		{"session-start", `{"source":"startup"}`},
	}

	for _, tc := range cases {
		t.Run(tc.use, func(t *testing.T) {
			cmd := Command(reg)
			out := runLeaf(t, cmd, tc.use, tc.stdin)
			assert.Empty(t, out, "handlers in this test all return the zero Response (silent allow)")
		})
	}
}

// TestCommand_MalformedStdinFailsOpen proves a stdin decode error never
// reaches the handler and never surfaces as a non-zero exit or crash.
func TestCommand_MalformedStdinFailsOpen(t *testing.T) {
	called := false
	reg := NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response {
		called = true
		return Response{}
	})

	cmd := Command(reg)
	out := runLeaf(t, cmd, "stop", `not json`)

	require.False(t, called, "the handler must not run when stdin fails to decode")
	assert.Empty(t, out)
}

// TestCommand_StdinReadErrorFailsOpen proves an io.ReadAll failure on
// stdin (distinct from a JSON-decode failure) also never reaches the
// handler and never surfaces as a non-zero exit.
func TestCommand_StdinReadErrorFailsOpen(t *testing.T) {
	called := false
	reg := NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response {
		called = true
		return Response{}
	})

	cmd := Command(reg)

	var out bytes.Buffer
	cmd.SetArgs([]string{"stop"})
	cmd.SetIn(errReader{})
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	require.False(t, called, "the handler must not run when stdin fails to read")
	assert.Empty(t, out.String())
}

// TestCommand_PanickingHandlerFailsOpen proves a handler panic is
// recovered at the built leaf: cmd.Execute() returns nil and no panic
// escapes.
func TestCommand_PanickingHandlerFailsOpen(t *testing.T) {
	reg := NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response {
		panic("kaboom")
	})

	cmd := Command(reg)

	assert.NotPanics(t, func() {
		out := runLeaf(t, cmd, "stop", `{}`)
		assert.Empty(t, out, "a recovered panic must not leave a partial Response on stdout")
	})
}
