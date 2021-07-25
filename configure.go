package http2

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	// ErrServerSupport indicates whether the server supports HTTP/2 or not.
	ErrServerSupport = errors.New("server doesn't support HTTP/2")
)

type ClientOpts struct {
	// OnRTT is assigned to every client after creation, and the handler
	// will be called after every RTT measurement (after receiving a PONG mesage).
	OnRTT func(time.Duration)
}

func configureDialer(d *Dialer) {
	if d.TLSConfig == nil {
		d.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		}
	}

	tlsConfig := d.TLSConfig

	emptyServerName := len(tlsConfig.ServerName) == 0
	if emptyServerName {
		host, _, err := net.SplitHostPort(d.Addr)
		if err != nil {
			host = d.Addr
		}

		tlsConfig.ServerName = host
	}

	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
}

// ConfigureClient configures the fasthttp.HostClient to run over HTTP/2.
func ConfigureClient(c *fasthttp.HostClient, opts ClientOpts) error {
	emptyServerName := c.TLSConfig != nil && len(c.TLSConfig.ServerName) == 0

	d := &Dialer{
		Addr:      c.Addr,
		TLSConfig: c.TLSConfig,
	}

	c2, err := d.Dial()
	if err != nil {
		if err == ErrServerSupport && c.TLSConfig != nil { // remove added config settings
			for i := range c.TLSConfig.NextProtos {
				if c.TLSConfig.NextProtos[i] == "h2" {
					c.TLSConfig.NextProtos = append(c.TLSConfig.NextProtos[:i], c.TLSConfig.NextProtos[i+1:]...)
				}
			}

			if emptyServerName {
				c.TLSConfig.ServerName = ""
			}
		}

		return err
	}
	defer c2.Close()

	c.IsTLS = true
	c.TLSConfig = d.TLSConfig

	cl := createClient(d)
	cl.onRTT = opts.OnRTT

	cl.conns.Init()

	c.Transport = cl.Do

	return nil
}

// ConfigureServer configures the fasthttp server to handle
// HTTP/2 connections. The HTTP/2 connection can be only
// established if the fasthttp server is using TLS.
//
// Future implementations may support HTTP/2 through plain TCP.
func ConfigureServer(s *fasthttp.Server) *Server {
	s2 := &Server{
		s: s,
	}

	s.NextProto(H2TLSProto, s2.ServeConn)

	return s2
}

// ConfigureServerAndConfig configures the fasthttp server to handle HTTP/2 connections
// and your own tlsConfig file. If you are NOT using your own tls config, you may want to use ConfigureServer.
func ConfigureServerAndConfig(s *fasthttp.Server, tlsConfig *tls.Config) *Server {
	s2 := &Server{
		s: s,
	}

	s.NextProto(H2TLSProto, s2.ServeConn)
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, H2TLSProto)

	return s2
}

var ErrNotAvailableStreams = errors.New("ran out of available streams")