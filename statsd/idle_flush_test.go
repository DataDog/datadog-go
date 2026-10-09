package statsd

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type idleFlushWriter struct {
	packets chan string
}

func (w *idleFlushWriter) Write(p []byte) (int, error) {
	w.packets <- string(p)
	return len(p), nil
}

func (w *idleFlushWriter) Close() error {
	return nil
}

func TestFlushAfterIdle(t *testing.T) {
	for name, mode := range map[string]Option{"mutex": WithMutexMode(), "channel": WithChannelMode()} {
		t.Run(name, func(t *testing.T) {
			w := &idleFlushWriter{packets: make(chan string, 10)}
			c, err := NewWithWriter(w, mode, WithoutTelemetry(), WithoutClientSideAggregation(), WithBufferFlushInterval(20*time.Millisecond))
			require.NoError(t, err)
			defer c.Close()

			for i := 0; i < 3; i++ {
				time.Sleep(60 * time.Millisecond)
				require.NoError(t, c.Count("after_idle", int64(i+1), nil, 1))
				select {
				case packet := <-w.packets:
					require.Contains(t, packet, fmt.Sprintf("after_idle:%d|c", i+1))
				case <-time.After(time.Second):
					t.Fatal("buffered metric was not flushed")
				}
			}
		})
	}
}

func TestConcurrentFlushKeepsBufferedMetrics(t *testing.T) {
	w := &idleFlushWriter{packets: make(chan string, 1000)}
	c, err := NewWithWriter(w, WithoutTelemetry(), WithoutClientSideAggregation(), WithBufferFlushInterval(time.Millisecond))
	require.NoError(t, err)
	defer c.Close()

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if err := c.Count(fmt.Sprintf("metric_%d_%d", g, i), 1, nil, 1); err != nil {
					t.Error(err)
				}
				if i%5 == 0 {
					if err := c.Flush(); err != nil {
						t.Error(err)
					}
				}
			}
		}(g)
	}
	wg.Wait()

	seen := make(map[string]bool)
	timeout := time.After(time.Second)
	for len(seen) < 400 {
		select {
		case packet := <-w.packets:
			for _, metric := range strings.Split(strings.TrimSpace(packet), "\n") {
				require.False(t, seen[metric], "duplicate metric: %s", metric)
				seen[metric] = true
			}
		case <-timeout:
			t.Fatalf("received %d of 400 buffered metrics", len(seen))
		}
	}
}

func TestCloseWithPendingFlushTimer(t *testing.T) {
	w := &idleFlushWriter{packets: make(chan string, 1)}
	c, err := NewWithWriter(w, WithoutTelemetry(), WithoutClientSideAggregation(), WithBufferFlushInterval(time.Hour))
	require.NoError(t, err)
	require.NoError(t, c.Count("pending", 1, nil, 1))

	closed := make(chan error, 1)
	go func() {
		closed <- c.Close()
	}()

	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("close waited for the flush timer")
	}
	select {
	case packet := <-w.packets:
		require.Contains(t, packet, "pending:1|c")
	case <-time.After(time.Second):
		t.Fatal("close did not flush the pending metric")
	}
}
