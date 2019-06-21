package statsd_test

import (
	"log"
	"net"
	"os"
	"testing"

	"github.com/DataDog/datadog-go/statsd"
)

func setupUDSClientServer(b *testing.B) (*statsd.Client, net.Listener) {
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
	client, err := statsd.New("unix://"+sockAddr, statsd.Buffered(), statsd.WithMaxMessagesPerPayload(1024), statsd.WithAsyncUDS())
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func setupUDPClientServer(b *testing.B) (*statsd.Client, *net.UDPConn) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		b.Error(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		b.Error(err)
	}
	client, err := statsd.New(conn.LocalAddr().String(), statsd.Buffered(), statsd.WithMaxMessagesPerPayload(1024), statsd.WithAsyncUDS())
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func benchmarkStatsd(b *testing.B, maxProc int, transport string) {
	var client *statsd.Client
	if transport == "udp" {
		var conn *net.UDPConn
		client, conn = setupUDPClientServer(b)
		defer conn.Close()
	} else {
		var conn net.Listener
		client, conn = setupUDSClientServer(b)
		defer conn.Close()
	}

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
}

func BenchmarkStatsdUDP1(b *testing.B) { benchmarkStatsd(b, 1, "udp") }

func BenchmarkStatsdUDP10(b *testing.B) { benchmarkStatsd(b, 10, "udp") }

func BenchmarkStatsdUDP100(b *testing.B) { benchmarkStatsd(b, 100, "udp") }

func BenchmarkStatsdUDP1000(b *testing.B) { benchmarkStatsd(b, 1000, "udp") }

func BenchmarkStatsdUDP10000(b *testing.B) { benchmarkStatsd(b, 10000, "udp") }

func BenchmarkStatsdUDP100000(b *testing.B) { benchmarkStatsd(b, 100000, "udp") }

func BenchmarkStatsdUDP200000(b *testing.B) { benchmarkStatsd(b, 200000, "udp") }

func BenchmarkStatsdUDS1(b *testing.B) { benchmarkStatsd(b, 1, "uds") }

func BenchmarkStatsdUDS10(b *testing.B) { benchmarkStatsd(b, 10, "uds") }

func BenchmarkStatsdUDS100(b *testing.B) { benchmarkStatsd(b, 100, "uds") }

func BenchmarkStatsdUDS1000(b *testing.B) { benchmarkStatsd(b, 1000, "uds") }

func BenchmarkStatsdUDS10000(b *testing.B) { benchmarkStatsd(b, 10000, "uds") }

func BenchmarkStatsdUDS100000(b *testing.B) { benchmarkStatsd(b, 100000, "uds") }

func BenchmarkStatsdUDS200000(b *testing.B) { benchmarkStatsd(b, 200000, "uds") }
