[![Build Status](https://circleci.com/gh/DataDog/datadog-go.svg?style=svg)](https://app.circleci.com/pipelines/github/DataDog/datadog-go)

# Datadog Go

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/datadog-go/v5/statsd)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](http://opensource.org/licenses/MIT)

`datadog-go` is a library that provides a [DogStatsD](https://docs.datadoghq.com/developers/dogstatsd/?code-lang=go) client in Golang.

Go 1.12+ is officially supported. Older versions might work but are not tested.

The following documentation is available:

* [GoDoc documentation for Datadog Go](http://godoc.org/github.com/DataDog/datadog-go/v5/statsd)
* [Official Datadog DogStatsD documentation](https://docs.datadoghq.com/developers/dogstatsd/?code-lang=go).


<!-- vim-markdown-toc GFM -->

* [New major version](#new-major-version)
* [Installation](#installation)
    - [Supported environment variables](#supported-environment-variables)
    - [Unix Domain Sockets Client](#unix-domain-sockets-client)
* [Usage](#usage)
    - [Metrics](#metrics)
    - [Events](#events)
    - [Service Checks](#service-checks)
* [Client side aggregation](#client-side-aggregation)
    - ["Basic" aggregation](#basic-aggregation)
    - ["Extended" aggregation](#extended-aggregation)
* [Performance / Metric drops](#performance--metric-drops)
    - [Monitoring this client](#monitoring-this-client)
    - [Tweaking kernel options](#tweaking-kernel-options)
        + [Unix Domain Sockets](#unix-domain-sockets)
    - [Maximum packets size in high-throughput scenarios](#maximum-packets-size-in-high-throughput-scenarios)
* [Development](#development)
* [License](#license)
* [Credits](#credits)

<!-- vim-markdown-toc -->


## New major version

The new major version `v5` is now the default. All new features will be added to this version and only bugfixes will be
backported to `v4` (see `v4` branch).

`v5` introduce a number of breaking changes compare to `v4`, see the
[CHANGELOG](https://github.com/DataDog/datadog-go/blob/master/CHANGELOG.md#500--2021-10-01) for more information.

Note that the import paths for `v5` and `v4` are different:
- `v5`: github.com/DataDog/datadog-go/v5/statsd
- `v4`: github.com/DataDog/datadog-go/statsd

When migrating to the `v5` you will need to upgrade your imports.

## Installation

Get the code with:

```shell
$ go get github.com/DataDog/datadog-go/v5/statsd
```

Then create a new DogStatsD client:

```go
package main

import (
    "log"
    "github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
    statsd, err := statsd.New("127.0.0.1:8125")
    if err != nil {
        log.Fatal(err)
    }
}
```

Find a list of all the available options for your DogStatsD Client in the [Datadog-go godoc documentation](https://godoc.org/github.com/DataDog/datadog-go/v5/statsd#Option) or in [Datadog public DogStatsD documentation](https://docs.datadoghq.com/developers/dogstatsd/?code-lang=go#client-instantiation-parameters).

### Supported environment variables

* If the `addr` parameter is empty, the client uses the `DD_AGENT_HOST` environment variables to build a target address.
  Example: `DD_AGENT_HOST=127.0.0.1:8125` for UDP, `DD_AGENT_HOST=unix:///path/to/socket` for UDS and `DD_AGENT_HOST=\\.\pipe\my_windows_pipe` for Windows named pipe.
* If the `DD_ENTITY_ID` environment variable is found, its value is injected as a global `dd.internal.entity_id` tag. The Datadog Agent uses this tag to insert container tags into the metrics.

To enable origin detection and set the `DD_ENTITY_ID` environment variable, add the following lines to your application manifest:

```yaml
env:
  - name: DD_ENTITY_ID
    valueFrom:
      fieldRef:
        fieldPath: metadata.uid
```

* `DD_ENV`, `DD_SERVICE`, and `DD_VERSION` can be used by the statsd client to set `{env, service, version}` as global tags for all data emitted.

### Unix Domain Sockets Client

Agent v6+ accepts packets through a Unix Socket datagram connection. Details about the advantages of using UDS over UDP are available in the [DogStatsD Unix Socket documentation](https://docs.datadoghq.com/developers/dogstatsd/unix_socket/). You can use this protocol by giving a `unix:///path/to/dsd.socket` address argument to the `New` constructor.

## Usage

In order to use DogStatsD metrics, events, and Service Checks, the Agent must be [running and available](https://docs.datadoghq.com/developers/dogstatsd/?code-lang=go).

### Metrics

After the client is created, you can start sending custom metrics to Datadog. See the dedicated [Metric Submission: DogStatsD documentation](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go) to see how to submit all supported metric types to Datadog with working code examples:

* [Submit a COUNT metric](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#count).
* [Submit a GAUGE metric](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#gauge).
* [Submit a SET metric](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#set)
* [Submit a HISTOGRAM metric](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#histogram)
* [Submit a DISTRIBUTION metric](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#distribution)

Metric names must only contain ASCII alphanumerics, underscores, and periods. The client will not replace nor check for invalid characters.

Some options are suppported when submitting metrics, like [applying a sample rate to your metrics](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#metric-submission-options) or [tagging your metrics with your custom tags](https://docs.datadoghq.com/metrics/dogstatsd_metrics_submission/?code-lang=go#metric-tagging). Find all the available functions to report metrics [in the Datadog Go client GoDoc documentation](https://godoc.org/github.com/DataDog/datadog-go/v5/statsd#Client).

### Events

After the client is created, you can start sending events to your Datadog Event Stream. See the dedicated [Event Submission: DogStatsD documentation](https://docs.datadoghq.com/developers/events/dogstatsd/?code-lang=go) to see how to submit an event to your Datadog Event Stream.

### Service Checks

After the client is created, you can start sending Service Checks to Datadog. See the dedicated [Service Check Submission: DogStatsD documentation](https://docs.datadoghq.com/developers/service_checks/dogstatsd_service_checks_submission/?code-lang=go) to see how to submit a Service Check to Datadog.

## Client side aggregation

Starting with version `5.0.0` (and `3.6.0` in beta), the client offers aggregation or value packing on the client side.

This feature aims at reducing both the number of packets sent to the Agent and the packet drops in very high throughput
scenarios.

The aggregation window is 2s by default and can be changed through `WithAggregationInterval()` option. Note that the
aggregation window on the Agent side is 10s for DogStatsD metrics. So for example, setting an aggregation window of 3s in
the client will produce a spike in your dashboard every 30 second for counts metrics (as the third 10s bucket on the
Agent will receive 4 samples from the client).

Aggregation can be disabled using the `WithoutClientSideAggregation()` option.

The telemetry `datadog.dogstatsd.client.metrics` is unchanged and represents the number of metrics before aggregation.
New metrics `datadog.dogstatsd.client.aggregated_context` and `datadog.dogstatsd.client.aggregated_context_by_type` have
been introduced. See the [Monitoring this client](#monitoring-this-client) section.

### "Basic" aggregation

Enabled by default, the client will aggregate `gauge`, `count` and `set`.

This can be disabled with the `WithoutClientSideAggregation()` option.

### "Extended" aggregation

This feature is only compatible with Agent's version >=6.25.0 && <7.0.0 or Agent's versions >=7.25.0.

Disabled by default, the client can also pack multiple values for `histogram`, `distribution` and `timing` in one
message. Real aggregation is not possible for those types since the Agent also aggregates and two aggregation levels
would change the final value sent to Datadog.

When this option is enabled, the agent will buffer the metrics by combination of metric name and tags, and send them in the fewest number of messages.

For example, if we sample 3 times the same metric. Instead of sending on the network:

```
my_distribution_metric:21|d|#all,my,tags
my_distribution_metric:43.2|d|#all,my,tags
my_distribution_metric:1657|d|#all,my,tags
```

The client will send only one message:

```
my_distribution_metric:21:43.2:1657|d|#all,my,tags
```

This will greatly reduce network usage and packet drops but will slightly increase the memory and CPU usage of the
client. Looking at the telemetry metrics `datadog.dogstatsd.client.metrics_by_type` /
`datadog.dogstatsd.client.aggregated_context_by_type` will show the aggregation ratio for each type. This is an
interesting data to know how useful extended aggregation is to your app.

This can be enabled with the `WithExtendedClientSideAggregation()` option.

## Performance / Metric drops

### Monitoring this client

This client automatically injects telemetry about itself in the DogStatsD stream.
Those metrics will not be counted as custom and will not be billed. This feature can be disabled using the `WithoutTelemetry` option.

See [Telemetry documentation](https://docs.datadoghq.com/developers/dogstatsd/high_throughput/?code-lang=go#client-side-telemetry) to learn more about it.

### Tweaking kernel options

In very high throughput environments it is possible to improve performance by changing the values of some kernel options.

#### Unix Domain Sockets

- `sysctl -w net.unix.max_dgram_qlen=X` - Set datagram queue size to X (default value is usually 10).
- `sysctl -w net.core.wmem_max=X` - Set the max size of the send buffer for all the host sockets.

### Maximum packets size in high-throughput scenarios

In order to have the most efficient use of this library in high-throughput scenarios,
default values for the maximum packets size have already been set to have the best
usage of the underlying network.
However, if you perfectly know your network and you know that a different value for the maximum packets
size should be used, you can set it with the option `WithMaxBytesPerPayload`. Example:

```go
package main

import (
    "log"
    "github.com/DataDog/datadog-go/v5/statsd"
)

func main() {
    statsd, err := statsd.New("127.0.0.1:8125", WithMaxBytesPerPayload(4096))
    if err != nil {
        log.Fatal(err)
    }
}
```

## Development

Run the tests with:

    $ go test

## License

datadog-go is released under the [MIT license](http://www.opensource.org/licenses/mit-license.php).

## Credits

Original code by [ooyala](https://github.com/ooyala/go-dogstatsd).
