package statsd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCardinality(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected CardinalityParameter
	}{
		{
			name:     "valid none",
			input:    "none",
			expected: CardinalityParameter{card: "none"},
		},
		{
			name:     "valid low",
			input:    "low",
			expected: CardinalityParameter{card: "low"},
		},
		{
			name:     "valid orchestrator",
			input:    "orchestrator",
			expected: CardinalityParameter{card: "orchestrator"},
		},
		{
			name:     "valid high",
			input:    "high",
			expected: CardinalityParameter{card: "high"},
		},
		{
			name:     "case insensitive none",
			input:    "NONE",
			expected: CardinalityParameter{card: "none"},
		},
		{
			name:     "case insensitive low",
			input:    "LOW",
			expected: CardinalityParameter{card: "low"},
		},
		{
			name:     "case insensitive orchestrator",
			input:    "ORCHESTRATOR",
			expected: CardinalityParameter{card: "orchestrator"},
		},
		{
			name:     "case insensitive high",
			input:    "HIGH",
			expected: CardinalityParameter{card: "high"},
		},
		{
			name:     "mixed case",
			input:    "OrChEsTrAtOr",
			expected: CardinalityParameter{card: "orchestrator"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: CardinalityParameter{card: ""},
		},
		{
			name:     "invalid value",
			input:    "invalid",
			expected: CardinalityParameter{card: ""},
		},
		{
			name:     "partial match",
			input:    "orchestr",
			expected: CardinalityParameter{card: ""},
		},
		{
			name:     "whitespace",
			input:    " none ",
			expected: CardinalityParameter{card: ""},
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
		inputCard          string
		ddCardinality      string
		datadogCardinality string
		expected           string
	}{
		{
			name:               "input parameter takes precedence",
			inputCard:          "high",
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           "high",
		},
		{
			name:               "DD_CARDINALITY used when input empty",
			inputCard:          "",
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           "low",
		},
		{
			name:               "DATADOG_CARDINALITY used when DD_CARDINALITY empty",
			inputCard:          "",
			ddCardinality:      "",
			datadogCardinality: "orchestrator",
			expected:           "orchestrator",
		},
		{
			name:               "empty when no environment variables set",
			inputCard:          "",
			ddCardinality:      "",
			datadogCardinality: "",
			expected:           "",
		},
		{
			name:               "invalid input parameter",
			inputCard:          "invalid",
			ddCardinality:      "low",
			datadogCardinality: "orchestrator",
			expected:           "low",
		},
		{
			name:               "invalid DD_CARDINALITY",
			inputCard:          "",
			ddCardinality:      "invalid",
			datadogCardinality: "orchestrator",
			expected:           "orchestrator",
		},
		{
			name:               "invalid DATADOG_CARDINALITY",
			inputCard:          "",
			ddCardinality:      "",
			datadogCardinality: "invalid",
			expected:           "",
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
	initTagCardinality("high")
	result := getTagCardinality()
	assert.Equal(t, "high", result)

	// Test that it returns empty string when not set
	initTagCardinality("")
	result = getTagCardinality()
	assert.Equal(t, "", result)
}

func TestConcurrentAccess(t *testing.T) {
	// Test that concurrent access to tag cardinality is safe
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			initTagCardinality("high")
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
	assert.Equal(t, "high", result)
}

func patchTagCardinality(userInput string, DDInput string, DATADOGInput string) {
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
