package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/soracom/soratun"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/device"
)

var (
	port    uint16
	address string
	headers []string
)

func soraProxyUpCmd() *cobra.Command {
	c := &cobra.Command{
		Use:    "up",
		Short:  "Setup proxy service for SORACOM Unified Endpoint",
		Args:   cobra.NoArgs,
		PreRun: initSoratun,
		Run: func(cmd *cobra.Command, args []string) {
			if Config.ArcSession == nil {
				log.Fatal("Failed to determine connection information. Please bootstrap or create a new session from the user console.")
			}
			logger := device.NewLogger(Config.LogLevel, "(soraproxy/proxy) ")
			logger.Verbosef("create a proxy tunnel")

			client, err := newArcUnifiedEndpointClient(Config, headers)
			if err != nil {
				log.Fatalf("Failed to create a new client: %s", err)
			}

			logger.Verbosef("setup proxy server for Unified Endpoint")
			errCh := make(chan error)
			sigCh := make(chan os.Signal, 1)

			go func() {
				r := mux.NewRouter()
				r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // NotFoundHandler as wild card
					logger.Verbosef("route %s %s to Unified Endpoint", r.Method, r.URL)

					var body []byte
					switch r.Method {
					case http.MethodPost:
						resp, err := client.Do(r.Method, r.URL.Path, r.Body)
						if err != nil {
							logger.Errorf("failed to get response from Unified Endpoint: %v", err)
							_, _ = io.WriteString(w, fmt.Sprintf("failed to get response from Unified Endpoint: %v", err))
						}
						defer func() {
							err := resp.Body.Close()
							if err != nil {
								fmt.Println("failed to close response", err)
							}
						}()

						body, err = io.ReadAll(resp.Body)
						if err != nil {
							logger.Errorf("failed to read response from Unified Endpoint: %v", err)
							_, _ = io.WriteString(w, fmt.Sprintf("failed to read response from Unified Endpoint: %v", err))
						}
					default:
						logger.Errorf("unsupported HTTP method: %s", r.Method)
						_, _ = io.WriteString(w, fmt.Sprintf("unsupported HTTP method: %s", r.Method))
					}

					_, err = io.WriteString(w, string(body))
				})

				if err = http.ListenAndServe(fmt.Sprintf("%s:%d", address, port), r); err != nil {
					errCh <- err
				}
			}()
			logger.Verbosef("proxy server for Unified Endpoint started at %s:%d", address, port)

			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			select {
			case err, ok := <-errCh:
				if ok {
					logger.Errorf("proxy server error: ", err)
				}
			case sig := <-sigCh:
				logger.Verbosef("received signal %s", sig)
				logger.Verbosef("proxy shut down")
			}
		},
	}

	c.Flags().Uint16VarP(&port, "port", "p", 8888, "local port number")
	c.Flags().StringVarP(&address, "address", "a", "127.0.0.1", "IP address to bind a proxy")
	c.Flags().StringArrayVarP(&headers, "header", "H", nil, "Pass custom header(s)")
	return c
}

func newArcUnifiedEndpointClient(config *soratun.Config, headers []string) (*soratun.ArcUnifiedEndpointHTTPClient, error) {
	c, err := soratun.NewArcUnifiedEndpointHTTPClient(config, headers)
	if err != nil {
		return nil, err
	}

	if v := os.Getenv("SORACOM_VERBOSE"); v != "" {
		c.SetVerbose(true)
	}

	return c, nil
}
