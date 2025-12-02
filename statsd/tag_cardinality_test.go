package statsd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvTagCardinality(t *testing.T) {
	// Save original environment variables
	originalDDCardinality := os.Getenv("DD_CARDINALITY")
	originalDatadogCardinality := os.Getenv("DATADOG_CARDINALITY")
	defer func() {
		// Restore original environment variables
		if originalDDCardinality != "" {
			os.Setenv("DD_CARDINALITY", originalDDCardinality)
		} else {
			os.Unsetenv("DD_CARDINALITY")
		}
		if originalDatadogCardinality != "" {
			os.Setenv("DATADOG_CARDINALITY", originalDatadogCardinality)
		} else {
			os.Unsetenv("DATADOG_CARDINALITY")
		}
	}()

	tests := []struct {
		name               string
		ddCardinality      string
		datadogCardinality string
		expected           Cardinality
		expectedOk         bool
	}{
		{
			name:               "DD_CARDINALITY used when input empty",
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           CardinalityLow,
			expectedOk:         true,
		},
		{
			name:               "DATADOG_CARDINALITY used when DD_CARDINALITY empty",
			ddCardinality:      "",
			datadogCardinality: "orchestrator",
			expected:           CardinalityOrchestrator,
			expectedOk:         true,
		},
		{
			name:               "empty when no environment variables set",
			ddCardinality:      "",
			datadogCardinality: "",
			expected:           CardinalityNotSet,
			expectedOk:         false,
		},
		{
			name:               "invalid DD_CARDINALITY",
			ddCardinality:      "invalid",
			datadogCardinality: "orchestrator",
			expected:           CardinalityOrchestrator,
			expectedOk:         true,
		},
		{
			name:               "invalid DATADOG_CARDINALITY",
			ddCardinality:      "",
			datadogCardinality: "invalid",
			expected:           CardinalityNotSet,
			expectedOk:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchTagCardinality(tt.ddCardinality, tt.datadogCardinality)

			// Verify the result
			result, ok := envTagCardinality()
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedOk, ok)

			resetTagCardinality()
		})
	}
}

func patchTagCardinality(DDInput string, DATADOGInput string) {
	if DDInput != "" {
		os.Setenv("DD_CARDINALITY", DDInput)
	}
	if DATADOGInput != "" {
		os.Setenv("DATADOG_CARDINALITY", DATADOGInput)
	}
}

func resetTagCardinality() {
	os.Unsetenv("DD_CARDINALITY")
	os.Unsetenv("DATADOG_CARDINALITY")
}

func TestParameterCardinality(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		card := parameterCardinality(nil, CardinalityHigh)
		assert.Equal(t, CardinalityHigh, card)
	})
	t.Run("empty", func(t *testing.T) {
		card := parameterCardinality([]Parameter{}, CardinalityHigh)
		assert.Equal(t, CardinalityHigh, card)
	})
	t.Run("missing", func(t *testing.T) {
		card := parameterCardinality([]Parameter{"foo"}, CardinalityHigh)
		assert.Equal(t, CardinalityHigh, card)
	})
	t.Run("invalid", func(t *testing.T) {
		card := parameterCardinality([]Parameter{Cardinality(CardinalityHigh + 1)}, CardinalityHigh)
		assert.Equal(t, CardinalityHigh, card)
	})
	t.Run("present", func(t *testing.T) {
		card := parameterCardinality([]Parameter{"foo", CardinalityLow}, CardinalityHigh)
		assert.Equal(t, CardinalityLow, card)
	})
	t.Run("multiple", func(t *testing.T) {
		card := parameterCardinality([]Parameter{"foo", CardinalityLow, CardinalityOrchestrator}, CardinalityHigh)
		assert.Equal(t, CardinalityLow, card)
	})
}

func TestIsValid(t *testing.T) {
	assert.False(t, (CardinalityNotSet - 1).isValid(), "make sure to update isValid when adding new value")

	assert.True(t, CardinalityNone.isValid())
	assert.True(t, CardinalityLow.isValid())
	assert.True(t, CardinalityOrchestrator.isValid())
	assert.True(t, CardinalityHigh.isValid())

	assert.False(t, (CardinalityHigh + 1).isValid(), "make sure to update isValid when adding new value")
}
