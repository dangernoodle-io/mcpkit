package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type envScalarConfig struct {
	Name    string
	Port    int
	Enabled bool
	Ratio   float64
	Retries uint
}

type envUnexportedFieldConfig struct {
	Name    string
	skipped string
}

// use reports skipped so the field isn't flagged as unused; it exists
// solely to exercise WithEnv's unexported-field skip path.
func (c envUnexportedFieldConfig) use() string { return c.skipped }

func TestWithEnv_StringField_Overridden(t *testing.T) {
	t.Setenv("MC3ENV_A_NAME", "from-env")

	got, err := Load(envScalarConfig{}, WithEnv("MC3ENV_A_"))

	require.NoError(t, err)
	assert.Equal(t, "from-env", got.Name)
}

func TestWithEnv_IntField_Overridden(t *testing.T) {
	t.Setenv("MC3ENV_B_PORT", "9999")

	got, err := Load(envScalarConfig{}, WithEnv("MC3ENV_B_"))

	require.NoError(t, err)
	assert.Equal(t, 9999, got.Port)
}

func TestWithEnv_BoolField_Overridden(t *testing.T) {
	t.Setenv("MC3ENV_C_ENABLED", "true")

	got, err := Load(envScalarConfig{}, WithEnv("MC3ENV_C_"))

	require.NoError(t, err)
	assert.True(t, got.Enabled)
}

func TestWithEnv_FloatField_Overridden(t *testing.T) {
	t.Setenv("MC3ENV_D_RATIO", "3.25")

	got, err := Load(envScalarConfig{}, WithEnv("MC3ENV_D_"))

	require.NoError(t, err)
	assert.InEpsilon(t, 3.25, got.Ratio, 0.0001)
}

func TestWithEnv_UintField_Overridden(t *testing.T) {
	t.Setenv("MC3ENV_G_RETRIES", "5")

	got, err := Load(envScalarConfig{}, WithEnv("MC3ENV_G_"))

	require.NoError(t, err)
	assert.Equal(t, uint(5), got.Retries)
}

func TestWithEnv_UintField_UnparseableValue_Errors(t *testing.T) {
	t.Setenv("MC3ENV_H_RETRIES", "-1")

	_, err := Load(envScalarConfig{}, WithEnv("MC3ENV_H_"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Retries")
}

func TestWithEnv_UnparseableValue_ErrorNamesFieldVarValue(t *testing.T) {
	t.Setenv("MC3ENV_E_PORT", "not-a-number")

	_, err := Load(envScalarConfig{}, WithEnv("MC3ENV_E_"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Port")
	assert.Contains(t, err.Error(), "MC3ENV_E_PORT")
	assert.Contains(t, err.Error(), "not-a-number")
}

func TestWithEnv_UnexportedField_SkippedNotSet(t *testing.T) {
	t.Setenv("MC3ENV_I_SKIPPED", "should-not-be-set")
	t.Setenv("MC3ENV_I_NAME", "from-env")

	got, err := Load(envUnexportedFieldConfig{}, WithEnv("MC3ENV_I_"))

	require.NoError(t, err)
	assert.Equal(t, "from-env", got.Name)
	assert.Empty(t, got.use())
}

func TestWithEnv_BoolField_UnparseableValue_Errors(t *testing.T) {
	t.Setenv("MC3ENV_K_ENABLED", "not-a-bool")

	_, err := Load(envScalarConfig{}, WithEnv("MC3ENV_K_"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Enabled")
}

func TestWithEnv_FloatField_UnparseableValue_Errors(t *testing.T) {
	t.Setenv("MC3ENV_J_RATIO", "not-a-float")

	_, err := Load(envScalarConfig{}, WithEnv("MC3ENV_J_"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Ratio")
}

func TestWithEnv_NonStructT_ClearErrorNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_, err := Load(0, WithEnv("MC3ENV_F_"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "struct")
	})
}
