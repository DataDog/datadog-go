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
	tagCardinality = CardinalityOption{card: ""}
)

// initTagCardinality initializes the tag cardinality.
func initTagCardinality() {
	var value = os.Getenv(ddTagCardinalityVarName)
	tagCardinality = validateCardinality(value)
}

// validCardinality checks if the tag cardinality is valid.
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

// WithCardinality sets the tag cardinality of the metric.
func WithCardinality(card string) CardinalityOption {
	return CardinalityOption{card: card}
}
