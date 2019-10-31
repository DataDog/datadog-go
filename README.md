[![Build Status](https://travis-ci.com/DataDog/datadog-go.svg?branch=master)](https://travis-ci.com/DataDog/datadog-go)
# Datadog Go

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/datadog-go/statsd)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](http://opensource.org/licenses/MIT)

`datadog-go` is a library that provides a [DogStatsD](https://docs.datadoghq.com/developers/dogstatsd/?tab=go) client in Golang.

Go 1.7+ is officially supported. Older versions might work but are not tested.

The following documentations are available:

* [Godoc documentation for Datadog Go](http://godoc.org/github.com/DataDog/datadog-go/statsd)
* [Official Datadog DogStatsD documentation](https://docs.datadoghq.com/developers/dogstatsd/?tab=go).

## Installation

Get the code with:

```shell
$ go get github.com/DataDog/datadog-go/statsd
```

Then create a new DogStatsD client:

```go
package main

import (
    "log"
    "github.com/DataDog/datadog-go/statsd"
)

func main() {
    statsd, err: = statsd.New("127.0.0.1:8125")
    if err != nil {
        log.Fatal(err)
    }
}
```

Find a list of all the available options for your DogStatsD Client in the [Datadog-go godoc documentation](https://godoc.org/github.com/DataDog/datadog-go/statsd#Option).

### Supported environment variables

* The client can use the `DD_AGENT_HOST` and (optionally) the `DD_DOGSTATSD_PORT` environment variables to build the target address if the `addr` parameter is empty.
* If the `DD_ENTITY_ID` environment variable is found, its value will be injected as a global `dd.internal.entity_id` tag. This tag will be used by the Datadog Agent to insert container tags to the metrics. You should only `append` to the `c.Tags` slice to avoid overwriting this global tag.

To enable origin detection and set the `DD_ENTITY_ID` environment variable, add the following lines to your application manifest:

```yaml
env:
  - name: DD_ENTITY_ID
    valueFrom:
      fieldRef:
        fieldPath: metadata.uid
```

### Unix Domain Sockets Client

The version 6 (and above) of the Agent accepts packets through a Unix Socket datagram connection. Details about the advantages of using UDS over UDP are available in our [docs](https://docs.datadoghq.com/developers/dogstatsd/unix_socket/). You can use this protocol by giving a `unix:///path/to/dsd.socket` address argument to the `New` constructor.

## Usage

For usage of DogStatsD metrics, the Agent must be [running and available](https://docs.datadoghq.com/developers/dogstatsd/).

### Metrics

After the client is created, you can start sending custom metrics to Datadog. See the dedicated [Metric Submission: DogStatsD documentation](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go) to see how to submit all supported metric types to Datadog with working code examples:

* [Submit a COUNT metric](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#count).
* [Submit a GAUGE metric](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#gauge).
* [Submit a SET metric](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#set)
* [Submit a HISTOGRAM metric](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#histogram)
* [Submit a DISTRIBUTION metric](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#distribution)

Some options are suppported when submitting metrics, like [applying a Sample Rate to your metrics](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#metric-submission-options) or [Tagging your metrics with your custom Tags](https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/?tab=go#metric-tagging). Find all the available functions to report metrics [in the Datadog-go Client godoc documentation](https://godoc.org/github.com/DataDog/datadog-go/statsd#Client).

### Events

After the client is created, you can start sending events to your Datadog Event Stream. See the dedicated [Event Submission: DogStatsD documentation](https://docs.datadoghq.com/developers/events/dogstatsd/?tab=go) to see how to submit an event to Datadog Event Stream.

### Service Checks

After the client is created, you can start sending Service Checks to Datadog. See the dedicated [Service Check Submission: DogStatsD documentation](https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/?tab=go) to see how to submit a Service Check to Datadog.

## Performance / Metric drops

### Tweaking kernel options

In very high throughput environments it is possible to improve performance by changing the values of some kernel options.

#### Unix Domain Sockets

- `sysctl -w net.unix.max_dgram_qlen=X` - Set datagram queue size to X (default value is usually 10).
- `sysctl -w net.core.wmem_max=X` - Set the max size of the send buffer for all the host sockets.

## Development

Run the tests with:

    $ go test

## License

datadog-go is released under the [MIT license](http://www.opensource.org/licenses/mit-license.php).

## Credits

Original code by [ooyala](https://github.com/ooyala/go-dogstatsd).
