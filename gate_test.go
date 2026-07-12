package mcpkit_test

import (
	"context"
	"testing"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/dangernoodle-io/mcpkit/testkit"
	"github.com/stretchr/testify/require"
)

type gateIn struct{}

type gateOut struct{}

func gateHandler(_ context.Context, _ *mcpx.CallToolRequest, _ gateIn) (*mcpx.CallToolResult, gateOut, error) {
	return nil, gateOut{}, nil
}

// riskGateCap registers one ungrouped tool per Risk value.
type riskGateCap struct{}

func (riskGateCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{Name: "ro", Description: "d"}, mcpkit.ReadOnly, gateHandler)
	mcpkit.AddTool(r, &mcpx.Tool{Name: "write", Description: "d"}, mcpkit.Write, gateHandler)
	mcpkit.AddTool(r, &mcpx.Tool{Name: "destructive", Description: "d"}, mcpkit.Destructive, gateHandler)
	return nil
}

// groupGateCap registers a ReadOnly tool in group "x", a ReadOnly tool in
// group "y", and an ungrouped ReadOnly tool.
type groupGateCap struct{}

func (groupGateCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{Name: "x-tool", Description: "d"}, mcpkit.ReadOnly, gateHandler, mcpkit.Group("x"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "y-tool", Description: "d"}, mcpkit.ReadOnly, gateHandler, mcpkit.Group("y"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "ungrouped", Description: "d"}, mcpkit.ReadOnly, gateHandler)
	return nil
}

// mixedGateCap combines the risk and group axes for the combined-gate test.
type mixedGateCap struct{}

func (mixedGateCap) Attach(r *mcpkit.Registrar) error {
	mcpkit.AddTool(r, &mcpx.Tool{Name: "ro-x", Description: "d"}, mcpkit.ReadOnly, gateHandler, mcpkit.Group("x"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "ro-y", Description: "d"}, mcpkit.ReadOnly, gateHandler, mcpkit.Group("y"))
	mcpkit.AddTool(r, &mcpx.Tool{Name: "write-y", Description: "d"}, mcpkit.Write, gateHandler, mcpkit.Group("y"))
	return nil
}

// TestGateReadOnlyModeBlocksNonReadOnly proves ReadOnlyMode hard-blocks
// every non-ReadOnly tool at startup: only the ReadOnly tool is advertised
// in tools/list once the app connects.
func TestGateReadOnlyModeBlocksNonReadOnly(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "gate-ro", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(mcpkit.ReadOnlyMode()))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro")
}

// TestBlockGroupsExcludesNamedGroup proves BlockGroups hard-blocks every
// tool tagged with a blocked group while leaving other groups/ungrouped
// tools advertised.
func TestBlockGroupsExcludesNamedGroup(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "gate-group", Version: "0.0.1"}, generic.New(), groupGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockGroups("x"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "y-tool", "ungrouped")
}

// TestGateReadOnlyAndBlockGroupsCombine proves the risk axis and the group
// axis intersect: a tool must pass both to be advertised.
func TestGateReadOnlyAndBlockGroupsCombine(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "gate-combo", Version: "0.0.1"}, generic.New(), mixedGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(mcpkit.ReadOnlyMode()))
	require.NoError(t, app.BlockGroups("y"))

	h := testkit.New(t, app)
	// ro-x: ReadOnly, group x (not blocked) -> advertised.
	// ro-y: ReadOnly, but group y (blocked) -> excluded.
	// write-y: Write (blocked by ReadOnlyMode) and group y (blocked) -> excluded.
	testkit.AssertToolSet(t, h, "ro-x")
}

// TestNoGateAdvertisesEverything is a regression guard: an App that never
// calls Gate/BlockGroups preserves MC-43's register-everything behavior.
func TestNoGateAdvertisesEverything(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "gate-none", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")
}

// TestGateAfterConnectErrors proves Gate/BlockGroups are pre-start-only:
// calling either after the App has already finalized registration (via
// Connect) returns an error and does not change the already-registered
// tool set.
func TestGateAfterConnectErrors(t *testing.T) {
	app, err := mcpkit.New(mcpkit.Info{Name: "gate-late", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")

	require.Error(t, app.Gate(mcpkit.ReadOnlyMode()))
	require.Error(t, app.BlockGroups("whatever"))

	// The already-registered set must be unchanged by the rejected calls.
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")
}
