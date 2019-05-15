[![Build Status](https://travis-ci.com/DataDog/datadog-go.svg?branch=master)](https://travis-ci.com/DataDog/datadog-go)
# Datadog Go

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/datadog-go/statsd)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](http://opensource.org/licenses/MIT)

datadog-go is a library that provides a [dogstatsd](http://docs.datadoghq.com/guides/dogstatsd/) client in Golang.

Go 1.7+ is officially supported. Older versions might work but are not tested.

## Get the code

    $ go get github.com/DataDog/datadog-go/statsd

## Usage

Start by creating a new client:

```go
client, err := statsd.New("127.0.0.1:8125",
    statsd.WithNamespace("flubber."),               // prefix every metric with the app name
    statsd.WithTags([]string{"region:us-east-1a"}), // send the EC2 availability zone as a tag with every metric
    // add more options here...
)
if err != nil {
    log.Fatal(err)
}
```

You can find a list of all the available options [here](https://godoc.org/github.com/DataDog/datadog-go/statsd#Option).

After the client is created, you can start sending metrics: 

```go
client.Gauge("kafka.health", 1, []string{"env:production", "partition:1", "partition:2"}, 1)
```

Each metric call requires the same parameters:

- `name (string)`: The metric name that will show up in Datadog
- `value`: The value of the metric. Type depends on the metric type
- `tags ([]string)`: The list of tags to apply to the metric. Multiple tags can have the same key
- `rate (float)`: The sampling rate in `[0,1]`. For example `0.5` means that half the calls will result in a metric being sent to Datadog. Set to `1` to disable sampling

You can find all the available functions to report metrics [here](https://godoc.org/github.com/DataDog/datadog-go/statsd#Client).

## Supported environment variables

- The client can use the `DD_AGENT_HOST` and (optionally) the `DD_DOGSTATSD_PORT` environment variables to build the target address if the `addr` parameter is empty.
- If the `DD_ENTITY_ID` environment variable is found, its value will be injected as a global `dd.internal.entity_id` tag. This tag will be used by the Datadog Agent to insert container tags to the metrics. You should only `append` to the `c.Tags` slice to avoid overwriting this global tag.

To enable origin detection and set the `DD_ENTITY_ID` environment variable, add the following lines to your application manifest
```yaml
env:
  - name: DD_ENTITY_ID
    valueFrom:
      fieldRef:
        fieldPath: metadata.uid
```

## Unix Domain Sockets Client

The version 6 (and above) of the Agent accepts packets through a Unix Socket datagram connection.
Details about the advantages of using UDS over UDP are available in our [docs](https://docs.datadoghq.com/developers/dogstatsd/unix_socket/).

You can use this protocol by giving a `unix:///path/to/dsd.socket` address argument to the `New` constructor.


### Blocking vs Asynchronous behavior

When transporting DogStatsD datagram over UDS, two modes are available, "blocking" and "asynchronous".

"blocking" allows for error checking but does not guarantee that calls to the library will return immediately. For example `client.Gauge(...)` might take an arbitrary amount of time to complete depending on server performance and load. If used in a hot path of your application, this behavior might significantly impact its performance.

"asynchronous" does not allow for error checking but guarantees that calls are instantaneous (<1ms). This is similar to UDP behavior.

Currently, in 2.x, "blocking" is the default behavior to ensure backward compatibility. To use the "asynchronous" behavior, use the `statsd.WithAsyncUDS()` option.
We recommend enabling the "asynchronous" mode.

## Performance / Metric drops

If you plan on sending metrics at a significant rate using this client, depending on your use case, you might need to configure the client and the datadog agent (dogstatsd server) to improve the performance and/or avoid dropping metrics.

### Buffering Client

DogStatsD accepts packets with multiple statsd messages in them. Using the `statsd.Buffered()` option will buffer up commands and send them when the buffer is reached or after 100msec.

Ex:
```go
client, err := statsd.New("127.0.0.1:8125",
    statsd.Buffered(), // enable buffering
    statsd.WithMaxMessagesPerPayload(16), // sets the maximum number of messages in a single datagram
)
```

### Tweaking kernel options

In very high throughput environments it is possible to improve performance further by changing the values of some kernel options.

#### Unix Domain Sockets

If you're still seeing datagram drops after enabling and configuring the buffering options, the following kernel options can help:
- `sysctl -w net.unix.max_dgram_qlen=X` - Set datagram queue size to X (default value is usually 10).
- `sysctl -w net.core.wmem_max=X` - Set the max size of the send buffer for all the host sockets.

## Development

Run the tests with:

    $ go test

## Documentation

Please see: http://godoc.org/github.com/DataDog/datadog-go/statsd

## License

datadog-go is released under the [MIT license](http://www.opensource.org/licenses/mit-license.php).

## Credits

Original code by [ooyala](https://github.com/ooyala/go-dogstatsd).
