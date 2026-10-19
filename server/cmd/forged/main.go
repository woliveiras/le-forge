// Command forged is the LeForge control plane: API, engine, gateways and
// the web console in one binary. Run several for high availability; they
// coordinate through the shared store and broker.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/woliveiras/le-forge/server/internal/localinstall"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/woliveiras/le-forge/server/internal/api"
	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/config"
)

func main() {
	cfgPath := flag.String("config", "", "path to forge.yaml (or LEFORGE_CONFIG)")
	version := flag.Bool("version", false, "print version and exit")
	local := flag.Bool("local", false, "start an initialized native profile (no Docker required)")
	initLocal := flag.Bool("init-local", false, "initialize a new private native data directory")
	dataDir := flag.String("data-dir", "", "native data directory (default: OS user config directory/LeForge)")
	webDir := flag.String("web-dir", "", "native init: bundled UI directory (default: web next to executable)")
	port := flag.Int("port", 8787, "native init: loopback console port")
	flag.Parse()
	if *version {
		fmt.Println(api.Version)
		return
	}
	if *local || *initLocal {
		if *cfgPath != "" || (*local && *initLocal) {
			fmt.Fprintln(os.Stderr, "choose either --local or --init-local, without --config")
			os.Exit(2)
		}
		invalidStartFlag := false
		flag.Visit(func(f *flag.Flag) {
			if *local && (f.Name == "port" || f.Name == "web-dir") {
				invalidStartFlag = true
			}
		})
		if invalidStartFlag {
			fmt.Fprintln(os.Stderr, "--port and --web-dir apply only to --init-local; edit the native profile explicitly to change an existing installation")
			os.Exit(2)
		}
		root := *dataDir
		var err error
		if root == "" {
			root, err = localinstall.DefaultDir()
		} else {
			root, err = filepath.Abs(root)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if *initLocal {
			web := *webDir
			if web == "" {
				exe, e := os.Executable()
				if e != nil {
					fmt.Fprintln(os.Stderr, e)
					os.Exit(2)
				}
				web = filepath.Join(filepath.Dir(exe), "web")
			}
			web, err = filepath.Abs(web)
			if err == nil {
				err = localinstall.Init(root, web, *port)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "native initialization:", err)
				os.Exit(1)
			}
			fmt.Println("Native profile initialized at", root)
			fmt.Println("Read admin.token in that directory yourself and enter it on the login screen. Keep it private.")
			fmt.Println("Start with: forged --local --data-dir", root)
			return
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err = localinstall.Run(ctx, root); err != nil {
			fmt.Fprintln(os.Stderr, "native startup:", err)
			os.Exit(1)
		}
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
