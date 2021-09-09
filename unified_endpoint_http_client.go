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

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// ArcUnifiedEndpointHTTPClient is short-lived HTTP client to SORACOM platform, connecting via SORACOM Arc.
type ArcUnifiedEndpointHTTPClient struct {
	httpClient *http.Client
	endpoint   *url.URL
	headers    []string
	verbose    bool
}

const UnifiedEndpointHostname = "100.127.69.42"
const UnifiedEndpointPort = 80

type tunnel struct {
	device   *device.Device
	tunnel   tun.Device
	net      *netstack.Net
	resolver *net.Resolver
}

type params struct {
	path    string
	body    io.Reader
	method  string
	headers []string
}

// NewArcUnifiedEndpointHTTPClient returns ArcUnifiedEndpointHTTPClient with given Config and HTTP headers.
func NewArcUnifiedEndpointHTTPClient(config *Config, headers []string) (*ArcUnifiedEndpointHTTPClient, error) {
	t, err := createTunnel(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %w", err)
	}
	endpoint, _ := url.Parse(fmt.Sprintf("http://%s:%d", UnifiedEndpointHostname, UnifiedEndpointPort))

	return &ArcUnifiedEndpointHTTPClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: t.DialContext,
			},
		},
		endpoint: endpoint,
		headers:  headers,
	}, nil
}

// Do sends HTTP request with given body and returns response.
func (c *ArcUnifiedEndpointHTTPClient) Do(method, path string, body io.Reader) (*http.Response, error) {
	if !(method == http.MethodGet || method == http.MethodPost) {
		return nil, fmt.Errorf("unsupported HTTP method %s. It should be GET or POST", method)
	}

	req, err := c.makeRequest(&params{
		path:    strings.TrimPrefix(path, "/"),
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
func (c *ArcUnifiedEndpointHTTPClient) SetVerbose(v bool) {
	c.verbose = v
}

// Verbose returns if verbose output is enabled or not.
func (c *ArcUnifiedEndpointHTTPClient) Verbose() bool {
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

func (c *ArcUnifiedEndpointHTTPClient) makeRequest(params *params) (*http.Request, error) {
	req, err := http.NewRequest(params.method,
		fmt.Sprintf("%s://%s:%s/%s",
			c.endpoint.Scheme,
			c.endpoint.Hostname(),
			c.endpoint.Port(),
			params.path,
		),
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

func (c *ArcUnifiedEndpointHTTPClient) doRequest(req *http.Request) (*http.Response, error) {
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

func createTunnel(config *Config) (*tunnel, error) {
	t, n, err := netstack.CreateNetTUN(
		[]net.IP{config.ArcSession.ArcClientPeerIpAddress},
		[]net.IP{net.ParseIP("100.127.0.53"), net.ParseIP("100.127.1.53")},
		config.Mtu,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a tunnel: %w", err)
	}

	logger := device.NewLogger(
		config.LogLevel,
		"(soraproxy/proxy/tunnel) ",
	)

	dev := device.NewDevice(t, conn.NewDefaultBind(), logger)

	conf := fmt.Sprintf(`private_key=%s
public_key=%s
endpoint=%s:%d
allowed_ip=0.0.0.0/0
`,
		config.PrivateKey.AsHexString(),
		config.ArcSession.ArcServerPeerPublicKey.AsHexString(),
		config.ArcSession.ArcServerEndpoint.IP,
		config.ArcSession.ArcServerEndpoint.Port)

	if err := dev.IpcSet(conf); err != nil {
		return nil, fmt.Errorf("failed to configure device: %w", err)
	}

	if err := dev.Up(); err != nil {
		return nil, fmt.Errorf("failed to setup device: %w", err)
	}

	return &tunnel{
		device: dev,
		tunnel: t,
		net:    n,
		resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return n.DialContext(ctx, network, "100.127.0.53:53")
			},
		},
	}, nil
}
