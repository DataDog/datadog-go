package statsd

import (
	"errors"
	"os"
	"testing"
)

func TestAddressFromEnvironment(t *testing.T) {
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
		// Nominal case
		{"127.0.0.1:8125", "", "", WriterNameUDP, "127.0.0.1:8125", nil},
		// Parameter overrides environment
		{"127.0.0.1:8125", "10.12.16.9", "1234", WriterNameUDP, "127.0.0.1:8125", nil},
		// Host and port passed as env
		{"", "10.12.16.9", "1234", WriterNameUDP, "10.12.16.9:1234", nil},
		// Host passed, default port
		{"", "10.12.16.9", "", WriterNameUDP, "10.12.16.9:8125", nil},
		// unix socket
		{"", "unix:///var/run/socket.sock", "", WriterNameUDS, "/var/run/socket.sock", nil},
		// No autodetection failed
		{"", "", "", WriterNameUDP, "", errors.New("No address passed and autodetection from environment failed")},
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
		case WriterNameUDP:
			writer := writer.(*udpWriter)
			if writer.remoteAddr().String() != tc.expectedAddr {
				t.Errorf("Expected %q, got %q", tc.expectedAddr, writer.remoteAddr().String())
			}
		case WriterNameUDS:
			writer := writer.(*udsWriter)
			if writer.addr.String() != tc.expectedAddr {
				t.Errorf("Expected %q, got %q", tc.expectedAddr, writer.addr.String())
			}
		}
		writer.Close()
	}
}
