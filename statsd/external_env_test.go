package statsd

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeExternalEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "normal string",
			input:    "production",
			expected: "production",
		},
		{
			name:     "string with spaces",
			input:    "staging environment",
			expected: "staging environment",
		},
		{
			name:     "string with pipe character",
			input:    "prod|staging",
			expected: "prodstaging",
		},
		{
			name:     "string with multiple pipe characters",
			input:    "prod|staging|dev",
			expected: "prodstagingdev",
		},
		{
			name:     "string with non-printable characters",
			input:    "prod\x00staging\x01dev",
			expected: "prodstagingdev",
		},
		{
			name:     "string with control characters",
			input:    "prod\nstaging\tdev\r",
			expected: "prodstagingdev",
		},
		{
			name:     "string with unicode characters",
			input:    "prod-环境-staging",
			expected: "prod-环境-staging",
		},
		{
			name:     "string with special characters except pipe",
			input:    "prod@#$%^&*()_+-=[]{};':\",./<>?",
			expected: "prod@#$%^&*()_+-=[]{};':\",./<>?",
		},
		{
			name:     "string with only pipe characters",
			input:    "|||",
			expected: "",
		},
		{
			name:     "string with only non-printable characters",
			input:    "\x00\x01\x02\x03",
			expected: "",
		},
		{
			name:     "string with mixed valid and invalid characters",
			input:    "prod\x00|staging\x01|dev",
			expected: "prodstagingdev",
		},
		{
			name:     "string with leading and trailing pipes",
			input:    "|prod|staging|",
			expected: "prodstaging",
		},
		{
			name:     "string with leading and trailing non-printable characters",
			input:    "\x00prod\x01staging\x02",
			expected: "prodstaging",
		},
		{
			name:     "string with emoji and special characters",
			input:    "prod🚀|staging🔥|dev✨",
			expected: "prod🚀staging🔥dev✨",
		},
		{
			name:     "string with numbers and symbols",
			input:    "prod-123|staging-456|dev-789",
			expected: "prod-123staging-456dev-789",
		},
		{
			name:     "string with underscores and hyphens",
			input:    "prod_env|staging-env|dev_env",
			expected: "prod_envstaging-envdev_env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeExternalEnv(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetExternalEnv(t *testing.T) {
	// Save original environment variable value
	originalValue := os.Getenv(ddExternalEnvVarName)
	defer os.Setenv(ddExternalEnvVarName, originalValue)

	tests := []struct {
		name           string
		envValue       string
		expectedResult string
	}{
		{
			name:           "no environment variable set",
			envValue:       "",
			expectedResult: "",
		},
		{
			name:           "normal environment value",
			envValue:       "production",
			expectedResult: "production",
		},
		{
			name:           "environment value with pipes",
			envValue:       "prod|staging",
			expectedResult: "prodstaging",
		},
		{
			name:           "environment value with special characters",
			envValue:       "prod-环境-staging",
			expectedResult: "prod-环境-staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the environment variable
			os.Setenv(ddExternalEnvVarName, tt.envValue)

			// Re-initialize the external environment
			initExternalEnv()

			// Test getExternalEnv
			result := getExternalEnv()
			assert.Equal(t, tt.expectedResult, result)

			defer resetExternalEnv()
		})
	}
}
func TestExternalEnvInitializationWithOriginDetection(t *testing.T) {
	// Save original environment variable values
	originalExternalEnv := os.Getenv(ddExternalEnvVarName)
	originalOriginDetection := os.Getenv(originDetectionEnabled)
	defer func() {
		os.Setenv(ddExternalEnvVarName, originalExternalEnv)
		os.Setenv(originDetectionEnabled, originalOriginDetection)
	}()

	tests := []struct {
		name                   string
		externalEnvValue       string
		originDetectionEnabled bool
		expectExternalEnvSet   bool
	}{
		{
			name:                   "external env should be set when origin detection enabled",
			externalEnvValue:       "production",
			originDetectionEnabled: true,
			expectExternalEnvSet:   true,
		},
		{
			name:                   "external env should be cleared when origin detection disabled",
			externalEnvValue:       "production",
			originDetectionEnabled: false,
			expectExternalEnvSet:   false,
		},
		{
			name:                   "no external env with origin detection enabled",
			externalEnvValue:       "",
			originDetectionEnabled: true,
			expectExternalEnvSet:   false,
		},
		{
			name:                   "no external env with origin detection disabled",
			externalEnvValue:       "",
			originDetectionEnabled: false,
			expectExternalEnvSet:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			os.Setenv(ddExternalEnvVarName, tt.externalEnvValue)
			os.Setenv(originDetectionEnabled, strconv.FormatBool(tt.originDetectionEnabled))

			// Clear external env first
			noExternalEnv()
			assert.Equal(t, "", getExternalEnv(), "External env should be cleared initially")

			// Simulate the logic from newWithWriter
			originDetection := isOriginDetectionEnabled(&Options{originDetection: tt.originDetectionEnabled})
			if !originDetection {
				noExternalEnv()
			} else {
				initExternalEnv()
			}

			// Check the result
			result := getExternalEnv()
			if tt.expectExternalEnvSet {
				assert.Equal(t, tt.externalEnvValue, result, "External env should be set when origin detection is enabled")
			} else {
				assert.Equal(t, "", result, "External env should be empty when origin detection is disabled")
			}

			// Test that the |e: field is not appended to metrics when origin detection is disabled
			buffer := []byte("test.metric:123|g")
			metricResult := appendExternalEnv(buffer)

			if tt.expectExternalEnvSet {
				assert.Contains(t, string(metricResult), "|e:"+tt.externalEnvValue, "Metric should contain |e: field when origin detection is enabled")
			} else {
				assert.NotContains(t, string(metricResult), "|e:", "Metric should not contain |e: field when origin detection is disabled")
			}

			// Clean up
			resetExternalEnv()
			os.Unsetenv(originDetectionEnabled)
		})
	}
}
