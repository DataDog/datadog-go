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
	defer tagCardinalityMutex.Unlock()

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
}

// validateCardinality checks if the tag cardinality is one of the valid values.
func validateCardinality(card string) CardinalityParameter {
	card = strings.ToLower(card)
	validValues := []string{"none", "low", "orchestrator", "high"}
	for _, valid := range validValues {
		if card == valid {
			return CardinalityParameter{card: card}
		}
	}

	return CardinalityParameter{card: ""}
}

func getTagCardinality() string {
	tagCardinalityMutex.RLock()
	defer tagCardinalityMutex.RUnlock()
	return tagCardinality.card
}

func parseTagCardinality(parameters []Parameter) CardinalityParameter {
	var cardinality = CardinalityParameter{card: getTagCardinality()}
	for _, o := range parameters {
		c, ok := o.(CardinalityParameter)
		if ok {
			cardinality = resolveCardinality(c)
		}
	}
	return cardinality
}

// resolveCardinality returns the cardinality to use, prioritizing the metric-level cardinality over the global setting.
// This function validates the cardinality and falls back to the global setting if invalid.
func resolveCardinality(cardinality CardinalityParameter) CardinalityParameter {
	value := validateCardinality(cardinality.card)
	if value.card == "" {
		return CardinalityParameter{card: getTagCardinality()}
	}
	return value
}
