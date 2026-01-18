package redirector

import (
	"context"
	"io"
	"log/slog"
	"net"
	"reflect"
)

type Dial func(net.Addr) (net.Conn, error)

func defaultDial(addr net.Addr) (net.Conn, error) {
	return net.Dial("tcp", addr.String())
}

type Redirection struct {
	Dial
	RedirectTo  net.Addr
	InboundConn net.Conn
}

type Redirector struct {
	ctx             context.Context
	redirectionChan chan *Redirection
}

func (r *Redirector) Redirect(redirection *Redirection) {
	select {
	case r.redirectionChan <- redirection:
		slog.Debug("redirect request")
	case <-r.ctx.Done():
		slog.Debug("redirector exiting")
	}
}

func (r *Redirector) worker() {
	for {
		select {
		case redirection := <-r.redirectionChan:
			handle := func(redirection *Redirection) {
				if redirection.InboundConn == nil || reflect.ValueOf(redirection.InboundConn).IsNil() {
					slog.Error("nil inbound connection")
					return
				}
				defer redirection.InboundConn.Close()
				if redirection.RedirectTo == nil || reflect.ValueOf(redirection.RedirectTo).IsNil() {
					slog.Error("nil redirection address")
					return
				}
				if redirection.Dial == nil {
					redirection.Dial = defaultDial
				}
				slog.Warn("redirecting connection",
					"from", redirection.InboundConn.RemoteAddr().String(),
					"to", redirection.RedirectTo.String(),
				)
				outboundConn, err := redirection.Dial(redirection.RedirectTo)
				if err != nil {
					slog.Error("failed to redirect to target address",
						"target", redirection.RedirectTo.String(),
						"error", err,
					)
					return
				}
				defer outboundConn.Close()
				errChan := make(chan error, 2)
				copyConn := func(a, b net.Conn) {
					_, err := io.Copy(a, b)
					errChan <- err
				}
				go copyConn(outboundConn, redirection.InboundConn)
				go copyConn(redirection.InboundConn, outboundConn)
				select {
				case err := <-errChan:
					if err != nil {
						slog.Error("failed to redirect connection", "error", err)
					}
					slog.Info("redirection done")
				case <-r.ctx.Done():
					slog.Debug("redirector exiting")
					return
				}
			}
			go handle(redirection)
		case <-r.ctx.Done():
			slog.Debug("shutting down redirector")
			return
		}
	}
}

func NewRedirector(ctx context.Context) *Redirector {
	r := &Redirector{
		ctx:             ctx,
		redirectionChan: make(chan *Redirection, 64),
	}
	go r.worker()
	return r
}
