package statsd_test

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
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
	client, err := statsd.New("unix://"+sockAddr, statsd.WithMaxMessagesPerPayload(1024))
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
	client, err := statsd.New(conn.LocalAddr().String(), statsd.WithMaxMessagesPerPayload(1024))
	if err != nil {
		b.Error(err)
	}
	return client, conn
}

func benchmarkStatsd(b *testing.B, transport string) {
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

	n := int32(0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		testNumber := atomic.AddInt32(&n, 1)
		name := fmt.Sprintf("test.metric%d", testNumber)
		for pb.Next() {
			client.Gauge(name, 1, []string{"tag:tag"}, 1)
		}
	})
	client.Flush()

	b.StopTimer()
	client.Close()
}

func BenchmarkStatsdUDP(b *testing.B) { benchmarkStatsd(b, "udp") }

func BenchmarkStatsdUDS(b *testing.B) { benchmarkStatsd(b, "uds") }
