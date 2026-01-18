package proxy

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strings"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/tunnel"
)

const Name = "PROXY"

const (
	MaxPacketSize = 1024 * 8
)

// Proxy relay connections and packets
type Proxy struct {
	sources []tunnel.Server
	sink    tunnel.Client
	ctx     context.Context
	cancel  context.CancelFunc
}

func (p *Proxy) Run() error {
	p.relayConnLoop()
	p.relayPacketLoop()
	<-p.ctx.Done()
	return nil
}

func (p *Proxy) Close() error {
	p.cancel()
	p.sink.Close()
	for _, source := range p.sources {
		source.Close()
	}
	return nil
}

func (p *Proxy) relayConnLoop() {
	for _, source := range p.sources {
		go func(source tunnel.Server) {
			for {
				inbound, err := source.AcceptConn(nil)
				if err != nil {
					select {
					case <-p.ctx.Done():
						slog.Debug("relay loop exiting")
						return
					default:
					}
					slog.Error("failed to accept connection", "error", err)
					continue
				}
				go func(inbound tunnel.Conn) {
					defer inbound.Close()
					outbound, err := p.sink.DialConn(inbound.Metadata().Address, nil)
					if err != nil {
						slog.Error("proxy failed to dial connection", "error", err)
						return
					}
					defer outbound.Close()
					errChan := make(chan error, 2)
					copyConn := func(a, b net.Conn) {
						_, err := io.Copy(a, b)
						errChan <- err
					}
					go copyConn(inbound, outbound)
					go copyConn(outbound, inbound)
					select {
					case err = <-errChan:
						if err != nil {
							slog.Error("connection relay error", "error", err)
						}
					case <-p.ctx.Done():
						slog.Debug("shutting down connection relay")
						return
					}
					slog.Debug("connection relay ended")
				}(inbound)
			}
		}(source)
	}
}

func (p *Proxy) relayPacketLoop() {
	for _, source := range p.sources {
		go func(source tunnel.Server) {
			for {
				inbound, err := source.AcceptPacket(nil)
				if err != nil {
					select {
					case <-p.ctx.Done():
						slog.Debug("relay loop exiting")
						return
					default:
					}
					slog.Error("failed to accept packet", "error", err)
					continue
				}
				go func(inbound tunnel.PacketConn) {
					defer inbound.Close()
					outbound, err := p.sink.DialPacket(nil)
					if err != nil {
						slog.Error("proxy failed to dial packet", "error", err)
						return
					}
					defer outbound.Close()
					errChan := make(chan error, 2)
					copyPacket := func(a, b tunnel.PacketConn) {
						for {
							buf := make([]byte, MaxPacketSize)
							n, metadata, err := a.ReadWithMetadata(buf)
							if err != nil {
								errChan <- err
								return
							}
							if n == 0 {
								errChan <- nil
								return
							}
							_, err = b.WriteWithMetadata(buf[:n], metadata)
							if err != nil {
								errChan <- err
								return
							}
						}
					}
					go copyPacket(inbound, outbound)
					go copyPacket(outbound, inbound)
					select {
					case err = <-errChan:
						if err != nil {
							slog.Error("packet relay error", "error", err)
						}
					case <-p.ctx.Done():
						slog.Debug("shutting down packet relay")
					}
					slog.Debug("packet relay ended")
				}(inbound)
			}
		}(source)
	}
}

func NewProxy(ctx context.Context, cancel context.CancelFunc, sources []tunnel.Server, sink tunnel.Client) *Proxy {
	return &Proxy{
		sources: sources,
		sink:    sink,
		ctx:     ctx,
		cancel:  cancel,
	}
}

type Creator func(ctx context.Context) (*Proxy, error)

var creators = make(map[string]Creator)

func RegisterProxyCreator(name string, creator Creator) {
	creators[name] = creator
}

func NewProxyFromConfigData(data []byte, isJSON bool) (*Proxy, error) {
	// create a unique context for each proxy instance to avoid duplicated authenticator
	ctx := context.WithValue(context.Background(), Name+"_ID", rand.Int())
	var err error
	if isJSON {
		ctx, err = config.WithJSONConfig(ctx, data)
		if err != nil {
			return nil, err
		}
	} else {
		ctx, err = config.WithYAMLConfig(ctx, data)
		if err != nil {
			return nil, err
		}
	}
	cfg := config.FromContext(ctx, Name).(*Config)
	create, ok := creators[strings.ToUpper(cfg.RunType)]
	if !ok {
		return nil, common.NewError("unknown proxy type: " + cfg.RunType)
	}

	output := io.Writer(os.Stdout)
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, common.NewError("failed to open log file").Base(err)
		}
		output = file
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: logLevelFromConfig(cfg.LogLevel),
	})
	slog.SetDefault(slog.New(handler))

	return create(ctx)
}

func logLevelFromConfig(level int) slog.Level {
	switch level {
	case 0:
		return slog.LevelDebug
	case 1:
		return slog.LevelInfo
	case 2:
		return slog.LevelWarn
	case 3:
		return slog.LevelError
	case 4:
		return slog.LevelError
	case 5:
		return slog.Level(1000)
	default:
		return slog.LevelInfo
	}
}
