package main

import (
	"log"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
	client, err := statsd.New("unix:///tmp/test.socket",
		statsd.WithTags([]string{"env:prod", "service:myservice"}),
	)
	if err != nil {
		log.Fatal(err)
	}

	for {
		client.Histogram("my.metrics", 21, []string{"tag1", "tag2:value"}, 1)
	}
	client.Close()
}
