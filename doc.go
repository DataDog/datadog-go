/*
Package statsd provides a Go dogstatsd client. Dogstatsd extends the popular statsd,
adding tags and histograms and pushing upstream to Datadog.

Refer to http://docs.datadoghq.com/guides/dogstatsd/ for information about DogStatsD.

Example Usage:

    // Create the client
    c, err := statsd.New("127.0.0.1:8125")
    if err != nil {
        log.Fatal(err)
    }
    // Prefix every metric with the app name
    c.Namespace = "flubber."
    // Send the EC2 availability zone as a tag with every metric
    c.Tags = append(c.Tags, "us-east-1a")
    err = c.Gauge("request.duration", 1.2, nil, 1)

statsd is based on go-statsd-client.
*/
package datadog_go

