package soratun

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// ArcHTTPClient is short-lived HTTP client to SORACOM platform, connecting via SORACOM Arc.
type ArcHTTPClient struct {
	httpClient *http.Client
	url        *url.URL
	headers    []string
	verbose    bool
}

type params struct {
	body    io.Reader
	method  string
	headers []string
}

// NewArcHTTPClient returns ArcHTTPClient with given Config, target url, and HTTP headers.
func NewArcHTTPClient(config *Config, url *url.URL, headers []string) (*ArcHTTPClient, error) {
	t, err := createTunnel(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %v", err)
	}

	return &ArcHTTPClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: t.DialContext,
			},
		},
		url:     url,
		headers: headers,
	}, nil
}

// Do sends HTTP request with given body and returns response.
func (c *ArcHTTPClient) Do(method string, body io.Reader) (*http.Response, error) {
	if !(method == http.MethodGet || method == http.MethodPost) {
		return nil, fmt.Errorf("unsupported HTTP method %s. It should be GET, POST", method)
	}

	req, err := c.makeRequest(&params{
		body:    body,
		method:  method,
		headers: c.headers,
	})
	if err != nil {
		return nil, err
	}

	res, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// SetVerbose sets if verbose output is enabled or not.
func (c *ArcHTTPClient) SetVerbose(v bool) {
	c.verbose = v
}

// Verbose returns if verbose output is enabled or not.
func (c *ArcHTTPClient) Verbose() bool {
	return c.verbose
}

// Close frees tunnel related resources.
func (t *tunnel) Close() error {
	if t.device != nil {
		t.device.Close()
	}

	t.device, t.net, t.tunnel = nil, nil, nil
	return nil
}

// DialContext exposes internal net.DialContext for consumption.
func (t *tunnel) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return t.net.DialContext(ctx, network, addr)
}

// Resolver returns internal resolver for the tunnel. Since we use gVisor as TCP stack we have to implement DNS resolver by ourselves.
func (t *tunnel) Resolver() *net.Resolver {
	return t.resolver
}

func (c *ArcHTTPClient) makeRequest(params *params) (*http.Request, error) {
	req, err := http.NewRequest(params.method,
		fmt.Sprintf("%s://%s:%s/%s", c.url.Scheme, c.url.Hostname(), c.url.Port(), strings.TrimPrefix(c.url.Path, "/")),
		params.body)
	if err != nil {
		return nil, err
	}

	for _, h := range params.headers {
		header := strings.Split(h, ":")
		if len(header) == 2 {
			req.Header.Set(header[0], header[1])
		}
	}

	return req, nil
}

func (c *ArcHTTPClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.Verbose() {
		fmt.Fprintln(os.Stderr, "--- Request dump ---------------------------------")
		r, _ := httputil.DumpRequest(req, true)
		fmt.Fprintf(os.Stderr, "%s\n", r)
		fmt.Fprintln(os.Stderr, "--- End of request dump --------------------------")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if c.Verbose() && res != nil {
		fmt.Fprintln(os.Stderr, "--- Response dump --------------------------------")
		r, _ := httputil.DumpResponse(res, true)
		fmt.Fprintf(os.Stderr, "%s\n", r)
		fmt.Fprintln(os.Stderr, "--- End of response dump -------------------------")
	}

	if res.StatusCode >= http.StatusBadRequest {
		defer func() {
			err := res.Body.Close()
			if err != nil {
				fmt.Println("failed to close response", err)
			}
		}()
		r, _ := ioutil.ReadAll(res.Body)
		return res, fmt.Errorf("%s: %s %s: %s", res.Status, req.Method, req.URL, r)
	}

	return res, nil
}
