package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log/slog"
	"strings"

	utls "github.com/refraction-networking/utls"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/tunnel"
	"github.com/p4gefau1t/trojan-go/tunnel/tls/fingerprint"
	"github.com/p4gefau1t/trojan-go/tunnel/transport"
)

// Client is a tls client
type Client struct {
	verify        bool
	sni           string
	ca            *x509.CertPool
	cipher        []uint16
	sessionTicket bool
	reuseSession  bool
	fingerprint   string
	helloID       utls.ClientHelloID
	keyLogger     io.WriteCloser
	underlay      tunnel.Client
}

func (c *Client) Close() error {
	if c.keyLogger != nil {
		c.keyLogger.Close()
	}
	return c.underlay.Close()
}

func (c *Client) DialPacket(tunnel.Tunnel) (tunnel.PacketConn, error) {
	panic("not supported")
}

func (c *Client) DialConn(_ *tunnel.Address, overlay tunnel.Tunnel) (tunnel.Conn, error) {
	conn, err := c.underlay.DialConn(nil, &Tunnel{})
	if err != nil {
		return nil, common.NewError("tls failed to dial conn").Base(err)
	}

	if c.fingerprint != "" {
		// utls fingerprint
		tlsConn := utls.UClient(conn, &utls.Config{
			RootCAs:            c.ca,
			ServerName:         c.sni,
			InsecureSkipVerify: !c.verify,
			KeyLogWriter:       c.keyLogger,
		}, c.helloID)
		if err := tlsConn.Handshake(); err != nil {
			return nil, common.NewError("tls failed to handshake with remote server").Base(err)
		}
		return &transport.Conn{
			Conn: tlsConn,
		}, nil
	}
	// golang default tls library
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify:     !c.verify,
		ServerName:             c.sni,
		RootCAs:                c.ca,
		KeyLogWriter:           c.keyLogger,
		CipherSuites:           c.cipher,
		SessionTicketsDisabled: !c.sessionTicket,
	})
	err = tlsConn.Handshake()
	if err != nil {
		return nil, common.NewError("tls failed to handshake with remote server").Base(err)
	}
	return &transport.Conn{
		Conn: tlsConn,
	}, nil
}

// NewClient creates a tls client
func NewClient(ctx context.Context, underlay tunnel.Client) (*Client, error) {
	cfg := config.FromContext(ctx, Name).(*Config)

	helloID := utls.ClientHelloID{}
	if cfg.TLS.Fingerprint != "" {
		switch cfg.TLS.Fingerprint {
		case "firefox":
			helloID = utls.HelloFirefox_Auto
		case "chrome":
			helloID = utls.HelloChrome_Auto
		case "ios":
			helloID = utls.HelloIOS_Auto
		default:
			return nil, common.NewError("invalid fingerprint " + cfg.TLS.Fingerprint)
		}
		slog.Info("tls fingerprint applied", "fingerprint", cfg.TLS.Fingerprint)
	}

	if cfg.TLS.SNI == "" {
		cfg.TLS.SNI = cfg.RemoteHost
		slog.Warn("tls sni is unspecified", "default", cfg.RemoteHost)
	}

	client := &Client{
		underlay:      underlay,
		verify:        cfg.TLS.Verify,
		sni:           cfg.TLS.SNI,
		cipher:        fingerprint.ParseCipher(strings.Split(cfg.TLS.Cipher, ":")),
		sessionTicket: cfg.TLS.ReuseSession,
		fingerprint:   cfg.TLS.Fingerprint,
		helloID:       helloID,
	}

	if cfg.TLS.CertPath != "" {
		caCertByte, err := ioutil.ReadFile(cfg.TLS.CertPath)
		if err != nil {
			return nil, common.NewError("failed to load cert file").Base(err)
		}
		client.ca = x509.NewCertPool()
		ok := client.ca.AppendCertsFromPEM(caCertByte)
		if !ok {
			slog.Warn("invalid cert list")
		}
		slog.Info("using custom cert")

		// print cert info
		pemCerts := caCertByte
		for len(pemCerts) > 0 {
			var block *pem.Block
			block, pemCerts = pem.Decode(pemCerts)
			if block == nil {
				break
			}
			if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
				continue
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				continue
			}
			slog.Debug("cert details", "issuer", cert.Issuer.String(), "subject", cert.Subject.String())
		}
	}

	if cfg.TLS.CertPath == "" {
		slog.Info("cert is unspecified; using default ca list")
	}

	slog.Debug("tls client created")
	return client, nil
}
