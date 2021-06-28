package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/soracom/soratun"
	"github.com/spf13/cobra"
)

var udp bool
var wait int

func ncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nc <host> <port>",
		Short: "Send a data to SORACOM platform via TCP/UDP over SORACOM Arc",
		Long: `Send a data to SORACOM platform via TCP/UDP over SORACOM Arc, without root user privilege. Less feature-rich netcat alternative. Data can be passed standard input i.e. cat data.json | soratun nc ...

Set SORACOM_VERBOSE=1 to dump request and response detail.
`,
		Args:   cobra.ExactArgs(2),
		PreRun: initSoratun,
		Run: func(cmd *cobra.Command, args []string) {
			if Config.ArcSession == nil {
				log.Fatal("Failed to determine connection information. Please bootstrap or create a new session from the user console.")
			}

			arcClient, err := newArcClient(Config, args[0], args[1])
			if err != nil {
				log.Fatalf("Failed to create a new client: %s", err)
			}
			defer func() {
				err := arcClient.Close()
				if err != nil {
					log.Printf("failed to close SORACOM Arc Client: %v ", err)
				}
			}()

			body, err := getBody()
			if err != nil {
				log.Fatalf("Failed to get body to send: %s", err)
			}

			_, err = arcClient.Write(body)
			if err != nil {
				log.Fatalf("Failed to get response from %s: %v", args[0], err)
			}

			buf := make([]byte, Config.Mtu)
			_, err = arcClient.Read(&buf)
			if err != nil {
				log.Fatalf("Failed to read response body from %s: %v", args[0], err)
			}

			fmt.Printf("%s", string(buf))
		},
	}

	cmd.Flags().BoolVarP(&udp, "udp", "u", false, "Use UDP instead of the default option of TCP")
	cmd.Flags().IntVarP(&wait, "wait", "w", 0, "If a connection and stdin are idle for more than timeout seconds, the connection is closed.")
	return cmd
}

func newArcClient(config *soratun.Config, host, port string) (*soratun.ArcClient, error) {
	var ctx context.Context
	var cancel context.CancelFunc
	if wait > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	network := "udp"
	if !udp {
		network = "tcp"
	}

	arcClient, err := soratun.NewArcClient(ctx, config, network, fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, err
	}

	return arcClient, nil
}
