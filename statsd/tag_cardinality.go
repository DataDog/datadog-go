package statsd

type ExtraOption interface{}
type CardinalityOption struct {
	card string
}

// WithCardinality sets the tag cardinality of the metric.
func WithCardinality(card string) CardinalityOption {
	return CardinalityOption{card: card}
}
