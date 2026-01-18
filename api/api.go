package api

import (
	"context"
	"log/slog"

	"github.com/p4gefau1t/trojan-go/statistic"
)

type Handler func(ctx context.Context, auth statistic.Authenticator) error

var handlers = make(map[string]Handler)

func RegisterHandler(name string, handler Handler) {
	handlers[name] = handler
}

func RunService(ctx context.Context, name string, auth statistic.Authenticator) error {
	if h, ok := handlers[name]; ok {
		slog.Debug("api handler found", "name", name)
		return h(ctx, auth)
	}
	slog.Debug("api handler not found", "name", name)
	return nil
}
