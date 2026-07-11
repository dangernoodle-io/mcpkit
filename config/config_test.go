package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Name    string
	Port    int
	Enabled bool
	Ratio   float64
}

type nestedTestConfig struct {
	Name  string
	Inner struct {
		Value string
	}
}

func TestLoad_NoOpts_DefaultsUnchangedNoIO(t *testing.T) {
	def := testConfig{Name: "widget", Port: 8080}

	got, err := Load(def)

	require.NoError(t, err)
	assert.Equal(t, def, got)
}

func TestLoad_DefaultsOnly_Verbatim(t *testing.T) {
	def := testConfig{Name: "widget", Port: 8080, Enabled: true, Ratio: 1.5}

	got, err := Load(def)

	require.NoError(t, err)
	assert.Equal(t, def, got)
}

func TestLoad_DefaultsAndMissingFile_DefaultsUnchangedNoError(t *testing.T) {
	def := testConfig{Name: "widget", Port: 8080}
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	got, err := Load(def, WithFile(missing))

	require.NoError(t, err)
	assert.Equal(t, def, got)
}

func TestLoad_DefaultsAndPartialFile_PresentFieldsOverridePresent(t *testing.T) {
	def := testConfig{Name: "widget", Port: 8080, Enabled: true, Ratio: 1.5}

	path := writeJSONFile(t, `{"Port": 9090}`)

	got, err := Load(def, WithFile(path))

	require.NoError(t, err)
	assert.Equal(t, "widget", got.Name) // absent key keeps default
	assert.Equal(t, 9090, got.Port)     // present key overrides
	assert.True(t, got.Enabled)
	assert.InEpsilon(t, 1.5, got.Ratio, 0.0001)
}

func TestLoad_MalformedFile_ErrorWrapsPath(t *testing.T) {
	path := writeJSONFile(t, `{not valid json`)

	_, err := Load(testConfig{}, WithFile(path))

	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

func TestLoad_FileReadError_NotMissing_ReturnsWrappedError(t *testing.T) {
	// A directory path makes os.ReadFile fail with a non-IsNotExist error
	// (exercises the loadFile branch distinct from "missing file").
	dir := t.TempDir()

	_, err := Load(testConfig{}, WithFile(dir))

	require.Error(t, err)
	assert.Contains(t, err.Error(), dir)
}

func TestLoad_DefaultsAndEnvNoMatch_DefaultsUnchanged(t *testing.T) {
	def := testConfig{Name: "widget", Port: 8080}

	got, err := Load(def, WithEnv("MC3TEST_NOMATCH_"))

	require.NoError(t, err)
	assert.Equal(t, def, got)
}

func TestLoad_FullStackSameField_EnvOverridesFileOverridesDefaults(t *testing.T) {
	path := writeJSONFile(t, `{"Port": 9090, "Name": "from-file"}`)

	t.Setenv("MC3TEST_STACK_PORT", "7070")

	got, err := Load(
		testConfig{Name: "from-default", Port: 8080},
		WithFile(path),
		WithEnv("MC3TEST_STACK_"),
	)

	require.NoError(t, err)
	assert.Equal(t, "from-file", got.Name) // file beats defaults, no env override
	assert.Equal(t, 7070, got.Port)        // env beats file beats defaults
}

func TestLoad_OptsOutOfOrder_PrecedenceUnchanged(t *testing.T) {
	path := writeJSONFile(t, `{"Port": 9090, "Name": "from-file"}`)

	t.Setenv("MC3TEST_ORDER_PORT", "7070")

	// Options passed env, file — reverse of natural precedence.
	got, err := Load(
		testConfig{Name: "from-default", Port: 8080},
		WithEnv("MC3TEST_ORDER_"),
		WithFile(path),
	)

	require.NoError(t, err)
	assert.Equal(t, "from-file", got.Name)
	assert.Equal(t, 7070, got.Port)
}

func TestLoad_RepeatedFileOption_LastOneWins(t *testing.T) {
	first := writeJSONFile(t, `{"Name": "first"}`)
	second := writeJSONFile(t, `{"Name": "second"}`)

	got, err := Load(testConfig{}, WithFile(first), WithFile(second))

	require.NoError(t, err)
	assert.Equal(t, "second", got.Name)
}

func TestLoad_RepeatedEnvOption_LastOneWins(t *testing.T) {
	t.Setenv("MC3TEST_FIRST_NAME", "from-first-prefix")
	t.Setenv("MC3TEST_SECOND_NAME", "from-second-prefix")

	got, err := Load(testConfig{}, WithEnv("MC3TEST_FIRST_"), WithEnv("MC3TEST_SECOND_"))

	require.NoError(t, err)
	assert.Equal(t, "from-second-prefix", got.Name)
}

func TestLoad_WithXDGFile_ResolvesDeterministically(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MC3XDGAPP_CONFIG_DIR", tmp)

	path := filepath.Join(tmp, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"Name": "xdg-widget"}`), 0o600))

	got, err := Load(testConfig{}, WithXDGFile("mc3xdgapp", "settings.json"))

	require.NoError(t, err)
	assert.Equal(t, "xdg-widget", got.Name)
}

func TestLoad_WithXDGFile_MissingBehavesLikeWithFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MC3XDGAPP2_CONFIG_DIR", tmp)

	got, err := Load(testConfig{Name: "default"}, WithXDGFile("mc3xdgapp2", "missing.json"))

	require.NoError(t, err)
	assert.Equal(t, "default", got.Name)
}

func TestLoad_WithXDGFile_MalformedBehavesLikeWithFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MC3XDGAPP3_CONFIG_DIR", tmp)

	path := filepath.Join(tmp, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{not valid`), 0o600))

	_, err := Load(testConfig{}, WithXDGFile("mc3xdgapp3", "settings.json"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

func TestLoad_NestedStructField_FileAndDefaultsHandleIt_EnvSkipsIt(t *testing.T) {
	def := nestedTestConfig{Name: "default"}
	def.Inner.Value = "default-inner"

	path := writeJSONFile(t, `{"Inner": {"Value": "from-file"}}`)

	t.Setenv("MC3TEST_NESTED_NAME", "from-env")

	got, err := Load(
		def,
		WithFile(path),
		WithEnv("MC3TEST_NESTED_"),
	)

	require.NoError(t, err)
	assert.Equal(t, "from-env", got.Name)         // scalar field: env applies
	assert.Equal(t, "from-file", got.Inner.Value) // struct field: env skips, file applies
}

func TestExpandHome_UserHomeDirError_ReturnsPathUnchanged(t *testing.T) {
	t.Setenv("HOME", "")

	got := ExpandHome("~/foo")

	// Without a resolvable home, ExpandHome must not panic or fabricate a
	// path; it falls back to returning the input unchanged.
	if _, err := os.UserHomeDir(); err != nil {
		assert.Equal(t, "~/foo", got)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tilde-slash expands", "~/foo", filepath.Join(home, "foo")},
		{"absolute unchanged", "/etc/foo", "/etc/foo"},
		{"empty unchanged", "", ""},
		{"other-user tilde unchanged", "~other/foo", "~other/foo"},
		{"plain relative unchanged", "foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExpandHome(tt.in))
		})
	}
}

// writeJSONFile writes body to a temp file and returns its path.
func writeJSONFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}
