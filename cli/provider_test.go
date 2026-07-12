package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a minimal CommandProvider for tests.
type fakeProvider struct {
	mounts []Mount
}

func (f fakeProvider) Mounts() []Mount { return f.mounts }

func newCmd(use string) *cobra.Command {
	return &cobra.Command{Use: use, RunE: func(*cobra.Command, []string) error { return nil }}
}

func TestMountAll_RootMount(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	err := MountAll(root, Mount{Cmd: newCmd("one")})

	require.NoError(t, err)

	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Use] = true
	}
	assert.True(t, names["one"])
}

func TestMountAll_OneLevelUnder(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	server := newCmd("server")
	root.AddCommand(server)

	err := MountAll(root, Mount{Under: []string{"server"}, Cmd: newCmd("sub")})

	require.NoError(t, err)

	found := false
	for _, c := range server.Commands() {
		if c.Use == "sub" {
			found = true
		}
	}
	assert.True(t, found, "sub must mount under server")
}

func TestMountAll_NestedUnder(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	hooks := newCmd("hooks")
	claude := HostNamespaceCmd("claude", hooks)
	root.AddCommand(claude)

	err := MountAll(root, Mount{Under: []string{"claude", "hooks"}, Cmd: newCmd("stop")})

	require.NoError(t, err)

	found := false
	for _, c := range hooks.Commands() {
		if c.Use == "stop" {
			found = true
		}
	}
	assert.True(t, found, "stop must mount under claude hooks")
}

func TestMountAll_UnknownSegmentErrors(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	err := MountAll(root, Mount{Under: []string{"nope"}, Cmd: newCmd("one")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
}

func TestMountAll_UnknownSecondSegmentErrors(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(newCmd("claude"))

	err := MountAll(root, Mount{Under: []string{"claude", "nope"}, Cmd: newCmd("one")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Contains(t, err.Error(), `"claude"`)
}

func TestMountAll_NilCmdIsSkippedSafely(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	assert.NotPanics(t, func() {
		err := MountAll(root, Mount{Cmd: nil})
		assert.NoError(t, err)
	})
	assert.Empty(t, root.Commands())
}

func TestMountAll_ProcessesInArgumentOrder(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	err := MountAll(root, Mount{Cmd: newCmd("one")}, Mount{Cmd: newCmd("two")})

	require.NoError(t, err)
	require.Len(t, root.Commands(), 2)
	assert.Equal(t, "one", root.Commands()[0].Use)
	assert.Equal(t, "two", root.Commands()[1].Use)
}

func TestMountProviders_MountsAllProvidersCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	p1 := fakeProvider{mounts: []Mount{{Cmd: newCmd("one")}}}
	p2 := fakeProvider{mounts: []Mount{{Cmd: newCmd("two")}, {Cmd: newCmd("three")}}}

	err := MountProviders(root, p1, p2)
	require.NoError(t, err)

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

	p := fakeProvider{mounts: []Mount{{Cmd: newCmd("one")}}}

	var err error
	assert.NotPanics(t, func() {
		err = MountProviders(root, nil, p)
	})
	require.NoError(t, err)

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

	var err error
	assert.NotPanics(t, func() {
		err = MountProviders(root, empty)
	})
	require.NoError(t, err)
	assert.Empty(t, root.Commands())
}

// TestMountProviders_PropagatesMountAllError proves a provider whose Mount
// declares an unresolved Under path fails MountProviders loudly, instead of
// silently dropping the mount.
func TestMountProviders_PropagatesMountAllError(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	p := fakeProvider{mounts: []Mount{{Under: []string{"nope"}, Cmd: newCmd("one")}}}

	err := MountProviders(root, p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
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
	err := MountProviders(root, fakeProvider{mounts: []Mount{{Cmd: ns}}})
	require.NoError(t, err)

	root.SetArgs([]string{"ns", "leaf"})
	err = root.Execute()

	require.NoError(t, err)
	assert.True(t, ran, "the nested leaf's RunE must execute via root.Execute()")
}
