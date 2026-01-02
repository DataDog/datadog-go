package main

import (
	"log"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
	if err := runExample(); err != nil {
		log.Fatal(err)
	}
}

func runExample() (err error) {
	client, err := statsd.New("127.0.0.1:8125",
		statsd.WithTags([]string{"env:prod", "service:myservice"}),
	)
	if err != nil {
		return err
	}

	if err = client.Histogram("my.metrics", 21, []string{"tag1", "tag2:value"}, 1); err != nil {
		return err
	}
	return client.Close()
}
