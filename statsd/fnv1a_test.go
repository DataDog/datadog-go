package statsd

import (
	stdfnv "hash/fnv"
	"testing"
)

var benchmarkFNV32Sink uint32

func BenchmarkAddString32Fragments(b *testing.B) {
	fragments := []string{
		"dd.event_query_client.requests",
		":",
		"environment:prod",
		",",
		"service:synthetics-alerting-evaluator",
		",",
		"region:us1",
		",",
		"resource_name:event_query",
		",",
		"status:success",
	}

	b.ReportAllocs()
	b.ResetTimer()

	var result uint32
	for i := 0; i < b.N; i++ {
		h := init32
		for _, fragment := range fragments {
			h = addString32(h, fragment)
		}
		result = h
	}

	benchmarkFNV32Sink = result
}

func TestHashString32MatchesStandardFNV1a(t *testing.T) {
	inputs := []string{
		"",
		"metric",
		"metric.with.ascii.tags",
		"metric.é",
		"12345678é",
		"metric.漢字",
		"日本語",
	}

	for _, input := range inputs {
		reference := stdfnv.New32a()
		_, _ = reference.Write([]byte(input))

		if got, want := hashString32(input), reference.Sum32(); got != want {
			t.Errorf(
				"hashString32(%q) = %d, want %d",
				input,
				got,
				want,
			)
		}
	}
}

func TestAddString32MatchesStandardFNV1a(t *testing.T) {
	parts := []string{
		"metric:",
		"タグ:value,",
		"status:漢字",
	}

	got := init32
	reference := stdfnv.New32a()

	for _, part := range parts {
		got = addString32(got, part)
		_, _ = reference.Write([]byte(part))
	}

	if want := reference.Sum32(); got != want {
		t.Fatalf("addString32() = %d, want %d", got, want)
	}
}
