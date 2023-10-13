package main

import (
	"log"
	"os"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
	addr := os.Getenv("DOGSTATSD_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8125"
	}
	client, err := statsd.New(addr,
		statsd.WithTags([]string{"env:prod", "service:myservice"}),
		statsd.WithErrorHandler(statsd.LoggingErrorHandler),
	)
	if err != nil {
		log.Fatal(err)
	}

	client.Histogram("my.metrics", 21, []string{"tag1", "tag2:value"}, 1)
	client.Close()
}
