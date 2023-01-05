package statsd_test

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"

	"go.uber.org/atomic"

	"github.com/DataDog/datadog-go/v5/statsd"
)

const writerNameUDP = "udp"
const writerNameUDS = "uds"

func setupUDSClientServer(b *testing.B, options []statsd.Option) (*statsd.Client, net.Listener) {
	sockAddr := "/tmp/test.sock"
	if err := os.RemoveAll(sockAddr); err != nil {
		log.Fatal(err)
	}
	conn, err := net.Listen("unix", sockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	go func() {
		for {
			_, err := conn.Accept()
			if err != nil {
				return
			}
		}
	}()
	client, err := statsd.New("unix://"+sockAddr, options...)
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func setupUDPClientServer(b *testing.B, options []statsd.Option) (*statsd.Client, *net.UDPConn) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		b.Error(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		b.Error(err)
	}

	client, err := statsd.New(conn.LocalAddr().String(), options...)
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func setupClient(b *testing.B, transport string, extraOptions []statsd.Option) (*statsd.Client, io.Closer) {
	options := []statsd.Option{statsd.WithMaxMessagesPerPayload(1024)}
	options = append(options, extraOptions...)

	if transport == writerNameUDP {
		return setupUDPClientServer(b, options)
	}
	return setupUDSClientServer(b, options)
}

func benchmarkStatsdDifferentMetrics(b *testing.B, transport string, extraOptions ...statsd.Option) {
	client, conn := setupClient(b, transport, extraOptions)
	defer conn.Close()

	n := atomic.NewInt32(0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		testNumber := n.Add(1)
		name := fmt.Sprintf("test.metric%d", testNumber)
		for pb.Next() {
			client.Gauge(name, 1, []string{"tag:tag"}, 1)
		}
	})
	client.Flush()
	t := client.GetTelemetry()
	reportMetric(b, float64(t.TotalDroppedOnReceive)/float64(t.TotalMetrics)*100, "%_dropRate")

	b.StopTimer()
	client.Close()
}

func benchmarkStatsdSameMetrics(b *testing.B, transport string, extraOptions ...statsd.Option) {
	client, conn := setupClient(b, transport, extraOptions)
	defer conn.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.Gauge("test.metric", 1, []string{"tag:tag"}, 1)
		}
	})
	client.Flush()
	t := client.GetTelemetry()
	reportMetric(b, float64(t.TotalDroppedOnReceive)/float64(t.TotalMetrics)*100, "%_dropRate")

	b.StopTimer()
	client.Close()
}

/*
UDP with the same metric
*/

// blocking + no aggregation
func BenchmarkStatsdUDPSameMetricMutex(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDP, statsd.WithMutexMode(), statsd.WithoutClientSideAggregation())
}

// dropping + no aggregation
func BenchmarkStatsdUDPSameMetricChannel(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDP, statsd.WithChannelMode(), statsd.WithoutClientSideAggregation())
}

// blocking + aggregation
func BenchmarkStatsdUDPSameMetricMutexAggregation(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDP, statsd.WithMutexMode(), statsd.WithClientSideAggregation())
}

// dropping + aggregation
func BenchmarkStatsdUDPSameMetricChannelAggregation(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDP, statsd.WithChannelMode(), statsd.WithClientSideAggregation())
}

/*
UDP with the different metrics
*/

// blocking + no aggregation
func BenchmarkStatsdUDPDifferentMetricMutex(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDP, statsd.WithMutexMode(), statsd.WithoutClientSideAggregation())
}

// dropping + no aggregation
func BenchmarkStatsdUDPDifferentMetricChannel(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDP, statsd.WithChannelMode(), statsd.WithoutClientSideAggregation())
}

// blocking + aggregation
func BenchmarkStatsdUDPDifferentMetricMutexAggregation(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDP, statsd.WithMutexMode(), statsd.WithClientSideAggregation())
}

// dropping + aggregation
func BenchmarkStatsdUDPDifferentMetricChannelAggregation(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDP, statsd.WithChannelMode(), statsd.WithClientSideAggregation())
}

/*
UDS with the same metric
*/
// blocking + no aggregation
func BenchmarkStatsdUDSSameMetricMutex(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDS, statsd.WithMutexMode(), statsd.WithoutClientSideAggregation())
}

// dropping + no aggregation
func BenchmarkStatsdUDSSameMetricChannel(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDS, statsd.WithChannelMode(), statsd.WithoutClientSideAggregation())
}

// blocking + aggregation
func BenchmarkStatsdUDSSameMetricMutexAggregation(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDS, statsd.WithMutexMode(), statsd.WithClientSideAggregation())
}

// dropping + aggregation
func BenchmarkStatsdUDSSameMetricChannelAggregation(b *testing.B) {
	benchmarkStatsdSameMetrics(b, writerNameUDS, statsd.WithChannelMode(), statsd.WithClientSideAggregation())
}

/*
UDS with different metrics
*/
// blocking + no aggregation
func BenchmarkStatsdUDPSifferentMetricMutex(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDS, statsd.WithMutexMode(), statsd.WithoutClientSideAggregation())
}

// dropping + no aggregation
func BenchmarkStatsdUDSDifferentMetricChannel(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDS, statsd.WithChannelMode(), statsd.WithoutClientSideAggregation())
}

// blocking + aggregation
func BenchmarkStatsdUDPSifferentMetricMutexAggregation(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDS, statsd.WithMutexMode(), statsd.WithClientSideAggregation())
}

// dropping + aggregation
func BenchmarkStatsdUDSDifferentMetricChannelAggregation(b *testing.B) {
	benchmarkStatsdDifferentMetrics(b, writerNameUDS, statsd.WithChannelMode(), statsd.WithClientSideAggregation())
}
