// +build windows

package statsd

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/Microsoft/go-winio"
)

func TestPipeWriter(t *testing.T) {
	f, err := ioutil.TempFile("", "test-pipe-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name())
	pipepath := WindowsPipeAddressPrefix + f.Name()
	ln, err := winio.ListenPipe(pipepath, &winio.PipeConfig{
		SecurityDescriptor: "D:AI(A;;GA;;;WD)",
		InputBufferSize:    1_000_000,
	})
	if err != nil {
		log.Fatal(err)
	}
	out := make(chan string)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		out <- string(buf[:n])
		conn.Close()
	}()

	client, err := New(pipepath)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Gauge("metric", 1, []string{"key:val"}, 1); err != nil {
		log.Fatal(err)
	}
	got := <-out
	if exp := "metric:1|g|#key:val"; got != exp {
		t.Fatalf("Expected %q, got %q", exp, got)
	}
	wg.Wait() // wait to close conn and goroutine
}
