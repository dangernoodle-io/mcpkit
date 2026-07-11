package xdgpath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unsetXDGEnv clears every env var xdgpath consults, so each test starts
// from a known-unset baseline regardless of the ambient shell. t.Setenv("")
// suffices: resolve treats an empty value the same as unset.
func unsetXDGEnv(t *testing.T, app string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv(envName(app)+"_CONFIG_DIR", "")
	t.Setenv(envName(app)+"_CACHE_DIR", "")
	t.Setenv(envName(app)+"_DATA_DIR", "")
}

func TestConfigDir_AllEnvUnset_ResolvesToDotConfig(t *testing.T) {
	unsetXDGEnv(t, "widget")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got := ConfigDir("widget")

	// Critical requirement: macOS must resolve to ~/.config, not
	// ~/Library/Application Support.
	assert.Equal(t, filepath.Join(home, ".config", "widget"), got)
	assert.NotContains(t, got, "Library")
}

func TestCacheDir_AllEnvUnset_ResolvesToDotCache(t *testing.T) {
	unsetXDGEnv(t, "widget")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got := CacheDir("widget")

	assert.Equal(t, filepath.Join(home, ".cache", "widget"), got)
	assert.NotContains(t, got, "Library")
}

func TestDataDir_AllEnvUnset_ResolvesToLocalShare(t *testing.T) {
	unsetXDGEnv(t, "widget")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got := DataDir("widget")

	// Critical requirement: macOS must resolve to ~/.local/share, not
	// ~/Library/Application Support.
	assert.Equal(t, filepath.Join(home, ".local", "share", "widget"), got)
	assert.NotContains(t, got, "Library")
}

func TestConfigDir_XDGConfigHome_Overrides(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

	assert.Equal(t, filepath.Join("/tmp/xdg-config", "widget"), ConfigDir("widget"))
}

func TestCacheDir_XDGCacheHome_Overrides(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")

	assert.Equal(t, filepath.Join("/tmp/xdg-cache", "widget"), CacheDir("widget"))
}

func TestDataDir_XDGDataHome_Overrides(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	assert.Equal(t, filepath.Join("/tmp/xdg-data", "widget"), DataDir("widget"))
}

func TestConfigDir_AppConfigDir_OverridesXDGConfigHome(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	t.Setenv("WIDGET_CONFIG_DIR", "/tmp/widget-config")

	// App-specific override is used verbatim, not joined with app again.
	assert.Equal(t, "/tmp/widget-config", ConfigDir("widget"))
}

func TestCacheDir_AppCacheDir_OverridesXDGCacheHome(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	t.Setenv("WIDGET_CACHE_DIR", "/tmp/widget-cache")

	assert.Equal(t, "/tmp/widget-cache", CacheDir("widget"))
}

func TestDataDir_AppDataDir_OverridesXDGDataHome(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	t.Setenv("WIDGET_DATA_DIR", "/tmp/widget-data")

	// App-specific override is used verbatim, not joined with app again.
	assert.Equal(t, "/tmp/widget-data", DataDir("widget"))
}

func TestConfigDir_NonAbsoluteAppConfigDir_Ignored(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_CONFIG_DIR", "relative/path")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".config", "widget"), ConfigDir("widget"))
}

func TestConfigDir_NonAbsoluteXDGConfigHome_Ignored(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_CONFIG_HOME", "relative/path")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".config", "widget"), ConfigDir("widget"))
}

func TestCacheDir_NonAbsoluteAppCacheDir_Ignored(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_CACHE_DIR", "relative/path")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".cache", "widget"), CacheDir("widget"))
}

func TestDataDir_NonAbsoluteAppDataDir_Ignored(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_DATA_DIR", "relative/path")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".local", "share", "widget"), DataDir("widget"))
}

func TestDataDir_NonAbsoluteXDGDataHome_Ignored(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("XDG_DATA_HOME", "relative/path")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".local", "share", "widget"), DataDir("widget"))
}

func TestConfigFile_JoinsConfigDirAndName(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_CONFIG_DIR", "/tmp/widget-config")

	assert.Equal(t, filepath.Join("/tmp/widget-config", "settings.json"), ConfigFile("widget", "settings.json"))
}

func TestCacheFile_JoinsCacheDirAndName(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_CACHE_DIR", "/tmp/widget-cache")

	assert.Equal(t, filepath.Join("/tmp/widget-cache", "blob.bin"), CacheFile("widget", "blob.bin"))
}

func TestDataFile_JoinsDataDirAndName(t *testing.T) {
	unsetXDGEnv(t, "widget")
	t.Setenv("WIDGET_DATA_DIR", "/tmp/widget-data")

	assert.Equal(t, filepath.Join("/tmp/widget-data", "store.db"), DataFile("widget", "store.db"))
}

func TestEnvName_HyphenatedAppName(t *testing.T) {
	assert.Equal(t, "MY_COOL_APP", envName("my-cool-app"))
	assert.Equal(t, "POGOPIN", envName("pogopin"))
	assert.Equal(t, "APP_2", envName("app.2"))
}
