// +build windows

package statsd

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
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

func TestPipeWriterConcurrency(t *testing.T) {
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

	// listen for new connections and send all incoming bytes down the
	// out channel:
	out := make(chan string)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		for {
			conn.SetReadDeadline(time.Now().Add(time.Second))
			buf := make([]byte, 512)
			n, err := conn.Read(buf)
			if err != nil {
				if err == winio.ErrTimeout {
					return
				}
				t.Fatal(err)
			}
			out <- string(buf[:n])
		}
	}()

	// connect to the pipe
	client, err := New(pipepath)
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	// create 9 goroutines sending 100 metrics concurrently
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			<-start
			defer wg.Done()
			for j := 0; j < 100; j++ {
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				if err := client.Gauge("metric", float64(i*100+j), []string{"key:val"}, 1); err != nil {
					t.Fatal(err)
				}
			}
		}(i)
	}

	close(start) // start all at the same time

	// record all output:
	var str strings.Builder
	tick := time.NewTimer(time.Second)
	defer tick.Stop()
loop:
	for {
		select {
		case s := <-out:
			str.WriteString(s)
			if !tick.Stop() {
				<-tick.C
			}
			tick.Reset(time.Second)
		case <-tick.C:
			break loop
		}
	}

	// ensure 901 metrics where there
	all := strings.Split(str.String(), "metric:")
	if len(all) != 10001 {
		t.Fatalf("Did not get all 500 metrics, got %d", len(all))
	}

	// ensure each individual metric is found and complete
	for i := 0; i < 10000; i++ {
		var found bool
		for _, m := range all {
			if strings.TrimSpace(m) == fmt.Sprintf("%d|g|#key:val", i) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Did not find metric #%d", i)
		}
	}
	wg.Wait()
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

	// subsequent attempts succeed with new connection
	for n := 0; n < 3; n++ {
		if err := client.Gauge("metric", 3, []string{"key:val"}, 1); err != nil {
			t.Fatalf("Failed to send second gauge: %s", err)
		}
		timeout = time.After(500 * time.Millisecond)
		select {
		case got := <-out:
			if exp := "metric:3|g|#key:val"; got != exp {
				t.Fatalf("Expected %q, got %q", exp, got)
			}
			return
		case <-timeout:
			continue
		}
	}
	t.Fatal("failed to reconnect")
}
