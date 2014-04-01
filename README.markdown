## Overview

Package dogstatsd provides a Go DogStatsD client. DogStatsD extends StatsD - adding tags and histograms. The documentation for DogStatsD is here: http://docs.datadoghq.com/guides/dogstatsd/

## Get the code

    $ go get github.com/xsleonard/go-dogstatsd

## Usage

    // Create the client
    c, err := dogstatsd.New("127.0.0.1:8125")
    if err != nil {
      log.Fatal(err)
    }
    // Prefix every metric with the app name
    c.Namespace = "flubber."
    // Send the EC2 availability zone as a tag with every metric
    append(c.Tags, "us-east-1a")
    err = c.Gauge("request.duration", 1.2, nil, 1)

## Development

Run the tests with:

    $ go test

## Documentation

Please see: http://godoc.org/github.com/xsleonard/go-dogstatsd

## License

go-dogstatsd is released under the [MIT license](http://www.opensource.org/licenses/mit-license.php).

## Authors

Original source by [ooyala](https://github.com/ooyala/go-dogstatsd).  This is a fork.
