package testkit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dangernoodle-io/mcpkit/mcpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResultText concatenates the text content of a CallToolResult.
var ResultText = mcpx.ResultText

// DecodeToolResult json-unmarshals a tool result's text content into T,
// failing the test on decode error.
func DecodeToolResult[T any](t testing.TB, res *mcpx.CallToolResult) T {
	t.Helper()
	var out T
	err := json.Unmarshal([]byte(mcpx.ResultText(res)), &out)
	require.NoError(t, err, "decode tool result")
	return out
}

// EventuallyContains polls fn until it returns a slice containing want, or
// fails the test after timeout.
func EventuallyContains(t testing.TB, timeout, interval time.Duration, fn func() []string, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, got := range fn() {
			if got == want {
				return true
			}
		}
		return false
	}, timeout, interval, "expected %q to eventually appear", want)
}

// AssertToolSet asserts that the app's advertised tools/list is exactly
// want, guarding against silent drift.
func AssertToolSet(t testing.TB, h *Harness, want ...string) {
	t.Helper()
	res, err := h.ListTools(context.Background())
	require.NoError(t, err, "list tools")

	got := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	assert.ElementsMatch(t, want, got, "tool set drift")
}
