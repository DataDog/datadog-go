package statsd

import (
	"os"
	"sync"
	"unicode"
)

// ddExternalEnvVarName specifies the env var to inject the environment name.
const ddExternalEnvVarName = "DD_EXTERNAL_ENV"

var (
	externalEnv      = ""
	externalEnvMutex sync.RWMutex
)

// initExternalEnv initializes the external environment name.
func initExternalEnv() {
	var value = os.Getenv(ddExternalEnvVarName)
	if value != "" {
		externalEnvMutex.Lock()
		externalEnv = sanitizeExternalEnv(value)
		externalEnvMutex.Unlock()
	}
}

// sanitizeExternalEnv removes non-printable characters and pipe characters from the external environment name.
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
	externalEnvMutex.RLock()
	defer externalEnvMutex.RUnlock()
	return externalEnv
}

func noExternalEnv() {
	externalEnvMutex.Lock()
	defer externalEnvMutex.Unlock()
	externalEnv = ""
}
