package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/soracom/soratun"
	"github.com/spf13/cobra"
)

var method string
var data string
var headers []string

func curlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "curl <url>",
		Short: "Send a data to SORACOM platform via HTTP over SORACOM Arc",
		Long: `Send a data to SORACOM platform via HTTP over SORACOM Arc, without root user privilege. Less feature-rich curl alternative. Data can be passed by one of following methods:

* --data '{"message": "hello"}'
* --data '@filename'
* standard input i.e. cat data.json | soratun curl ...

If standard input is provided or GET is specified as request command, --data option will be ignored.

Set SORACOM_VERBOSE=1 to dump request and response detail.
`,
		Args:   cobra.ExactArgs(1),
		PreRun: initSoratun,
		Run: func(cmd *cobra.Command, args []string) {
			if Config.ArcSession == nil {
				log.Fatal("Failed to determine connection information. Please bootstrap or create a new session from the user console.")
			}

			arcClient, err := newArcHTTPClient(Config, args[0], headers)
			if err != nil {
				log.Fatalf("Failed to create a new client: %s", err)
			}

			body, err := getBody()
			if err != nil {
				log.Fatalf("Failed to get body to send: %s", err)
			}

			res, err := arcClient.Do(method, body)
			if err != nil {
				log.Fatalf("Failed to get response from %s: %v", args[0], err)
			}

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatalf("Failed to read response body from %s: %v", args[0], err)
			}
			fmt.Printf("%s", resBody)
		},
	}

	cmd.Flags().StringVarP(&method, "request", "X", "GET", "Specify request command to use (GET or POST)")
	cmd.Flags().StringVarP(&data, "data", "d", "", "HTTP POST data, '@' allowed")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Pass custom header(s)")
	return cmd
}

func newArcHTTPClient(config *soratun.Config, arg string, headers []string) (*soratun.ArcHTTPClient, error) {
	u, err := url.Parse(arg)
	if err != nil {
		return nil, err
	}

	arcHTTPClient, err := soratun.NewArcHTTPClient(config, u, headers)
	if err != nil {
		return nil, err
	}

	if v := os.Getenv("SORACOM_VERBOSE"); v != "" {
		arcHTTPClient.SetVerbose(true)
	}

	return arcHTTPClient, nil
}

func getBody() (io.Reader, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}

	if strings.HasPrefix(data, "@") {
		f := strings.TrimPrefix(data, "@")
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}

	return strings.NewReader(data), nil
}
