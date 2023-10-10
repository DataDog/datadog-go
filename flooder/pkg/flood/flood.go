package flood

import (
	"github.com/spf13/cobra"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
)

type client struct {
	client              *statsd.Client
	pointsPer10Seconds  int
	sendAtStartOfBucket bool
}

func initClient(command *cobra.Command) (*client, error) {
	var options []statsd.Option

	tags := []string{}

	b, err := command.Flags().GetBool("client-side-aggregation")
	if err != nil {
		return nil, err
	}
	if b {
		options = append(options, statsd.WithClientSideAggregation())
	} else {
		options = append(options, statsd.WithoutClientSideAggregation())
	}
	tags = append(tags, "client-side-aggregation:"+strconv.FormatBool(b))

	b, err = command.Flags().GetBool("extended-client-side-aggregation")
	if err != nil {
		return nil, err
	}
	if b {
		options = append(options, statsd.WithExtendedClientSideAggregation())
	}
	tags = append(tags, "extended-client-side-aggregation"+strconv.FormatBool(b))

	b, err = command.Flags().GetBool("channel-mode")
	if err != nil {
		return nil, err
	}
	if b {
		options = append(options, statsd.WithChannelMode())
	}
	tags = append(tags, "channel-mode:"+strconv.FormatBool(b))

	i, err := command.Flags().GetInt("channel-mode-buffer-size")
	if err != nil {
		return nil, err
	}
	options = append(options, statsd.WithChannelModeBufferSize(i))
	tags = append(tags, "channel-mode-buffer-size:"+strconv.Itoa(i))

	i, err = command.Flags().GetInt("sender-queue-size")
	if err != nil {
		return nil, err
	}
	options = append(options, statsd.WithSenderQueueSize(i))
	tags = append(tags, "sender-queue-size:"+strconv.Itoa(i))

	d, err := command.Flags().GetDuration("buffer-flush-interval")
	if err != nil {
		return nil, err
	}
	options = append(options, statsd.WithBufferFlushInterval(d))
	tags = append(tags, "buffer-flush-interval:"+d.String())

	d, err = command.Flags().GetDuration("write-timeout")
	if err != nil {
		return nil, err
	}
	options = append(options, statsd.WithWriteTimeout(d))
	tags = append(tags, "write-timeout:"+d.String())

	pointsPer10Seconds, err := command.Flags().GetInt("points-per-10seconds")
	if err != nil {
		return nil, err
	}
	tags = append(tags, "points-per-10seconds:"+strconv.Itoa(pointsPer10Seconds))

	sendAtStart, err := command.Flags().GetBool("send-at-start-of-bucket")
	if err != nil {
		return nil, err
	}
	tags = append(tags, "send-at-start-of-bucket:"+strconv.FormatBool(sendAtStart))

	address, err := command.Flags().GetString("address")
	if err != nil {
		return nil, err
	}
	tags = append(tags, "address:"+address)

	transport := "udp"
	if strings.HasPrefix(address, statsd.UnixAddressPrefix) {
		transport = "unix"
	}
	tags = append(tags, "transport:"+transport)

	telemetryAddress, err := command.Flags().GetString("telemetry-address")
	if err != nil {
		return nil, err
	}
	if telemetryAddress == "" {
		telemetryAddress = address
	}
	tags = append(tags, "telemetry-address:"+telemetryAddress)
	options = append(options, statsd.WithTelemetryAddr(telemetryAddress))

	telemetryTransport := "udp"
	if strings.HasPrefix(telemetryAddress, statsd.UnixAddressPrefix) {
		telemetryTransport = "unix"
	}
	tags = append(tags, "telemetry-transport:"+telemetryTransport)

	t, err := command.Flags().GetStringSlice("tags")
	tags = append(tags, t...)
	h := hash(tags)
	tags = append(tags, "client-hash:"+strconv.Itoa(int(h)))

	options = append(options, statsd.WithTags(tags))

	log.Printf("Tags: %v - Hash: %x", tags, h)

	options = append(options, statsd.WithOriginDetection())

	c, err := statsd.New(address, options...)
	if err != nil {
		return nil, err
	}

	return &client{
		client:              c,
		pointsPer10Seconds:  pointsPer10Seconds,
		sendAtStartOfBucket: sendAtStart,
	}, nil
}

func hash(s []string) uint32 {
	h := fnv.New32a()
	for _, e := range s {
		h.Write([]byte(e))
	}
	return h.Sum32()
}

func Flood(command *cobra.Command, args []string) {
	c, err := initClient(command)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Sending %d points per 10 seconds", c.pointsPer10Seconds)

	for {
		t1 := time.Now()

		for sent := 0; sent < c.pointsPer10Seconds; sent++ {
			err := c.client.Incr("flood.dogstatsd.count", []string{}, 1)
			if err != nil {
				log.Printf("Error: %v", err)
			}
			if !c.sendAtStartOfBucket {
				time.Sleep(time.Duration(8) * time.Second / time.Duration(c.pointsPer10Seconds))
			}
		}
		err := c.client.Count("flood.dogstatsd.expected", int64(c.pointsPer10Seconds), []string{}, 1)
		if err != nil {
			log.Printf("Error: %v", err)
		}

		t2 := time.Now()

		duration := t2.Sub(t1)
		s := time.Duration(10)*time.Second - duration
		if s > 0 {
			// Sleep until the next bucket
			log.Printf("Sleeping for %f seconds", s.Seconds())
			time.Sleep(s)
		} else {
			log.Printf("We're %f seconds behind", s.Seconds()*-1)
		}
	}
	c.client.Close()
}
