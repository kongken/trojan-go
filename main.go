package main

import (
	"flag"
	"log/slog"
	"os"

	_ "github.com/p4gefau1t/trojan-go/component"
	"github.com/p4gefau1t/trojan-go/option"
)

func main() {
	flag.Parse()
	for {
		h, err := option.PopOptionHandler()
		if err != nil {
			slog.Error("invalid options", "error", err)
			os.Exit(1)
		}
		err = h.Handle()
		if err == nil {
			break
		}
	}
}
