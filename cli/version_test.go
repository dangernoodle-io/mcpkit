package cli_test

import (
	"bytes"
	"testing"

	"github.com/dangernoodle-io/mcpkit"
	"github.com/dangernoodle-io/mcpkit/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd(t *testing.T) {
	cmd := cli.VersionCmd(mcpkit.Info{Name: "acme", Version: "1.2.3"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Equal(t, "acme 1.2.3\n", out.String())
}
