package statsd

import (
	"os"
	"strings"
)

type ExtraOption interface{}

// Possible values for the cardinality are "none", "low", "orchestrator", and "high".
type CardinalityOption struct {
	card string
}

// ddExternalEnvVarName specifies the env var to inject the environment name.
const ddTagCardinalityVarName = "DD_TAG_CARDINALITY"

var (
	// Global setting of the tag cardinality.
	tagCardinality = CardinalityOption{card: ""}
)

// initTagCardinality initializes the tag cardinality.
func initTagCardinality(card string) {
	// If the user has not provided a value, read the value from the DD_TAG_CARDINITY environment variable.
	if card == "" {
		card = os.Getenv(ddTagCardinalityVarName)
	}
	tagCardinality = validateCardinality(card)
}

// validCardinality checks if the tag cardinality is a valid value.
func validateCardinality(tagCardinality string) CardinalityOption {
	tagCardinality = strings.ToLower(tagCardinality)
	validValues := []string{"none", "low", "orchestrator", "high"}
	for _, valid := range validValues {
		if tagCardinality == valid {
			return CardinalityOption{card: tagCardinality}
		}
	}

	return CardinalityOption{card: ""}
}

func getTagCardinality() string {
	return tagCardinality.card
}
