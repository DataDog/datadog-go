package statsd

import (
	"time"
	"errors"
	"os"
	"bufio"
	"bytes"
	"io"
)

var (
	stdoutPrefix = []byte("MONITORING|")
	stdoutSuffix = []byte("\n")
)

// stdOutWriter is an internal class wrapping around management of STDOUT output formatter.
// This is useful for example in LAMBDA monitoring.
type stdOutWriter struct {
	output io.Writer  // by default it's stdout
}

// newStdOutWriter returns a pointer to a new stdOutWriter.
func newStdOutWriter() (*stdOutWriter, error) {
	return &stdOutWriter{output: os.Stdout}, nil
}

// Write data to stdout with no error handling
func (w *stdOutWriter) Write(data []byte) (n int, err error) {
	buf := bufio.NewWriter(w.output)
	for _, s := range bytes.Split(data, []byte("\n")) {
		if len(s) > 0 {
			buf.Write(stdoutPrefix)
			buf.Write(s)
			buf.Write(stdoutSuffix)
		}
	}
	totalSize := buf.Size()
	return totalSize, buf.Flush()
}

// SetWriteTimeout is not needed for stdout writer, returns error
func (w *stdOutWriter) SetWriteTimeout(time.Duration) error {
	return errors.New("SetWriteTimeout: not supported for stdout writer")
}


// Close stdout writer, does nothing.
func (w *stdOutWriter) Close() error {
	return nil
}
