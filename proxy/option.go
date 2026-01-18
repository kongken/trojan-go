package proxy

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/constant"
	"github.com/p4gefau1t/trojan-go/option"
)

type Option struct {
	path *string
}

func (o *Option) Name() string {
	return Name
}

func detectAndReadConfig(file string) ([]byte, bool, error) {
	isJSON := false
	switch {
	case strings.HasSuffix(file, ".json"):
		isJSON = true
	case strings.HasSuffix(file, ".yaml"), strings.HasSuffix(file, ".yml"):
		isJSON = false
	default:
		slog.Error("unsupported config format; use .yaml or .json", "path", file)
		os.Exit(1)
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, false, err
	}
	return data, isJSON, nil
}

func (o *Option) Handle() error {
	defaultConfigPath := []string{
		"config.json",
		"config.yml",
		"config.yaml",
	}

	isJSON := false
	var data []byte
	var err error

	switch *o.path {
	case "":
		slog.Warn("no config file specified; using default search paths")
		for _, file := range defaultConfigPath {
			slog.Warn("try to load config from default path", "path", file)
			data, isJSON, err = detectAndReadConfig(file)
			if err != nil {
				slog.Warn("failed to read config", "path", file, "error", err)
				continue
			}
			break
		}
	default:
		data, isJSON, err = detectAndReadConfig(*o.path)
		if err != nil {
			slog.Error("failed to read config", "path", *o.path, "error", err)
			os.Exit(1)
		}
	}

	if data != nil {
		slog.Info("trojan-go initializing", "version", constant.Version)
		proxy, err := NewProxyFromConfigData(data, isJSON)
		if err != nil {
			slog.Error("failed to build proxy from config", "error", err)
			os.Exit(1)
		}
		err = proxy.Run()
		if err != nil {
			slog.Error("proxy run failed", "error", err)
			os.Exit(1)
		}
	}

	slog.Error("no valid config")
	os.Exit(1)
	return nil
}

func (o *Option) Priority() int {
	return -1
}

func init() {
	option.RegisterHandler(&Option{
		path: flag.String("config", "", "Trojan-Go config filename (.yaml/.yml/.json)"),
	})
	option.RegisterHandler(&StdinOption{
		format:       flag.String("stdin-format", "disabled", "Read from standard input (yaml/json)"),
		suppressHint: flag.Bool("stdin-suppress-hint", false, "Suppress hint text"),
	})
}

type StdinOption struct {
	format       *string
	suppressHint *bool
}

func (o *StdinOption) Name() string {
	return Name + "_STDIN"
}

func (o *StdinOption) Handle() error {
	isJSON, e := o.isFormatJson()
	if e != nil {
		return e
	}

	if o.suppressHint == nil || !*o.suppressHint {
		fmt.Printf("Trojan-Go %s (%s/%s)\n", constant.Version, runtime.GOOS, runtime.GOARCH)
		if isJSON {
			fmt.Println("Reading JSON configuration from stdin.")
		} else {
			fmt.Println("Reading YAML configuration from stdin.")
		}
	}

	data, e := ioutil.ReadAll(bufio.NewReader(os.Stdin))
	if e != nil {
		slog.Error("failed to read from stdin", "error", e)
		os.Exit(1)
	}

	proxy, err := NewProxyFromConfigData(data, isJSON)
	if err != nil {
		slog.Error("failed to build proxy from stdin config", "error", err)
		os.Exit(1)
	}
	err = proxy.Run()
	if err != nil {
		slog.Error("proxy run failed", "error", err)
		os.Exit(1)
	}

	return nil
}

func (o *StdinOption) Priority() int {
	return 0
}

func (o *StdinOption) isFormatJson() (isJson bool, e error) {
	if o.format == nil {
		return false, common.NewError("format specifier is nil")
	}
	if *o.format == "disabled" {
		return false, common.NewError("reading from stdin is disabled")
	}
	return strings.ToLower(*o.format) == "json", nil
}
