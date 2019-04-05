package statsd

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSendBlockingUDSErrors(t *testing.T) {
	dir, err := ioutil.TempDir("", "socket")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	message := "test message"

	addr := filepath.Join(dir, "dsd.socket")
	udsAddr, err := net.ResolveUnixAddr("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}

	client, err := New(UnixAddressPrefix + addr)
	if err != nil {
		t.Fatal(err)
	}

	// Server not listening yet
	err = client.sendMsg([]byte(message))
	if err == nil || !strings.HasSuffix(err.Error(), "no such file or directory") {
		t.Errorf("Expected error \"no such file or directory\", got: %s", err.Error())
	}

	// Start server and send packet
	server, err := net.ListenUnixgram("unixgram", udsAddr)
	if err != nil {
		t.Fatal(err)
	}
	err = client.sendMsg([]byte(message))
	if err != nil {
		t.Errorf("Expected no error to be returned when server is listening, got: %s", err.Error())
	}
	bytes := make([]byte, 1024)
	n, err := server.Read(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes[:n]) != message {
		t.Errorf("Expected: %s. Actual: %s", string(message), string(bytes))
	}

	// close server and send packet
	server.Close()
	os.Remove(addr)
	err = client.sendMsg([]byte(message))
	if err == nil {
		t.Error("Expected an error, got nil")
	}

	// Restart server and send packet
	server, err = net.ListenUnixgram("unixgram", udsAddr)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	defer server.Close()
	err = client.sendMsg([]byte(message))
	if err != nil {
		t.Errorf("Expected no error to be returned when server is listening, got: %s", err.Error())
	}

	bytes = make([]byte, 1024)
	n, err = server.Read(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes[:n]) != message {
		t.Errorf("Expected: %s. Actual: %s", string(message), string(bytes))
	}
}

func TestSendBlockingUDSIgnoreErrors(t *testing.T) {
	client, err := New("unix://invalid")
	if err != nil {
		t.Fatal(err)
	}

	// Default mode throws error
	err = client.sendMsg([]byte("message"))
	if err == nil || !strings.HasSuffix(err.Error(), "no such file or directory") {
		t.Errorf("Expected error \"connect: no such file or directory\", got: %s", err.Error())
	}

	// Skip errors
	client.SkipErrors = true
	err = client.sendMsg([]byte("message"))
	if err != nil {
		t.Errorf("Expected no error to be returned when in skip errors mode, got: %s", err.Error())
	}
}
