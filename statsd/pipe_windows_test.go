// +build windows

package statsd

import (
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

// acceptOne accepts one single connection from ln, reads 512 bytes from it
// and sends it to the out channel, afterwards closing the connection.
func acceptOne(t *testing.T, ln net.Listener, out chan string) {
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	out <- string(buf[:n])
}

func TestPipeWriter(t *testing.T) {
	f, err := ioutil.TempFile("", "test-pipe-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	pipepath := WindowsPipeAddressPrefix + f.Name()
	ln, err := winio.ListenPipe(pipepath, &winio.PipeConfig{
		SecurityDescriptor: "D:AI(A;;GA;;;WD)",
		InputBufferSize:    1_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan string)
	go acceptOne(t, ln, out)

	client, err := New(pipepath)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Gauge("metric", 1, []string{"key:val"}, 1); err != nil {
		t.Fatal(err)
	}
	got := <-out
	if exp := "metric:1|g|#key:val"; got != exp {
		t.Fatalf("Expected %q, got %q", exp, got)
	}
}

func TestPipeWriterReconnect(t *testing.T) {
	f, err := ioutil.TempFile("", "test-pipe-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	pipepath := WindowsPipeAddressPrefix + f.Name()
	ln, err := winio.ListenPipe(pipepath, &winio.PipeConfig{
		SecurityDescriptor: "D:AI(A;;GA;;;WD)",
		InputBufferSize:    1_000_000,
	})
	if err != nil {
		t.Fatalf("Listen: %s", err)
	}
	out := make(chan string)
	go acceptOne(t, ln, out)
	client, err := New(pipepath)
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	// first attempt works, then connection closes
	if err := client.Gauge("metric", 1, []string{"key:val"}, 1); err != nil {
		t.Fatalf("Failed to send gauge: %s", err)
	}
	timeout := time.After(1 * time.Second)
	select {
	case got := <-out:
		if exp := "metric:1|g|#key:val"; got != exp {
			t.Fatalf("Expected %q, got %q", exp, got)
		}
	case <-timeout:
		t.Fatal("timeout1")
	}

	// second attempt fails by attempting the same connection
	go acceptOne(t, ln, out)
	if err := client.Gauge("metric", 2, []string{"key:val"}, 1); err != nil {
		t.Fatalf("Failed to send second gauge: %s", err)
	}
	timeout = time.After(100 * time.Millisecond)
	select {
	case <-out:
		t.Fatal("Second attempt should have timed out")
	case <-timeout:
		// ok
	}

	// third attempt succeeds with new connection
	if err := client.Gauge("metric", 3, []string{"key:val"}, 1); err != nil {
		t.Fatalf("Failed to send second gauge: %s", err)
	}
	timeout = time.After(100 * time.Millisecond)
	select {
	case got := <-out:
		if exp := "metric:3|g|#key:val"; got != exp {
			t.Fatalf("Expected %q, got %q", exp, got)
		}
	case <-timeout:
		t.Fatal("Third attempt should have succeeded")
	}
}
