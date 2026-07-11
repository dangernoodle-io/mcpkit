package mcpx_test

import (
	"testing"

	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/stretchr/testify/require"
)

// TestBoolPtr proves BoolPtr returns a pointer that dereferences to the
// given value, for populating ToolAnnotations' *bool hint fields without a
// go-sdk import.
func TestBoolPtr(t *testing.T) {
	p := mcpx.BoolPtr(true)
	require.NotNil(t, p)
	require.True(t, *p)

	p = mcpx.BoolPtr(false)
	require.NotNil(t, p)
	require.False(t, *p)
}

// TestToolAnnotationsAlias proves mcpx.ToolAnnotations is settable with the
// full hint field set a consumer needs, without importing go-sdk.
func TestToolAnnotationsAlias(t *testing.T) {
	ann := &mcpx.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: mcpx.BoolPtr(false),
		OpenWorldHint:   mcpx.BoolPtr(true),
	}

	require.True(t, ann.ReadOnlyHint)
	require.True(t, ann.IdempotentHint)
	require.NotNil(t, ann.DestructiveHint)
	require.False(t, *ann.DestructiveHint)
	require.NotNil(t, ann.OpenWorldHint)
	require.True(t, *ann.OpenWorldHint)
}
