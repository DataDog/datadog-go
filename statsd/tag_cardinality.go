package statsd

import (
	"os"
	"strings"
)

type Parameter interface{}

// Possible values for the cardinality are "none", "low", "orchestrator", and "high".
type CardinalityParameter struct {
	card string
}

var (
	// Global setting of the tag cardinality.
	tagCardinality = CardinalityParameter{card: ""}
)

// initTagCardinality initializes the tag cardinality.
func initTagCardinality(card string) {
	// If the user has not provided a value, read the value from the DD_TAG_CARDINITY environment variable.
	if card == "" {
		card = os.Getenv("DD_CARDINALITY")
	}
	if card == "" {
		card = os.Getenv("DATADOG_CARDINALITY")
	}
	tagCardinality = validateCardinality(card)
}

// validCardinality checks if the tag cardinality is a valid value.
func validateCardinality(tagCardinality string) CardinalityParameter {
	tagCardinality = strings.ToLower(tagCardinality)
	validValues := []string{"none", "low", "orchestrator", "high"}
	for _, valid := range validValues {
		if tagCardinality == valid {
			return CardinalityParameter{card: tagCardinality}
		}
	}

	return CardinalityParameter{card: ""}
}

func getTagCardinality() string {
	return tagCardinality.card
}
