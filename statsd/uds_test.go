package statsd

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type testUnixgramServer struct {
	tmpDir string
	*net.UnixConn
}

// newUnixgramServer returns a unix:// addr as well as a *net.UnixConn
// server. Any error will fail the test given in param.
func newTestUnixgramServer(t *testing.T) *testUnixgramServer {
	dir, err := ioutil.TempDir("", "socket")
	if err != nil {
		t.Fatal(err)
	}

	addr := filepath.Join(dir, "dsd.socket")

	udsAddr, err := net.ResolveUnixAddr("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}

	server, err := net.ListenUnixgram("unixgram", udsAddr)
	if err != nil {
		t.Fatal(err)
	}

	return &testUnixgramServer{dir, server}
}

func (ts *testUnixgramServer) Cleanup() {
	os.RemoveAll(ts.tmpDir) // clean up
	ts.Close()
}

func (ts *testUnixgramServer) AddrString() string {
	return UnixAddressPrefix + ts.LocalAddr().String()
}

// UdsTestSuite contains generic tests valid for both UDS implementations
type UdsTestSuite struct {
	suite.Suite
	options []Option
}

func TestUdsAsync(t *testing.T) {
	suite.Run(t, &UdsTestSuite{
		options: []Option{WithAsyncUDS()},
	})
}

func TestUdsBlocking(t *testing.T) {
	suite.Run(t, &UdsTestSuite{})
}

func (suite *UdsTestSuite) TestClientUDS() {
	server := newTestUnixgramServer(suite.T())
	defer server.Cleanup()

	client, err := New(server.AddrString(), suite.options...)
	if err != nil {
		suite.T().Fatal(err)
	}

	for _, tt := range dogstatsdTests {
		client.Namespace = tt.GlobalNamespace
		client.Tags = tt.GlobalTags
		method := reflect.ValueOf(client).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Metric),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Tags),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			suite.T().Fatal(errInter.(error))
		}

		bytes := make([]byte, 1024)
		n, err := server.Read(bytes)
		if err != nil {
			suite.T().Fatal(err)
		}
		message := bytes[:n]
		if string(message) != tt.Expected {
			suite.T().Errorf("Expected: %s. Actual: %s", tt.Expected, string(message))
		}
	}
}

func (suite *UdsTestSuite) TestClientUDSConcurrent() {
	server := newTestUnixgramServer(suite.T())
	defer server.Cleanup()

	client, err := New(server.AddrString())
	if err != nil {
		suite.T().Fatal(err)
	}

	numWrites := 10
	for i := 0; i < numWrites; i++ {
		go func() {
			err := client.Gauge("test.gauge", 1.0, []string{}, 1)
			if err != nil {
				suite.T().Errorf("error outputting gauge %s", err)
			}
		}()
	}

	expected := "test.gauge:1.000000|g"
	var msgs []string
	for {
		bytes := make([]byte, 1024)
		server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := server.Read(bytes)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			break
		}
		if err != nil {
			suite.T().Fatal(err)
		}
		message := string(bytes[:n])
		msgs = append(msgs, message)
		if message != expected {
			suite.T().Errorf("Got %s, expected %s", message, expected)
		}
	}

	if len(msgs) != numWrites {
		suite.T().Errorf("Got %d writes, expected %d. Data: %v", len(msgs), numWrites, msgs)
	}
}

func (suite *UdsTestSuite) TestClientUDSClose() {
	ts := newTestUnixgramServer(suite.T())
	defer ts.Cleanup()

	// close the server ourselves and test code when nobody's listening
	ts.Close()
	client, err := New(ts.AddrString())
	if err != nil {
		suite.T().Fatal(err)
	}

	assertNotPanics(suite.T(), func() { client.Close() })
}
