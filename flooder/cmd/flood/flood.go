package flood

import (
	"github.com/DataDog/datadog-go/v5/flooder/pkg/flood"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// floodCmd represents the base command when called without any subcommands
var floodCmd = &cobra.Command{
	Use:   "v5",
	Short: "Sends a lot of statsd points.",
	Run: func(cmd *cobra.Command, args []string) {
		flood.Flood(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := floodCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	floodCmd.Flags().StringP("address", "", "127.0.0.1:8125", "Address of the statsd server")
	floodCmd.Flags().StringP("telemetry-address", "", "", "Address of the telemetry server")
	floodCmd.Flags().BoolP("client-side-aggregation", "", false, "Enable client-side aggregation")
	floodCmd.Flags().BoolP("extended-client-side-aggregation", "", false, "Enable extended client-side aggregation")
	floodCmd.Flags().BoolP("channel-mode", "", false, "Enable channel mode")
	floodCmd.Flags().IntP("channel-mode-buffer-size", "", 4096, "Set channel mode buffer size")
	floodCmd.Flags().IntP("sender-queue-size", "", 512, "Set sender queue size")
	floodCmd.Flags().DurationP("buffer-flush-interval", "", time.Duration(4)*time.Second, "Set buffer flush interval")
	floodCmd.Flags().DurationP("write-timeout", "", time.Duration(100)*time.Millisecond, "Set write timeout")
	floodCmd.Flags().StringSliceP("tags", "", []string{}, "Set tags")
	floodCmd.Flags().IntP("points-per-10seconds", "", 100000, "Set points per 10 seconds")
	floodCmd.Flags().BoolP("send-at-start-of-bucket", "", false, "Send all the points at the start of the 10 sec time bucket.")
	floodCmd.Flags().BoolP("verbose", "", false, "Enable verbose mode")
	floodCmd.Flags().BoolP("send-with-timestamp", "", false, "Send the timestamp of the time the point was sent")
}
