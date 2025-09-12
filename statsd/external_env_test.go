package statsd

import (
	"os"
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
func TestAppendExternalEnvWithOriginDetection(t *testing.T) {
	originalExternalEnv := os.Getenv(ddExternalEnvVarName)
	originalOriginDetection := os.Getenv(originDetectionEnabled)
	defer func() {
		os.Setenv(ddExternalEnvVarName, originalExternalEnv)
		os.Setenv(originDetectionEnabled, originalOriginDetection)
	}()

	tests := []struct {
		name              string
		o                 *Options
		configEnvVarValue string
		expectExternalEnv bool
	}{
		{
			name:              "nominal case",
			o:                 &Options{originDetection: defaultOriginDetection},
			configEnvVarValue: "",
			expectExternalEnv: true,
		},
		{
			name:              "has user-provided container ID and origin detection enabled",
			o:                 &Options{containerID: "user-provided", originDetection: defaultOriginDetection},
			configEnvVarValue: "",
			expectExternalEnv: true,
		},
		{
			name:              "originDetection option disabled",
			o:                 &Options{originDetection: false},
			configEnvVarValue: "",
			expectExternalEnv: false,
		},
		{
			name:              "DD_ORIGIN_DETECTION_ENABLED=false",
			o:                 &Options{originDetection: defaultOriginDetection},
			configEnvVarValue: "false",
			expectExternalEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(originDetectionEnabled, tt.configEnvVarValue)
			patchExternalEnv("test-env")

			buffer := []byte("test.metric:123|g")

			originDetection := isOriginDetectionEnabled(tt.o)
			result := appendExternalEnv(buffer, originDetection)

			if tt.expectExternalEnv {
				assert.Contains(t, string(result), "|e:test-env", "External env should be appended when origin detection is enabled")
			} else {
				assert.NotContains(t, string(result), "|e:", "External env should not be appended when origin detection is disabled")
			}

			resetExternalEnv()
		})
	}
}
