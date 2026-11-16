package statsd

import (
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferedMetricContextsSampleNoTagsPaths(t *testing.T) {
	t.Run("drops_metric_when_sampling_rejects", func(t *testing.T) {
		contexts := newBufferedContexts(newHistogramMetric, 0)
		contexts.random = rand.New(rand.NewSource(1))

		require.NoError(t, contexts.sample("metric.drop", 1, nil, -1, CardinalityNotSet))
		assert.Empty(t, contexts.values)
	})

	t.Run("creates_and_reuses_metric_no_tags", func(t *testing.T) {
		contexts := newBufferedContexts(newHistogramMetric, 0)

		require.NoError(t, contexts.sample("metric.no_tags", 1, nil, 1, CardinalityNotSet))
		require.NoError(t, contexts.sample("metric.no_tags", 2, nil, 1, CardinalityNotSet))

		v := contexts.values["metric.no_tags"]
		require.NotNil(t, v)
		assert.Equal(t, int64(2), v.storedSamples)
		assert.Equal(t, []float64{1, 2}, v.data[:v.storedSamples])
	})
}

func TestBufferedMetricContextsSampleNoTagsSecondCheckLocking(t *testing.T) {
	const goroutines = 128
	contexts := newBufferedContexts(newHistogramMetric, 0)
	contexts.mutex.RLock()

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			require.NoError(t, contexts.sample("metric.concurrent", float64(i), nil, 1, CardinalityNotSet))
		}()
	}
	close(start)
	time.Sleep(20 * time.Millisecond)
	contexts.mutex.RUnlock()
	wg.Wait()

	v := contexts.values["metric.concurrent"]
	require.NotNil(t, v)
	assert.Equal(t, int64(goroutines), v.storedSamples)
}

func TestBufferedMetricContextsLargeContextBuffers(t *testing.T) {
	contexts := newBufferedContexts(newHistogramMetric, 0)

	largeTag := strings.Repeat("x", smallContextBufferSize+1)
	heapTag := strings.Repeat("x", largeContextBufferSize+1)

	require.NoError(t, contexts.sample("metric.large", 1, []string{largeTag}, 1, CardinalityLow))
	require.NoError(t, contexts.sample("metric.heap", 2, []string{heapTag}, 1, CardinalityLow))

	assert.Contains(t, contexts.values, "metric.large:low|"+largeTag)
	assert.Contains(t, contexts.values, "metric.heap:low|"+heapTag)
}

func TestBufferedMetricContextsAlignment(t *testing.T) {
	contextType := reflect.TypeOf(bufferedMetricContexts{})

	if got := contextType.Size() % 8; got != 0 {
		t.Errorf("bufferedMetricContexts size must be a multiple of 8: size=%d", contextType.Size())
	}

	nbContext, ok := contextType.FieldByName("nbContext")
	if !ok {
		t.Fatal("nbContext field not found")
	}
	if nbContext.Offset != 0 {
		t.Errorf("nbContext must be the first field: offset=%d", nbContext.Offset)
	}

	aggregatorType := reflect.TypeOf(aggregator{})
	for _, name := range []string{"histograms", "distributions", "timings"} {
		field, ok := aggregatorType.FieldByName(name)
		if !ok {
			t.Fatalf("%s field not found", name)
		}
		if field.Offset%8 != 0 {
			t.Errorf("%s must be 8-byte aligned: offset=%d", name, field.Offset)
		}
	}
}

func TestBufferedMetricContextsConcurrentFlushKeepsSamples(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
	}{
		{"untagged", nil},
		{"tagged", []string{"region:test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contexts := newBufferedContexts(newDistributionMetric, 0)
			const writers, samples = 4, 10000
			results := make(chan []metric, writers)
			for i := 0; i < writers; i++ {
				go func(first int) {
					var batches []metric
					for j := 0; j < samples; j++ {
						contexts.sample("duration", float64(first+j), tc.tags, 1, CardinalityNotSet)
						if j%10 == 0 {
							batches = contexts.flush(batches)
						}
					}
					results <- batches
				}(i * samples)
			}
			var batches []metric
			for i := 0; i < writers; i++ {
				batches = append(batches, (<-results)...)
			}
			counts := make(map[float64]int, writers*samples)
			for _, batch := range contexts.flush(batches) {
				for _, value := range batch.fvalues {
					counts[value]++
				}
			}
			for sample := 0; sample < writers*samples; sample++ {
				if count := counts[float64(sample)]; count != 1 {
					t.Fatalf("sample %d appeared %d times, want 1", sample, count)
				}
			}
			require.Len(t, counts, writers*samples)
		})
	}
}

func TestBufferedMetricContextsFlushPreservesFullReservoir(t *testing.T) {
	contexts := newBufferedContexts(newDistributionMetric, 1)
	contexts.random = rand.New(rand.NewSource(1))
	require.NoError(t, contexts.sample("duration", 10, nil, 1, CardinalityNotSet))
	buffer := contexts.values["duration"]
	batch := contexts.flush(nil)

	// Resume a writer that found the buffer before the flush. This draw would
	// replace the only value in a live reservoir.
	buffer.maybeKeepSample(20, contexts.random, &contexts.randomLock)
	require.Equal(t, []float64{10}, batch[0].fvalues)
}
