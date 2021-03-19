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

func TestAddressFromEnvironmentWindows(t *testing.T) {
	hostInitialValue, hostInitiallySet := os.LookupEnv(autoHostEnvName)
	if hostInitiallySet {
		defer os.Setenv(autoHostEnvName, hostInitialValue)
	} else {
		defer os.Unsetenv(autoHostEnvName)
	}
	portInitialValue, portInitiallySet := os.LookupEnv(autoPortEnvName)
	if portInitiallySet {
		defer os.Setenv(autoPortEnvName, portInitialValue)
	} else {
		defer os.Unsetenv(autoPortEnvName)
	}

	for _, tc := range []struct {
		addrParam          string
		hostEnv            string
		portEnv            string
		expectedWriterType string
		expectedAddr       string
		expectedErr        error
	}{
		// unix socket
		{"", `\\.\pipe\C:\testing`, "", WriterWindowsPipe, `\\.\pipe\C:\testing`, nil},
	} {
		os.Setenv(autoHostEnvName, tc.hostEnv)
		os.Setenv(autoPortEnvName, tc.portEnv)

		// Test the error
		writer, writerType, err := resolveAddr(tc.addrParam)
		if tc.expectedErr == nil {
			if err != nil {
				t.Errorf("Unexpected error while getting writer: %s", err)
			}
		} else {
			if err == nil || tc.expectedErr.Error() != err.Error() {
				t.Errorf("Unexpected error %q, got %q", tc.expectedErr, err)
			}
		}

		if writer == nil {
			if tc.expectedAddr != "" {
				t.Error("Nil writer while we were expecting a valid one")
			}

			// Do not test for the addr if writer is nil
			continue
		}

		if writerType != tc.expectedWriterType {
			t.Errorf("expected writer type %q, got %q", tc.expectedWriterType, writerType)
		}

		switch writerType {
		case WriterWindowsPipe:
			writer := writer.(*pipeWriter)
			writer.conn.RemoteAddr()
			if writer.pipepath != tc.expectedAddr {
				t.Errorf("Expected %q, got %q", tc.expectedAddr, writer.pipepath)
			}
		default:
			t.Errorf("was not expecting writer type %s", writerType)
		}
		writer.Close()
	}
}
