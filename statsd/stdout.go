package statsd

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

var (
	defaultStdoutPrefix = []byte("MONITORING|")
	defaultStdoutSuffix = []byte("\n")
)

// stdOutWriter is an internal class wrapping around management of STDOUT output formatter.
// This is useful for example in LAMBDA monitoring.
type stdOutWriter struct {
	prefix []byte
	suffix []byte
	output io.Writer  // by default it's stdout
}

// newStdOutWriter returns a pointer to a new stdOutWriter.
func newStdOutWriter(addr string) (*stdOutWriter, error) {
	prefix := defaultStdoutPrefix
	suffix := defaultStdoutSuffix

	// allow to override
	for i, s := range strings.SplitN(addr, "/", 2) {
		if i == 0 && s != "" {
			prefix = []byte(s)
		} else if i == 1 && s != "" {
			suffix = []byte(s)
			if suffix[len(suffix)-1] != '\n' { // always ensure ends with newline
				suffix = append(suffix, '\n')
			}
		}
	}

	writer := &stdOutWriter{
		prefix: prefix,
		suffix: suffix,
		output: os.Stdout,
	}

	return writer, nil
}

// Write data to stdout with no error handling
func (w *stdOutWriter) Write(data []byte) (n int, err error) {
	buf := bufio.NewWriter(w.output)
	totalSize := 0
	for _, s := range bytes.Split(data, []byte("\n")) {
		if len(s) > 0 {
			buf.Write(w.prefix)
			buf.Write(s)
			buf.Write(w.suffix)
			totalSize += len(w.prefix) + len(s) + len(w.suffix) + 1
		}
	}
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
