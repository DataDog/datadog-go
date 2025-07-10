package statsd

import (
	"os"
	"strings"
	"sync"
)

type Parameter interface{}

// Possible values for the cardinality are "none", "low", "orchestrator", and "high".
type CardinalityParameter struct {
	card string
}

var (
	// Global setting of the tag cardinality.
	tagCardinality      = CardinalityParameter{card: ""}
	tagCardinalityMutex sync.RWMutex
)

// initTagCardinality initializes the tag cardinality.
func initTagCardinality(card string) {
	tagCardinalityMutex.Lock()

	// Read in and validate the user-provided value.
	tagCardinality = validateCardinality(card)

	// If the user has not provided a valid value, read the value from the DD_CARDINALITY environment variable.
	if tagCardinality.card == "" {
		tagCardinality = validateCardinality(os.Getenv("DD_CARDINALITY"))
	}
	// If DD_CARDINALITY is not set or valid, read the value from the DATADOG_CARDINALITY environment variable.
	if tagCardinality.card == "" {
		tagCardinality = validateCardinality(os.Getenv("DATADOG_CARDINALITY"))
	}
	tagCardinalityMutex.Unlock()
}

// validateCardinality checks if the tag cardinality is one of the valid values.
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
	tagCardinalityMutex.RLock()
	defer tagCardinalityMutex.RUnlock()
	return tagCardinality.card
}
