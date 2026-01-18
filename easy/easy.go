package easy

import (
	"encoding/json"
	"flag"
	"log/slog"
	"net"
	"os"
	"strconv"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/option"
	"github.com/p4gefau1t/trojan-go/proxy"
)

type easy struct {
	server   *bool
	client   *bool
	password *string
	local    *string
	remote   *string
	cert     *string
	key      *string
}

type ClientConfig struct {
	RunType    string   `json:"run_type"`
	LocalAddr  string   `json:"local_addr"`
	LocalPort  int      `json:"local_port"`
	RemoteAddr string   `json:"remote_addr"`
	RemotePort int      `json:"remote_port"`
	Password   []string `json:"password"`
}

type TLS struct {
	SNI  string `json:"sni"`
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type ServerConfig struct {
	RunType    string   `json:"run_type"`
	LocalAddr  string   `json:"local_addr"`
	LocalPort  int      `json:"local_port"`
	RemoteAddr string   `json:"remote_addr"`
	RemotePort int      `json:"remote_port"`
	Password   []string `json:"password"`
	TLS        `json:"ssl"`
}

func (o *easy) Name() string {
	return "easy"
}

func (o *easy) Handle() error {
	if !*o.server && !*o.client {
		return common.NewError("empty")
	}
	if *o.password == "" {
		slog.Error("empty password is not allowed")
		os.Exit(1)
	}
	slog.Info("easy mode enabled; config file not used")
	if *o.client {
		if *o.local == "" {
			slog.Warn("client local addr is unspecified", "default", "127.0.0.1:1080")
			*o.local = "127.0.0.1:1080"
		}
		localHost, localPortStr, err := net.SplitHostPort(*o.local)
		if err != nil {
			slog.Error("invalid local addr format", "addr", *o.local, "error", err)
			os.Exit(1)
		}
		remoteHost, remotePortStr, err := net.SplitHostPort(*o.remote)
		if err != nil {
			slog.Error("invalid remote addr format", "addr", *o.remote, "error", err)
			os.Exit(1)
		}
		localPort, err := strconv.Atoi(localPortStr)
		if err != nil {
			slog.Error("invalid local port", "port", localPortStr, "error", err)
			os.Exit(1)
		}
		remotePort, err := strconv.Atoi(remotePortStr)
		if err != nil {
			slog.Error("invalid remote port", "port", remotePortStr, "error", err)
			os.Exit(1)
		}
		clientConfig := ClientConfig{
			RunType:    "client",
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			Password: []string{
				*o.password,
			},
		}
		clientConfigJSON, err := json.Marshal(&clientConfig)
		common.Must(err)
		slog.Info("generated config", "config", string(clientConfigJSON))
		proxy, err := proxy.NewProxyFromConfigData(clientConfigJSON, true)
		if err != nil {
			slog.Error("failed to build proxy config", "error", err)
			os.Exit(1)
		}
		if err := proxy.Run(); err != nil {
			slog.Error("proxy run failed", "error", err)
			os.Exit(1)
		}
	} else if *o.server {
		if *o.remote == "" {
			slog.Warn("server remote addr is unspecified", "default", "127.0.0.1:80")
			*o.remote = "127.0.0.1:80"
		}
		if *o.local == "" {
			slog.Warn("server local addr is unspecified", "default", "0.0.0.0:443")
			*o.local = "0.0.0.0:443"
		}
		localHost, localPortStr, err := net.SplitHostPort(*o.local)
		if err != nil {
			slog.Error("invalid local addr format", "addr", *o.local, "error", err)
			os.Exit(1)
		}
		remoteHost, remotePortStr, err := net.SplitHostPort(*o.remote)
		if err != nil {
			slog.Error("invalid remote addr format", "addr", *o.remote, "error", err)
			os.Exit(1)
		}
		localPort, err := strconv.Atoi(localPortStr)
		if err != nil {
			slog.Error("invalid local port", "port", localPortStr, "error", err)
			os.Exit(1)
		}
		remotePort, err := strconv.Atoi(remotePortStr)
		if err != nil {
			slog.Error("invalid remote port", "port", remotePortStr, "error", err)
			os.Exit(1)
		}
		serverConfig := ServerConfig{
			RunType:    "server",
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			Password: []string{
				*o.password,
			},
			TLS: TLS{
				Cert: *o.cert,
				Key:  *o.key,
			},
		}
		serverConfigJSON, err := json.Marshal(&serverConfig)
		common.Must(err)
		slog.Info("generated config", "config", string(serverConfigJSON))
		proxy, err := proxy.NewProxyFromConfigData(serverConfigJSON, true)
		if err != nil {
			slog.Error("failed to build proxy config", "error", err)
			os.Exit(1)
		}
		if err := proxy.Run(); err != nil {
			slog.Error("proxy run failed", "error", err)
			os.Exit(1)
		}
	}
	return nil
}

func (o *easy) Priority() int {
	return 50
}

func init() {
	option.RegisterHandler(&easy{
		server:   flag.Bool("server", false, "Run a trojan-go server"),
		client:   flag.Bool("client", false, "Run a trojan-go client"),
		password: flag.String("password", "", "Password for authentication"),
		remote:   flag.String("remote", "", "Remote address, e.g. 127.0.0.1:12345"),
		local:    flag.String("local", "", "Local address, e.g. 127.0.0.1:12345"),
		key:      flag.String("key", "server.key", "Key of the server"),
		cert:     flag.String("cert", "server.crt", "Certificates of the server"),
	})
}
