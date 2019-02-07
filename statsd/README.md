## Overview

Package `statsd` provides a Go [dogstatsd](http://docs.datadoghq.com/guides/dogstatsd/) client.  Dogstatsd extends Statsd, adding tags
and histograms.

## Get the code

    $ go get github.com/DataDog/datadog-go/statsd

## Usage

```go
// Create the client
c, err := statsd.New("127.0.0.1:8125")
if err != nil {
    log.Fatal(err)
}
// Prefix every metric with the app name
c.Namespace = "flubber."
// Send the EC2 availability zone as a tag with every metric
c.Tags = append(c.Tags, "us-east-1a")

// Do some metrics!
err = c.Gauge("request.queue_depth", 12, nil, 1)
err = c.Timing("request.duration", duration, nil, 1) // Uses a time.Duration!
err = c.TimeInMilliseconds("request", 12, nil, 1)
err = c.Incr("request.count_total", nil, 1)
err = c.Decr("request.count_total", nil, 1)
err = c.Count("request.count_total", 2, nil, 1)
```
## Unix Domain Sockets Client

DogStatsD version 6 accepts packets through a Unix Socket datagram connection. You can use this protocol by giving a
`unix:///path/to/dsd.socket` addr argument to the `New` or `NewBufferingClient`.

With this protocol, writes can become blocking if the server's receiving buffer is full. Our default behaviour is to
timeout and drop the packet after 1 ms. You can set a custom timeout duration via the `SetWriteTimeout` method.

The default mode is to pass write errors from the socket to the caller. This includes write errors the library will
automatically recover from (DogStatsD server not ready yet or is restarting). You can drop these errors and emulate
the UDP behaviour by setting the `SkipErrors` property to `true`. Please note that packets will be dropped in both modes.

## Improving the performance / metric drops

If you plan on sending metrics at a significant rate using this client, depending on your use case, you might need to configure the client and the agent to improve the performance or avoid dropping metrics.

### Buffering Client

DogStatsD accepts packets with multiple statsd payloads in them. Using the BufferingClient via `NewBufferingClient` will buffer up commands and send them when the buffer is reached or after 100msec.

Ex:
```go
client, err := statsd.NewBuffered("127.0.0.1:8125", 128)
```

By default, the client is configured to send datagrams with a maximum size of `1432`. Use `client.DatagramMaxSize` to increase this size. The goal is to avoid fragmentation when using UDP. However, with UDS this is completely sub-optimal as UDS communication does not go through an underlying network protocol but within the kernel and does not have any form of fragmentation. The only limit to keep in mind is the value of the send buffer (`net.core.wmem_max`).

The agent also has a maximum size for the incoming datagrams that *needs* to be set to a value >= than `client.DatagramMaxSize`. If not the agent will receive truncated datagrams. The name of the option is `dogstatsd_buffer_size`.

Ex:
Client:
```go
client.DatagramMaxSize = 32768
```
Agent (datadog.yaml)
```yaml
dogstatsd_buffer_size: 32768
```

### Tweaking kernel options

In very high throughput environments it's possible to improve things even further by changing the values of some kernel options.

#### Unix Domain Sockets

If you're still seeing datagram drops after enabling and configuring the buffering options, the following kernel options can help:
- `sysctl -w net.unix.max_dgram_qlen=X` - Set datagram queue size to X (default value is usually 10).
- `sysctl -w net.core.wmem_max=X` - Set the max size of all the host sockets send buffer to X.

## Development

Run the tests with:

    $ go test

## Documentation

Please see: http://godoc.org/github.com/DataDog/datadog-go/statsd

## License

go-dogstatsd is released under the [MIT license](http://www.opensource.org/licenses/mit-license.php).

## Credits

Original code by [ooyala](https://github.com/ooyala/go-dogstatsd).
