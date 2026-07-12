package mcpkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/dangernoodle-io/mcpkit/testkit"
	"github.com/stretchr/testify/require"
)

type lockIn struct{}

type lockOut struct{}

func lockHandler(_ context.Context, _ *mcpx.CallToolRequest, _ lockIn) (*mcpx.CallToolResult, lockOut, error) {
	return nil, lockOut{}, nil
}

// hwGroupCap registers one tool in group "hw" and one ungrouped tool.
type hwGroupCap struct{}

func (hwGroupCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{Name: "hw-tool", Description: "d"}, mcpkit.ReadOnly, lockHandler, mcpkit.Group("hw"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "ungrouped-tool", Description: "d"}, mcpkit.ReadOnly, lockHandler)
	return nil
}

// TestLockBeforeConnectThenUnlockAtRuntime proves the lazy-tier mechanism:
// locking a group before the app ever connects keeps that group's tools out
// of the very first tools/list, and a runtime Unlock later brings them in
// and notifies the connected client via notifications/tools/list_changed
// (observed here via testkit.AssertToolListChanged, MC-47).
func TestLockBeforeConnectThenUnlockAtRuntime(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "lazy-tier", Version: "0.0.1"}, generic.New(), hwGroupCap{})
	require.NoError(t, err)

	require.NoError(t, app.Lock("hw"))

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "ungrouped-tool")

	require.NoError(t, app.Unlock("hw"))

	testkit.AssertToolListChanged(t, h, 5*time.Second)

	testkit.AssertToolSet(t, h, "hw-tool", "ungrouped-tool")
}

// TestLockAtRuntimeUnregistersTool proves a runtime Lock (called after the
// app is already connected) truly unregisters the group's tools, not just
// hides them: the tool disappears from tools/list, and calling it directly
// by name is rejected by the server.
func TestLockAtRuntimeUnregistersTool(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "runtime-lock", Version: "0.0.1"}, generic.New(), hwGroupCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "hw-tool", "ungrouped-tool")

	require.NoError(t, app.Lock("hw"))

	testkit.AssertToolSet(t, h, "ungrouped-tool")

	_, err = h.CallTool(context.Background(), "hw-tool", map[string]any{})
	require.Error(t, err, "a locked-off tool must be truly unregistered, not merely hidden")
}

// TestLockHardBlockedGroupErrors proves the startup gate (MC-44 BlockGroups)
// takes precedence over MC-45's runtime lock: both Lock and Unlock on a
// hard-blocked group return an error, and the tool set is unaffected by
// either call.
func TestLockHardBlockedGroupErrors(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "hard-block", Version: "0.0.1"}, generic.New(), hwGroupCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockGroups("hw"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ungrouped-tool")

	require.Error(t, app.Lock("hw"))
	testkit.AssertToolSet(t, h, "ungrouped-tool")

	require.Error(t, app.Unlock("hw"))
	testkit.AssertToolSet(t, h, "ungrouped-tool")
}

type readOnlyGuardIn struct{}

type readOnlyGuardOut struct{}

func readOnlyGuardHandler(_ context.Context, _ *mcpx.CallToolRequest, _ readOnlyGuardIn) (*mcpx.CallToolResult, readOnlyGuardOut, error) {
	return nil, readOnlyGuardOut{}, nil
}

// mixedRiskGroupCap registers a ReadOnly and a Write tool in the same group,
// so Unlock's gate re-check can be observed acting on one but not the other.
type mixedRiskGroupCap struct{}

func (mixedRiskGroupCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{Name: "hw-read", Description: "d"}, mcpkit.ReadOnly, readOnlyGuardHandler, mcpkit.Group("hw"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "hw-write", Description: "d"}, mcpkit.Write, readOnlyGuardHandler, mcpkit.Group("hw"))
	return nil
}

// TestUnlockDoesNotResurrectReadOnlyGateBlockedTool proves Unlock's
// shouldRegister re-check is genuine: under ReadOnlyMode, a Write tool
// gate-blocked at finalize is never brought back by Unlock, even though a
// ReadOnly tool in the very same (previously locked) group is.
func TestUnlockDoesNotResurrectReadOnlyGateBlockedTool(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "ro-guard", Version: "0.0.1"}, generic.New(), mixedRiskGroupCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(mcpkit.ReadOnlyMode()))

	h := testkit.New(t, app)
	// hw-write is gate-blocked at finalize and never registers; hw-read
	// (ReadOnly) does.
	testkit.AssertToolSet(t, h, "hw-read")

	// Lock then Unlock the group at runtime so Unlock's re-registration loop
	// (not finalize's) is what's under test.
	require.NoError(t, app.Lock("hw"))
	testkit.AssertToolSet(t, h)

	require.NoError(t, app.Unlock("hw"))
	testkit.AssertToolSet(t, h, "hw-read")
}

// TestLockUnlockIdempotency proves double Lock and double Unlock calls are
// safe no-ops: neither panics, and neither double-registers or leaves the
// tool set in an unexpected state.
func TestLockUnlockIdempotency(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "idempotent-lock", Version: "0.0.1"}, generic.New(), hwGroupCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "hw-tool", "ungrouped-tool")

	require.NotPanics(t, func() {
		require.NoError(t, app.Lock("hw"))
		require.NoError(t, app.Lock("hw"))
	})
	testkit.AssertToolSet(t, h, "ungrouped-tool")

	require.NotPanics(t, func() {
		require.NoError(t, app.Unlock("hw"))
		require.NoError(t, app.Unlock("hw"))
	})
	testkit.AssertToolSet(t, h, "hw-tool", "ungrouped-tool")
}
