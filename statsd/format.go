package statsd

import (
	"strconv"
	"strings"
)

var (
	gaugeSymbol        = []byte("g")
	countSymbol        = []byte("c")
	histogramSymbol    = []byte("h")
	distributionSymbol = []byte("d")
	setSymbol          = []byte("s")
	timingSymbol       = []byte("ms")
)

func appendHeader(buffer []byte, namespace string, name string) []byte {
	if namespace != "" {
		buffer = append(buffer, namespace...)
	}
	buffer = append(buffer, name...)
	buffer = append(buffer, ':')
	return buffer
}

func appendRate(buffer []byte, rate float64) []byte {
	if rate < 1 {
		buffer = append(buffer, "|@"...)
		buffer = strconv.AppendFloat(buffer, rate, 'f', -1, 64)
	}
	return buffer
}

func appendWithoutNewlines(buffer []byte, s string) []byte {
	// fastpath for strings without newlines
	if strings.IndexByte(s, '\n') == -1 {
		return append(buffer, s...)
	}

	for _, b := range []byte(s) {
		if b != '\n' {
			buffer = append(buffer, b)
		}
	}
	return buffer
}

func appendTags(buffer []byte, globalTags []string, tags []string) []byte {
	if len(globalTags) == 0 && len(tags) == 0 {
		return buffer
	}
	buffer = append(buffer, "|#"...)
	firstTag := true

	for _, tag := range globalTags {
		if !firstTag {
			buffer = append(buffer, ',')
		}
		buffer = appendWithoutNewlines(buffer, tag)
		firstTag = false
	}
	for _, tag := range tags {
		if !firstTag {
			buffer = append(buffer, ',')
		}
		buffer = appendWithoutNewlines(buffer, tag)
		firstTag = false
	}
	return buffer
}

func appendFloatMetric(buffer []byte, typeSymbol []byte, namespace string, globalTags []string, name string, value float64, tags []string, rate float64) []byte {
	buffer = appendHeader(buffer, namespace, name)
	buffer = strconv.AppendFloat(buffer, value, 'f', 6, 64)
	buffer = append(buffer, '|')
	buffer = append(buffer, typeSymbol...)
	buffer = appendRate(buffer, rate)
	buffer = appendTags(buffer, globalTags, tags)
	return buffer
}

func appendIntegerMetric(buffer []byte, typeSymbol []byte, namespace string, globalTags []string, name string, value int64, tags []string, rate float64) []byte {
	buffer = appendHeader(buffer, namespace, name)
	buffer = strconv.AppendInt(buffer, value, 10)
	buffer = append(buffer, '|')
	buffer = append(buffer, typeSymbol...)
	buffer = appendRate(buffer, rate)
	buffer = appendTags(buffer, globalTags, tags)
	return buffer
}

func appendStringMetric(buffer []byte, typeSymbol []byte, namespace string, globalTags []string, name string, value string, tags []string, rate float64) []byte {
	buffer = appendHeader(buffer, namespace, name)
	buffer = append(buffer, value...)
	buffer = append(buffer, '|')
	buffer = append(buffer, typeSymbol...)
	buffer = appendRate(buffer, rate)
	buffer = appendTags(buffer, globalTags, tags)
	return buffer
}

func appendGauge(buffer []byte, namespace string, globalTags []string, name string, value float64, tags []string, rate float64) []byte {
	return appendFloatMetric(buffer, gaugeSymbol, namespace, globalTags, name, value, tags, rate)
}

func appendCount(buffer []byte, namespace string, globalTags []string, name string, value int64, tags []string, rate float64) []byte {
	return appendIntegerMetric(buffer, countSymbol, namespace, globalTags, name, value, tags, rate)
}

func appendHistogram(buffer []byte, namespace string, globalTags []string, name string, value float64, tags []string, rate float64) []byte {
	return appendFloatMetric(buffer, histogramSymbol, namespace, globalTags, name, value, tags, rate)
}

func appendDistribution(buffer []byte, namespace string, globalTags []string, name string, value float64, tags []string, rate float64) []byte {
	return appendFloatMetric(buffer, distributionSymbol, namespace, globalTags, name, value, tags, rate)
}

func appendDecrement(buffer []byte, namespace string, globalTags []string, name string, tags []string, rate float64) []byte {
	return appendIntegerMetric(buffer, countSymbol, namespace, globalTags, name, -1, tags, rate)
}

func appendIncrement(buffer []byte, namespace string, globalTags []string, name string, tags []string, rate float64) []byte {
	return appendIntegerMetric(buffer, countSymbol, namespace, globalTags, name, 1, tags, rate)
}

func appendSet(buffer []byte, namespace string, globalTags []string, name string, value string, tags []string, rate float64) []byte {
	return appendStringMetric(buffer, setSymbol, namespace, globalTags, name, value, tags, rate)
}

func appendTiming(buffer []byte, namespace string, globalTags []string, name string, value float64, tags []string, rate float64) []byte {
	return appendFloatMetric(buffer, timingSymbol, namespace, globalTags, name, value, tags, rate)
}
