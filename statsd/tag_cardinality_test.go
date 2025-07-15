package statsd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	CardinalityInvalid = 5
)

func TestValidateCardinality(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Cardinality
	}{
		{
			name:     "valid none",
			input:    "none",
			expected: CardinalityNone,
		},
		{
			name:     "valid low",
			input:    "low",
			expected: CardinalityLow,
		},
		{
			name:     "valid orchestrator",
			input:    "orchestrator",
			expected: CardinalityOrchestrator,
		},
		{
			name:     "valid high",
			input:    "high",
			expected: CardinalityHigh,
		},
		{
			name:     "case insensitive none",
			input:    "NONE",
			expected: CardinalityNone,
		},
		{
			name:     "case insensitive low",
			input:    "LOW",
			expected: CardinalityLow,
		},
		{
			name:     "case insensitive orchestrator",
			input:    "ORCHESTRATOR",
			expected: CardinalityOrchestrator,
		},
		{
			name:     "case insensitive high",
			input:    "HIGH",
			expected: CardinalityHigh,
		},
		{
			name:     "mixed case",
			input:    "OrChEsTrAtOr",
			expected: CardinalityOrchestrator,
		},
		{
			name:     "empty string",
			input:    "",
			expected: CardinalityNotSet,
		},
		{
			name:     "invalid value",
			input:    "invalid",
			expected: CardinalityNotSet,
		},
		{
			name:     "partial match",
			input:    "orchestr",
			expected: CardinalityNotSet,
		},
		{
			name:     "whitespace",
			input:    " none ",
			expected: CardinalityNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateCardinality(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInitTagCardinality(t *testing.T) {
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
		inputCard          Cardinality
		ddCardinality      string
		datadogCardinality string
		expected           Cardinality
	}{
		{
			name:               "input parameter takes precedence",
			inputCard:          CardinalityHigh,
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           CardinalityHigh,
		},
		{
			name:               "DD_CARDINALITY used when input empty",
			inputCard:          CardinalityNotSet,
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           CardinalityLow,
		},
		{
			name:               "DATADOG_CARDINALITY used when DD_CARDINALITY empty",
			inputCard:          CardinalityNotSet,
			ddCardinality:      "",
			datadogCardinality: "orchestrator",
			expected:           CardinalityOrchestrator,
		},
		{
			name:               "empty when no environment variables set",
			inputCard:          CardinalityNotSet,
			ddCardinality:      "",
			datadogCardinality: "",
			expected:           CardinalityNotSet,
		},
		{
			name:               "invalid input parameter",
			inputCard:          5,
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           CardinalityLow,
		},
		{
			name:               "invalid DD_CARDINALITY",
			inputCard:          CardinalityNotSet,
			ddCardinality:      "invalid",
			datadogCardinality: "orchestrator",
			expected:           CardinalityOrchestrator,
		},
		{
			name:               "invalid DATADOG_CARDINALITY",
			inputCard:          CardinalityNotSet,
			ddCardinality:      "",
			datadogCardinality: "invalid",
			expected:           CardinalityNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchTagCardinality(tt.inputCard, tt.ddCardinality, tt.datadogCardinality)

			// Verify the result
			result := getTagCardinality()
			assert.Equal(t, tt.expected, result)

			resetTagCardinality()
		})
	}
}

func TestGetTagCardinality(t *testing.T) {
	// Test that getTagCardinality returns the current value
	initTagCardinality(CardinalityHigh)
	result := getTagCardinality()
	assert.Equal(t, CardinalityHigh, result)

	// Test that it returns empty string when not set
	initTagCardinality(CardinalityNotSet)
	result = getTagCardinality()
	assert.Equal(t, CardinalityNotSet, result)
}

func TestConcurrentAccess(t *testing.T) {
	// Test that concurrent access to tag cardinality is safe
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			initTagCardinality(CardinalityHigh)
			_ = getTagCardinality()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	result := getTagCardinality()
	assert.Equal(t, CardinalityHigh, result)
}

func TestResolveCardinality(t *testing.T) {
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
		name          string
		input         Cardinality
		globalSetting Cardinality
		expected      Cardinality
	}{
		{
			name:          "valid cardinality returns same value",
			input:         CardinalityLow,
			globalSetting: CardinalityHigh,
			expected:      CardinalityLow,
		},
		{
			name:          "empty cardinality uses global setting",
			input:         CardinalityNotSet,
			globalSetting: CardinalityHigh,
			expected:      CardinalityHigh,
		},
		{
			name:          "empty cardinality with empty global",
			input:         CardinalityNotSet,
			globalSetting: CardinalityNotSet,
			expected:      CardinalityNotSet,
		},
		{
			name:          "invalid cardinality falls back to global",
			input:         CardinalityNotSet,
			globalSetting: CardinalityLow,
			expected:      CardinalityLow,
		},
		{
			name:          "invalid cardinality with empty global",
			input:         CardinalityInvalid,
			globalSetting: CardinalityNotSet,
			expected:      CardinalityNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the global cardinality setting
			patchTagCardinality(tt.globalSetting, "", "")

			// Call resolveCardinality
			result := resolveCardinality(tt.input)

			// Verify the result
			assert.Equal(t, tt.expected, result)

			// Clean up
			resetTagCardinality()
		})
	}
}

func TestResolveCardinalityWithEnvironmentVariables(t *testing.T) {
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
		input              Cardinality
		ddCardinality      string
		datadogCardinality string
		expected           Cardinality
	}{
		{
			name:               "empty cardinality uses DD_CARDINALITY",
			input:              CardinalityNotSet,
			ddCardinality:      "high",
			datadogCardinality: "low",
			expected:           CardinalityHigh,
		},
		{
			name:               "empty cardinality uses DATADOG_CARDINALITY when DD_CARDINALITY empty",
			input:              CardinalityNotSet,
			ddCardinality:      "",
			datadogCardinality: "orchestrator",
			expected:           CardinalityOrchestrator,
		},
		{
			name:               "empty cardinality with no environment variables",
			input:              CardinalityNotSet,
			ddCardinality:      "",
			datadogCardinality: "",
			expected:           CardinalityNotSet,
		},
		{
			name:               "invalid cardinality falls back to DD_CARDINALITY",
			input:              CardinalityInvalid,
			ddCardinality:      "low",
			datadogCardinality: "high",
			expected:           CardinalityLow,
		},
		{
			name:               "invalid cardinality falls back to DATADOG_CARDINALITY when DD_CARDINALITY invalid",
			input:              CardinalityInvalid,
			ddCardinality:      "invalid",
			datadogCardinality: "high",
			expected:           CardinalityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the environment variables
			patchTagCardinality(CardinalityNotSet, tt.ddCardinality, tt.datadogCardinality)

			// Call resolveCardinality
			result := resolveCardinality(tt.input)

			// Verify the result
			assert.Equal(t, tt.expected, result)

			// Clean up
			resetTagCardinality()
		})
	}
}
func patchTagCardinality(userInput Cardinality, DDInput string, DATADOGInput string) {
	if DDInput != "" {
		os.Setenv("DD_CARDINALITY", DDInput)
	}
	if DATADOGInput != "" {
		os.Setenv("DATADOG_CARDINALITY", DATADOGInput)
	}
	initTagCardinality(userInput)
}

func resetTagCardinality() {
	os.Unsetenv("DD_CARDINALITY")
	os.Unsetenv("DATADOG_CARDINALITY")
	tagCardinality = defaultTagCardinality
}
