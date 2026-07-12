package mcpkit

import (
	"context"
	"testing"

	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/stretchr/testify/require"
)

// TestRegistryFinalizeIdempotent proves finalize registers each pending
// tool exactly once even when called twice (the MC-43 guard Run+Connect,
// or Run+Run, both rely on).
func TestRegistryFinalizeIdempotent(t *testing.T) {
	reg := &registry{}

	calls := 0
	reg.add(pendingTool{name: "t1", register: func(_ *mcpx.Server) { calls++ }})

	reg.finalize(nil)
	reg.finalize(nil)

	require.Equal(t, 1, calls, "finalize must not re-register a pending tool on a second call")
	require.True(t, reg.started)
}

// TestRegistryByGroup proves finalize buckets registered tool names by
// group, including the "" (ungrouped) bucket, and never touches a group a
// tool wasn't assigned to.
func TestRegistryByGroup(t *testing.T) {
	reg := &registry{}
	reg.add(pendingTool{name: "a", group: "g1", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "b", group: "g1", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "c", group: "g2", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "d", register: func(_ *mcpx.Server) {}})

	reg.finalize(nil)

	require.ElementsMatch(t, []string{"a", "b"}, reg.byGroup["g1"])
	require.ElementsMatch(t, []string{"c"}, reg.byGroup["g2"])
	require.ElementsMatch(t, []string{"d"}, reg.byGroup[""])
	require.Len(t, reg.byGroup, 3, "no extra/ghost group buckets")
}

type groupedCap struct{}

func (groupedCap) Attach(r *Registrar) error {
	AddTool(r, &mcpx.Tool{Name: "grouped-a", Description: "a"}, ReadOnly,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		}, Group("alpha"))
	AddTool(r, &mcpx.Tool{Name: "grouped-b", Description: "b"}, Write,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		}, Group("alpha"))
	AddTool(r, &mcpx.Tool{Name: "ungrouped", Description: "c"}, ReadOnly,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		})
	return nil
}

// TestDeferredRegistrationPreFinalize proves registration is genuinely
// deferred: right after New (capabilities have already called AddTool),
// the registry holds pending entries but has not started, i.e. nothing has
// registered against the live server yet. This is asserted at the registry
// (the actual seam MC-43 introduces) rather than through the wire protocol,
// because connecting a session to observe tools/list would itself trigger
// finalize — the very thing under test.
func TestDeferredRegistrationPreFinalize(t *testing.T) {
	app, err := New(Info{Name: "deferred", Version: "0.0.1"}, generic.New(), groupedCap{})
	require.NoError(t, err)

	require.False(t, app.reg.started, "registration must not have run before finalize")
	require.Len(t, app.reg.pending, 3, "AddTool must capture pending tools without registering them")
	require.Empty(t, app.reg.byGroup, "byGroup is only populated by finalize")

	app.finalize()

	require.True(t, app.reg.started)
	require.ElementsMatch(t, []string{"grouped-a", "grouped-b"}, app.reg.byGroup["alpha"])
	require.ElementsMatch(t, []string{"ungrouped"}, app.reg.byGroup[""])
}

// TestAppFinalizeIdempotent proves App.finalize itself is guarded: calling
// it directly more than once (the same path Run/Connect/HTTPHandler share)
// does not re-register tools against the live mcpx.Server, which would
// otherwise panic on a duplicate tool name.
func TestAppFinalizeIdempotent(t *testing.T) {
	app, err := New(Info{Name: "idempotent", Version: "0.0.1"}, generic.New(), groupedCap{})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		app.finalize()
		app.finalize()
		app.finalize()
	})
}
