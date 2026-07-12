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

// TestRegistryFinalizeGateExcludesBlockedFromByGroup proves the MC-44 gate
// predicate in finalize is a true hard block: a gated-off tool's register
// closure is never invoked (never reaches the live server) and its name
// never lands in byGroup, so a startup-blocked tool can't later be
// resurrected by MC-45's runtime Unlock (which will only ever see byGroup).
func TestRegistryFinalizeGateExcludesBlockedFromByGroup(t *testing.T) {
	registered := map[string]bool{}

	reg := &registry{
		gate: gateState{
			readOnly:      true,
			blockedGroups: map[string]bool{"x": true},
		},
	}
	reg.add(pendingTool{name: "ro", group: "a", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro"] = true }})
	reg.add(pendingTool{name: "write", group: "a", risk: Write, register: func(_ *mcpx.Server) { registered["write"] = true }})
	reg.add(pendingTool{name: "ro-x", group: "x", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro-x"] = true }})

	reg.finalize(nil)

	require.True(t, registered["ro"], "an allowed tool must still register")
	require.False(t, registered["write"], "ReadOnlyMode must gate off a non-ReadOnly tool before register runs")
	require.False(t, registered["ro-x"], "BlockGroups must gate off a blocked-group tool before register runs")

	require.Equal(t, []string{"ro"}, reg.byGroup["a"])
	require.Empty(t, reg.byGroup["x"], "byGroup must not track a gated-off tool")
}

// TestRegistryApplyGateAfterStartedErrors proves applyGate/blockGroups are
// pre-start-only: once finalize has run, mutating the gate can no longer
// affect the already-registered set, so both return an error instead of a
// silent no-op.
func TestRegistryApplyGateAfterStartedErrors(t *testing.T) {
	reg := &registry{}
	reg.finalize(nil)

	err := reg.applyGate(ReadOnlyMode())
	require.Error(t, err)

	err = reg.blockGroups("x")
	require.Error(t, err)
}

// TestRegistryApplyGateBeforeStarted proves applyGate/blockGroups succeed
// pre-finalize and actually mutate reg.gate.
func TestRegistryApplyGateBeforeStarted(t *testing.T) {
	reg := &registry{}

	require.NoError(t, reg.applyGate(ReadOnlyMode()))
	require.True(t, reg.gate.readOnly)

	require.NoError(t, reg.blockGroups("x", "y"))
	require.True(t, reg.gate.blockedGroups["x"])
	require.True(t, reg.gate.blockedGroups["y"])
}
