package hooks

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFailOpen_HappyPath proves a non-panicking, non-erroring fn returns
// nil and FailOpen is transparent.
func TestFailOpen_HappyPath(t *testing.T) {
	called := false

	err := FailOpen(func() error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

// TestFailOpen_SwallowsError proves a returned error is swallowed (logged,
// not propagated) — FailOpen still returns nil.
func TestFailOpen_SwallowsError(t *testing.T) {
	err := FailOpen(func() error {
		return errors.New("boom")
	})

	require.NoError(t, err, "FailOpen must swallow a returned error")
}

// TestFailOpen_RecoversPanic proves a panicking fn is recovered and
// FailOpen still returns nil rather than propagating the panic.
func TestFailOpen_RecoversPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		err := FailOpen(func() error {
			panic("kaboom")
		})
		assert.NoError(t, err, "FailOpen must recover a panic and return nil")
	})
}

// TestRegistry_BuilderChainsAndDefaultsToUnregistered proves NewRegistry
// starts empty and each builder method registers exactly its own event
// without disturbing the others.
func TestRegistry_BuilderChainsAndDefaultsToUnregistered(t *testing.T) {
	reg := NewRegistry()

	assert.Nil(t, reg.stop)
	assert.Nil(t, reg.subagentStop)
	assert.Nil(t, reg.subagentStart)
	assert.Nil(t, reg.userPromptSubmit)
	assert.Nil(t, reg.preToolUse)
	assert.Nil(t, reg.postToolUse)
	assert.Nil(t, reg.preCompact)
	assert.Nil(t, reg.sessionStart)

	got := reg.
		Stop(func(_ context.Context, _ io.Reader, _ StopPayload) Response { return Response{} }).
		SessionStart(func(_ context.Context, _ io.Reader, _ SessionStartPayload) Response { return Response{} })

	assert.Same(t, reg, got, "each builder method must return the same *Registry for chaining")
	assert.NotNil(t, reg.stop)
	assert.NotNil(t, reg.sessionStart)
	assert.Nil(t, reg.subagentStop, "registering Stop/SessionStart must not register unrelated events")
}
