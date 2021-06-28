package soratun

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// ArcClient is short-lived TCP/UDP client to SORACOM platform, connecting via SORACOM Arc.
type ArcClient struct {
	network string
	conn    net.Conn
	verbose bool
}

type tunnel struct {
	device   *device.Device
	tunnel   tun.Device
	net      *netstack.Net
	resolver *net.Resolver
}

// NewArcClient returns ArcClient with given Config, network (tcp or udp), and address.
func NewArcClient(ctx context.Context, config *Config, network string, addr string) (*ArcClient, error) {
	t, err := createTunnel(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %v", err)
	}

	if network == "tcp" {
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve TCP address: %v", err)
		}

		c, err := t.net.DialContextTCP(ctx, tcpAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to dial %s: %v", tcpAddr, err)
		}

		return &ArcClient{
			network: network,
			conn:    c,
		}, nil
	}

	c, err := t.net.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", addr, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		err = c.SetDeadline(deadline)
		if err != nil {
			return nil, fmt.Errorf("failed to specify deadline %s: %v", deadline, err)
		}
	}

	return &ArcClient{
		network: network,
		conn:    c,
	}, nil
}

// Read reads bytes from the connection.
func (c *ArcClient) Read(buf *[]byte) (int, error) {
	l, err := c.conn.Read(*buf)
	if c.Verbose() {
		fmt.Printf("%d received\n", l)
	}
	return l, err
}

// Write writes bytes to the connection.
func (c *ArcClient) Write(body io.Reader) (int, error) {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return 0, err
	}

	l, err := c.conn.Write(b)
	if c.Verbose() {
		fmt.Printf("%d sent\n", l)
	}
	return l, err
}

// Close frees related resources.
func (c *ArcClient) Close() error {
	return c.conn.Close()
}

// SetVerbose sets if verbose output is enabled or not.
func (c *ArcClient) SetVerbose(v bool) {
	c.verbose = v
}

// Verbose returns if verbose output is enabled or not.
func (c *ArcClient) Verbose() bool {
	return c.verbose
}

func createTunnel(config *Config) (*tunnel, error) {
	t, n, err := netstack.CreateNetTUN(
		[]net.IP{config.ArcSession.ArcClientPeerIpAddress},
		[]net.IP{net.ParseIP("100.127.0.53"), net.ParseIP("100.127.1.53")},
		device.DefaultMTU,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a tunnel: %v", err)
	}

	dev := device.NewDevice(t, conn.NewDefaultBind(), &device.Logger{
		Verbosef: device.DiscardLogf,
		Errorf:   device.DiscardLogf,
	})

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
		return nil, fmt.Errorf("failed to configure device: %v", err)
	}

	if err := dev.Up(); err != nil {
		return nil, fmt.Errorf("failed to setup device: %v", err)
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
