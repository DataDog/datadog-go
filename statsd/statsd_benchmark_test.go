package statsd_test

import (
	"net"
	"testing"

	"github.com/DataDog/datadog-go/statsd"
)

func setupClientServer(b *testing.B) (*statsd.Client, *net.UDPConn) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		b.Error(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		b.Error(err)
	}
	client, err := statsd.New(conn.LocalAddr().String())
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func BenchmarkStatsd(b *testing.B) {
	client, conn := setupClientServer(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client.Gauge("test.metric", 1, []string{"tag:tag"}, 1)
	}
	client.Flush()

	b.StopTimer()
	client.Close()
	conn.Close()
}

func benchmarkConcurentStatsd(b *testing.B, maxProc int) {
	client, conn := setupClientServer(b)
	b.SetParallelism(maxProc)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.Gauge("test.metric", 1, []string{"tag:tag"}, 1)
		}
	})
	client.Flush()

	b.StopTimer()
	client.Close()
	conn.Close()
}

func BenchmarkConcurentStatsd10(b *testing.B) { benchmarkConcurentStatsd(b, 10) }

func BenchmarkConcurentStatsd100(b *testing.B) { benchmarkConcurentStatsd(b, 100) }

func BenchmarkConcurentStatsd1000(b *testing.B) { benchmarkConcurentStatsd(b, 1000) }

func BenchmarkConcurentStatsd10000(b *testing.B) { benchmarkConcurentStatsd(b, 10000) }

func BenchmarkConcurentStatsd100000(b *testing.B) { benchmarkConcurentStatsd(b, 100000) }

func BenchmarkConcurentStatsd200000(b *testing.B) { benchmarkConcurentStatsd(b, 200000) }
