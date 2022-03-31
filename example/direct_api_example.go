package main

import (
	"log"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
	client, err := statsd.New("127.0.0.1:8125",
		statsd.WithTags([]string{"env:prod", "service:myservice"}),
		statsd.WithExtendedClientSideAggregation(),
	)
	if err != nil {
		log.Fatal(err)
	}

	d, err := client.NewDistribution("my.metrics", []string{"tag1", "tag2:value"})
	if err != nil {
		log.Fatal(err)
	}

	d.Sample(21)
	d.Sample(22)
	d.Sample(23)

	client.Close()
}
