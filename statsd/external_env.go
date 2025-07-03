package statsd

import (
	"os"
	"unicode"
)

// ddExternalEnvVarName specifies the env var to inject the environment name.
const ddExternalEnvVarName = "DD_EXTERNAL_ENV"

var (
	externalEnv = ""
)

// initExternalEnv initializes the external environment name.
func initExternalEnv() {
	if value := os.Getenv(ddExternalEnvVarName); value != "" {
		externalEnv = sanitizeExternalEnv(value)
	}
}

func sanitizeExternalEnv(externalEnv string) string {
	if externalEnv == "" {
		return ""
	}
	var output string
	for _, r := range externalEnv {
		if unicode.IsPrint(r) && r != '|' {
			output += string(r)
		}
	}

	return output
}

func getExternalEnv() string {
	return externalEnv
}
