package statsd

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

// TODO: Optimize this by hoisting the top level resource attributes.
func attrsToTags(resAttrs attribute.Set, optAttrs []attribute.KeyValue) []string {
	attrs := make([]string, 0, resAttrs.Len()+len(optAttrs))
	for _, keyValue := range resAttrs.ToSlice() {
		attrs = append(attrs, keyValueAttrToTag(keyValue))
	}
	for _, keyValue := range optAttrs {
		attrs = append(attrs, keyValueAttrToTag(keyValue))
	}
	return attrs
}

func keyValueAttrToTag(keyValue attribute.KeyValue) string {
	return fmt.Sprintf("%s:%v", keyValue.Key, keyValue.Value.AsInterface())
}
