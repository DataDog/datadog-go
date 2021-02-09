// +build windows

package statsd

import (
	"io/ioutil"
	"log"
	"os"
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
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		conn.Close()
		out <- string(buf[:n])
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
}
