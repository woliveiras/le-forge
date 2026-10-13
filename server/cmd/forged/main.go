// Command forged is the LeForge control plane: API, engine, gateways and
// the web console in one binary. Run several for high availability; they
// coordinate through the shared store and broker.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/woliveiras/le-forge/server/internal/api"
	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/config"
)

func main() {
	cfgPath := flag.String("config", "", "path to forge.yaml (or LEFORGE_CONFIG)")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Println(api.Version)
		return
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup:", err)
		os.Exit(1)
	}
	if err := a.Run(ctx); err != nil {
		a.Log.Error("exited", "err", err)
		os.Exit(1)
	}
}
