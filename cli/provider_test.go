package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a minimal CommandProvider for tests.
type fakeProvider struct {
	cmds []*cobra.Command
}

func (f fakeProvider) Commands() []*cobra.Command { return f.cmds }

func TestMountProviders_MountsAllProvidersCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	p1 := fakeProvider{cmds: []*cobra.Command{
		{Use: "one", RunE: func(*cobra.Command, []string) error { return nil }},
	}}
	p2 := fakeProvider{cmds: []*cobra.Command{
		{Use: "two", RunE: func(*cobra.Command, []string) error { return nil }},
		{Use: "three", RunE: func(*cobra.Command, []string) error { return nil }},
	}}

	MountProviders(root, p1, p2)

	names := make(map[string]bool)
	for _, c := range root.Commands() {
		names[c.Use] = true
	}

	assert.True(t, names["one"], "provider 1's command must be mounted")
	assert.True(t, names["two"], "provider 2's first command must be mounted")
	assert.True(t, names["three"], "provider 2's second command must be mounted")
}

func TestHostNamespaceCmd_BuildsRunlessParentWithChildren(t *testing.T) {
	leaf1 := &cobra.Command{Use: "leaf1", RunE: func(*cobra.Command, []string) error { return nil }}
	leaf2 := &cobra.Command{Use: "leaf2", RunE: func(*cobra.Command, []string) error { return nil }}

	ns := HostNamespaceCmd("claude", leaf1, leaf2)

	assert.Equal(t, "claude", ns.Use)
	assert.NotEmpty(t, ns.Short)
	assert.Nil(t, ns.RunE, "the namespace command itself must not run")

	found := map[string]bool{}
	for _, c := range ns.Commands() {
		found[c.Use] = true
	}

	assert.True(t, found["leaf1"])
	assert.True(t, found["leaf2"])
}

func TestMountProviders_NilProviderIsSkippedSafely(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	p := fakeProvider{cmds: []*cobra.Command{
		{Use: "one", RunE: func(*cobra.Command, []string) error { return nil }},
	}}

	assert.NotPanics(t, func() {
		MountProviders(root, nil, p)
	})

	found := false
	for _, c := range root.Commands() {
		if c.Use == "one" {
			found = true
		}
	}
	assert.True(t, found, "the non-nil provider's command must still mount")
}

func TestMountProviders_ProviderWithNoCommandsIsNoop(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	empty := fakeProvider{}

	assert.NotPanics(t, func() {
		MountProviders(root, empty)
	})
	assert.Empty(t, root.Commands())
}

// TestMountProviders_DispatchesThroughNestedNamespace proves the seam
// supports multi-level dispatch end-to-end: root -> HostNamespaceCmd("ns")
// -> leaf, executed via root.Execute() with args targeting the leaf.
func TestMountProviders_DispatchesThroughNestedNamespace(t *testing.T) {
	ran := false
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(*cobra.Command, []string) error {
			ran = true
			return nil
		},
	}

	ns := HostNamespaceCmd("ns", leaf)

	root := &cobra.Command{Use: "root"}
	MountProviders(root, fakeProvider{cmds: []*cobra.Command{ns}})

	root.SetArgs([]string{"ns", "leaf"})
	err := root.Execute()

	require.NoError(t, err)
	assert.True(t, ran, "the nested leaf's RunE must execute via root.Execute()")
}
