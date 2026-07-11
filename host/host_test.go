package host_test

import (
	"testing"

	"github.com/dangernoodle-io/mcpkit/host"
	"github.com/dangernoodle-io/mcpkit/host/claudecode"
	"github.com/dangernoodle-io/mcpkit/host/cursor"
	"github.com/dangernoodle-io/mcpkit/host/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapters(t *testing.T) {
	cases := []struct {
		name    string
		adapter host.Adapter
		want    string
	}{
		{"generic", generic.New(), "generic"},
		{"claudecode", claudecode.New(), "claude-code"},
		{"cursor", cursor.New(), "cursor"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.adapter.Name())
			require.NotNil(t, tc.adapter.Transport())
		})
	}
}
