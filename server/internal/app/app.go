// Package app wires configuration to adapters and starts the control plane.
// It is the composition root: the only package that knows which concrete
// storage, queue, provider, runtime and sandbox implementations exist.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/woliveiras/le-forge/server/internal/api"
	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/environment"
	"github.com/woliveiras/le-forge/server/internal/events"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"github.com/woliveiras/le-forge/server/internal/hitl"
	"github.com/woliveiras/le-forge/server/internal/llm"
	_ "github.com/woliveiras/le-forge/server/internal/llm/providers"
	"github.com/woliveiras/le-forge/server/internal/llmconfig"
	"github.com/woliveiras/le-forge/server/internal/observability"
	"github.com/woliveiras/le-forge/server/internal/queue"
	"github.com/woliveiras/le-forge/server/internal/queue/brokers"
	"github.com/woliveiras/le-forge/server/internal/runtime"
	"github.com/woliveiras/le-forge/server/internal/runtime/echo"
	"github.com/woliveiras/le-forge/server/internal/runtime/external"
	"github.com/woliveiras/le-forge/server/internal/runtime/native"
	"github.com/woliveiras/le-forge/server/internal/sandboxcapacity"
	"github.com/woliveiras/le-forge/server/internal/secrets"
	"github.com/woliveiras/le-forge/server/internal/storage"
	"github.com/woliveiras/le-forge/server/internal/storage/stores"
	"github.com/woliveiras/le-forge/server/internal/tools"
	"github.com/woliveiras/le-forge/server/internal/workspace"
)

// App is a running control plane.
type App struct {
	Config    *config.Config
	Log       *slog.Logger
	Store     storage.Store
	Broker    queue.Broker
	Bus       *events.Bus
	Approvals *hitl.Service
	Models    *llm.Registry
	Providers *llmconfig.Manager
	Tools     *tools.Registry
	Gateway   *tools.Gateway
	Engine    *engine.Engine
	// Environments builds project environments on the sandbox daemon.
	Environments *environment.Manager
	Proposals    *environment.Proposals
	API          *api.Server

	shutdownObs func(context.Context) error
}

// New builds every component. Log may be nil (observability.Setup decides).
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	a := &App{Config: cfg}
	log, shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName: cfg.Observability.ServiceName, ServiceVersion: api.Version, Instance: cfg.Instance,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint, SampleRatio: cfg.Observability.TraceSampleRatio,
		LogLevel: cfg.Observability.LogLevel, LogFormat: cfg.Observability.LogFormat,
	})
	if err != nil {
		// Telemetry must never keep the platform from starting.
		fmt.Fprintln(os.Stderr, "observability disabled:", err)
		log, shutdown, _ = observability.Setup(ctx, observability.Config{
			ServiceName: cfg.Observability.ServiceName, Instance: cfg.Instance,
			LogLevel: cfg.Observability.LogLevel, LogFormat: cfg.Observability.LogFormat,
		})
	}
	a.Log, a.shutdownObs = log, shutdown
	slog.SetDefault(log)

	a.Store, err = stores.Open(ctx, stores.Config{Type: cfg.Storage.Type, DSN: cfg.Storage.DSN, Dir: cfg.Storage.Dir})
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	a.Broker, err = openBroker(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("queue: %w", err)
	}
	a.Bus = events.NewBus(a.Store, log)
	a.Approvals = hitl.New(a.Store, a.Bus, log)

	a.Models, err = llm.NewRegistry(cfg.LLM.Providers, cfg.LLM.Default)
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	box, keySource, err := loadSecrets(cfg.Secrets)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}
	switch keySource {
	case secrets.SourceNone:
		log.Warn("no installation encryption key: credentials cannot be stored from the console (set LEFORGE_SECRET_KEY or secrets.keyFile)")
	case secrets.SourceGenerated:
		log.Warn("generated a new installation encryption key; back it up - stored credentials cannot be decrypted without it", "file", cfg.Secrets.KeyFile, "key_id", box.PrimaryKeyID())
	default:
		log.Info("installation encryption key loaded", "source", keySource, "key_id", box.PrimaryKeyID())
	}
	a.Providers, err = llmconfig.New(llmconfig.Options{
		Store: a.Store, Box: box, KeySource: keySource, Registry: a.Models,
		Static: cfg.LLM.Providers, StaticDefault: cfg.LLM.Default, Bus: a.Bus, Log: log,
		InUse: a.providerUsers,
	})
	if err != nil {
		return nil, err
	}
	if err := a.Providers.Load(ctx); err != nil {
		return nil, fmt.Errorf("llm providers: %w", err)
	}
	a.Tools = tools.Builtin(tools.BuiltinConfig{AllowPrivateNetworks: cfg.Tools.AllowPrivateNetworks})
	a.Gateway = tools.NewGateway(a.Tools, a.Approvals, a.Bus)

	ws, err := workspace.NewManager(cfg.Workspaces.Root, cfg.Workspaces.HostRoot, cfg.Workspaces.AllowAbsolute)
	if err != nil {
		return nil, err
	}

	secret := ""
	if cfg.Runtimes.SharedSecretEnv != "" {
		secret = os.Getenv(cfg.Runtimes.SharedSecretEnv)
	}
	runtimes := map[domain.RuntimeKind]runtime.Runtime{
		domain.RuntimeNative: native.New(),
		domain.RuntimeEcho:   echo.New(),
		domain.RuntimeExternal: external.New(external.Options{
			SharedSecret: secret, DockerBin: cfg.Isolation.Docker.Bin,
			DockerNetwork: os.Getenv("LEFORGE_RUNTIME_NETWORK"), CallbackURL: cfg.Server.InternalURL,
		}),
	}

	a.Environments, err = environment.New(environment.Options{
		Store: a.Store, Bus: a.Bus, Workspaces: ws, Instance: cfg.Instance, Log: log,
		DockerBin: cfg.Isolation.Docker.Bin, DockerHost: cfg.Isolation.Docker.Host,
		Capacity: capacityManager(cfg, a.Store),
	})
	if err != nil {
		return nil, err
	}
	a.Proposals = &environment.Proposals{Store: a.Store, Approvals: a.Approvals, Models: a.Models, Manager: a.Environments, Directory: ws.GraphDir}
	a.Environments.SetProposalValidator(a.Proposals.ValidateAccepted)
	a.Engine, err = engine.New(engine.Options{
		Instance: cfg.Instance, Store: a.Store, Broker: a.Broker, Bus: a.Bus, Approvals: a.Approvals,
		Models: a.Models, Tools: toolGateway{a.Tools, a.Gateway}, Runtimes: runtimes, Workspaces: ws,
		Sandboxes: sandboxFactory(cfg, a.Environments, a.Store, a.Bus), Environments: a.Environments, EnvironmentProposals: a.Proposals,
		CallbackURL: cfg.Server.InternalURL, Log: log,
		LeaseTTL: cfg.Engine.LeaseTTL, ReconcileInterval: cfg.Engine.ReconcileInterval,
		RetryBackoff: cfg.Engine.RetryBackoff, MaxConcurrentTurns: cfg.Engine.MaxConcurrentTurns,
	})
	if err != nil {
		return nil, err
	}
	a.Proposals.Save = func(ctx context.Context, g *domain.Graph, version int, actor string) error {
		_, _, err := a.Engine.SaveGraph(ctx, g, version, actor)
		return err
	}
	a.Proposals.Start = func(ctx context.Context, id, actor string) error {
		return a.Engine.GraphAction(ctx, id, "start", actor)
	}
	auth, err := api.NewAuthenticator(cfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	if cfg.Auth.Disabled {
		log.Warn("API authentication is DISABLED; never expose this instance beyond localhost")
	}
	a.API = api.New(api.Deps{
		Config: cfg, Engine: a.Engine, Store: a.Store, Broker: a.Broker, Bus: a.Bus, Approvals: a.Approvals,
		Models: a.Models, Providers: a.Providers, Tools: a.Tools, Workspaces: ws, Auth: auth, Log: log,
		Environments: a.Environments, Proposals: a.Proposals,
	})
	return a, nil
}

func loadSecrets(c config.Secrets) (*secrets.Box, secrets.KeySource, error) {
	opts := secrets.Options{KeyFile: c.KeyFile, Generate: c.GenerateKeyFile}
	if c.KeyEnv != "" {
		opts.Key = os.Getenv(c.KeyEnv)
	}
	if c.PreviousKeysEnv != "" {
		opts.Previous = strings.Split(os.Getenv(c.PreviousKeysEnv), ",")
	}
	return secrets.Load(opts)
}

// providerUsers lists graph nodes that select a provider by name, so it is
// not deleted from under them.
func (a *App) providerUsers(ctx context.Context, provider string) ([]string, error) {
	gs, err := a.Store.ListGraphs(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range gs {
		if g.Spec.Defaults.Model.Provider == provider {
			out = append(out, fmt.Sprintf("graph %q (defaults)", g.Metadata.ID))
		}
		for _, n := range g.Spec.Nodes {
			if n.Model.Provider == provider {
				out = append(out, fmt.Sprintf("graph %q node %q", g.Metadata.ID, n.ID))
			}
		}
	}
	return out, nil
}

// refreshProviders applies provider changes made through other instances.
func (a *App) refreshProviders(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.Providers.Refresh(ctx); err != nil && ctx.Err() == nil {
				a.Log.Warn("refresh model providers", "err", err)
			}
		}
	}
}

type toolGateway struct {
	reg *tools.Registry
	gw  *tools.Gateway
}

func (t toolGateway) Invoke(ctx context.Context, p *tools.Principal, name string, args json.RawMessage) (*tools.Result, error) {
	return t.gw.Invoke(ctx, p, name, args)
}

func (t toolGateway) Schemas(p *tools.Principal) []llm.ToolSchema { return t.reg.Schemas(p) }

func (t toolGateway) NeedsApproval(p *tools.Principal, name string) bool {
	tool, ok := t.reg.Get(name)
	if !ok {
		return false
	}
	g, err := tools.Authorize(p, tool)
	return err == nil && g.RequiresApproval
}

func openBroker(ctx context.Context, cfg *config.Config) (queue.Broker, error) {
	var last error
	// Brokers in a compose stack may still be starting; retry briefly.
	for i := 0; i < 30; i++ {
		b, err := brokers.Open(ctx, brokers.Config{Type: cfg.Queue.Type, URL: cfg.Queue.URL, Dir: cfg.Queue.Dir, Options: cfg.Queue.Options})
		if err == nil {
			return b, nil
		}
		last = err
		if cfg.Queue.Type == "" || cfg.Queue.Type == "fs" || cfg.Queue.Type == "memory" {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, last
}

func capacityManager(cfg *config.Config, store storage.Store) *sandboxcapacity.Manager {
	host := cfg.Isolation.Docker.Host
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	scope := fmt.Sprintf("%x", sha256.Sum256([]byte(host)))
	return &sandboxcapacity.Manager{Store: store, Policy: cfg.Isolation.Docker.Capacity, Scope: scope, Notify: func(name, state string) {
		slog.Info("sandbox capacity", "sandbox", name, "state", state)
	}}
}

// sandboxFactory picks the command sandbox per node isolation mode.
//
// Docker-isolated nodes without an image of their own use the graph's
// environment when it defines one (its image, caches and run network);
// otherwise the installation's default sandbox image.
func sandboxFactory(cfg *config.Config, envs *environment.Manager, store storage.Store, bus *events.Bus) engine.SandboxFactory {
	capacity := capacityManager(cfg, store)

	return func(ctx context.Context, s engine.SandboxSpec) (tools.Sandbox, error) {
		switch s.Node.Isolation.Mode {
		case domain.IsolationDocker:
			img := s.Node.Isolation.Image
			net := s.Node.Isolation.Network
			var volumes []string
			if engine.UsesEnvironment(s.Graph, s.Node) {
				env, err := envs.SandboxFor(ctx, s.Graph)
				if err != nil {
					return nil, err
				}
				img, volumes = env.Image, env.Volumes
				if net == "" {
					net = env.Network
				}
			}
			if img == "" {
				img = cfg.Isolation.Docker.Image
			}
			if net == "" {
				net = cfg.Isolation.Docker.Network
			}
			budget := *capacity
			budget.Notify = func(name, state string) {
				slog.Info("sandbox capacity", "graph", s.Graph.Metadata.ID, "node", s.Node.ID, "sandbox", name, "state", state)
				bus.Publish(context.Background(), domain.NewEvent("sandbox.capacity", s.Graph.Metadata.ID, map[string]any{"node": s.Node.ID, "replica": s.Replica, "state": state}))
			}
			sb, err := tools.NewDockerSandbox(tools.DockerSandboxConfig{
				Capacity: &budget,
				Name:     fmt.Sprintf("leforge-sbx-%s-%s-%d", s.Graph.Metadata.ID, s.Node.ID, s.Replica),
				Image:    img, HostDir: s.HostWorkdir, Network: net,
				CPU: s.Node.Isolation.CPU, Memory: s.Node.Isolation.Memory,
				DockerBin: cfg.Isolation.Docker.Bin, DockerHost: cfg.Isolation.Docker.Host,
				Labels:  map[string]string{"leforge.graph": s.Graph.Metadata.ID, "leforge.node": s.Node.ID},
				Volumes: volumes,
			})
			if err != nil {
				return nil, err
			}
			if err := sb.Start(ctx); err != nil {
				return nil, errors.Join(err, sb.Close())
			}
			return sb, nil
		case domain.IsolationKubernetes:
			// In Kubernetes the platform pod runs tools in its own (restricted)
			// container; per-replica tool pods are an extension point.
			return &tools.ProcessSandbox{Root: s.Workdir}, nil
		default:
			for _, g := range s.Node.Tools {
				if g.Name == "*" || strings.HasPrefix(g.Name, "shell") {
					slog.Warn("node holds shell.exec under process isolation: commands run as the platform user and can read its environment and files; use isolation.mode docker (or kubernetes) for untrusted work",
						"graph", s.Graph.Metadata.ID, "node", s.Node.ID)
					break
				}
			}
			return &tools.ProcessSandbox{Root: s.Workdir}, nil
		}
	}
}

// Bootstrap loads graph documents from files/directories and starts the
// listed graphs - GitOps-style provisioning for examples and production.
func (a *App) Bootstrap(ctx context.Context) error {
	for _, p := range a.Config.Bootstrap.Graphs {
		files := []string{p}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			files = nil
			for _, pat := range []string{"*.yaml", "*.yml", "*.json"} {
				m, _ := filepath.Glob(filepath.Join(p, pat))
				files = append(files, m...)
			}
		}
		for _, f := range files {
			if err := a.loadGraph(ctx, f); err != nil {
				a.Log.Error("bootstrap graph", "file", f, "err", err)
			}
		}
	}
	for _, id := range a.Config.Bootstrap.Start {
		if err := a.Engine.GraphAction(ctx, id, "start", "bootstrap"); err != nil {
			a.Log.Error("bootstrap start", "graph", id, "err", err)
		}
	}
	return nil
}

func (a *App) loadGraph(ctx context.Context, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	parsed, unknown, err := graph.Decode(b, true)
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return &engine.ValidationError{Report: &graph.Report{Errors: unknown}}
	}
	// Compare the effective (normalised) documents, but save the file as
	// written so its defaults keep applying to nodes that do not override them.
	var effective domain.Graph
	if b, err := json.Marshal(parsed); err == nil {
		_ = json.Unmarshal(b, &effective)
	}
	a.Engine.Validate(&effective)
	if cur, _, err := a.Engine.GetGraph(ctx, effective.Metadata.ID, 0); err == nil {
		// Only save when the document changed, so restarts don't pile up versions.
		effective.Metadata.Version = cur.Metadata.Version
		effective.Metadata.CreatedAt, effective.Metadata.UpdatedAt = cur.Metadata.CreatedAt, cur.Metadata.UpdatedAt
		x, _ := yaml.Marshal(cur)
		y, _ := yaml.Marshal(&effective)
		if string(x) == string(y) {
			return nil
		}
	}
	saved, _, err := a.Engine.SaveGraph(ctx, parsed, 0, "bootstrap:"+filepath.Base(file))
	if err == nil {
		a.Log.Info("graph loaded", "graph", saved.Metadata.ID, "file", file)
	}
	return err
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func (a *App) Run(ctx context.Context) error {
	if err := a.Engine.Start(ctx); err != nil {
		return err
	}
	if err := a.Bootstrap(ctx); err != nil {
		return err
	}
	go a.refreshProviders(ctx)
	go a.Proposals.Run(ctx)
	srv := &http.Server{
		Addr:              a.Config.Server.Addr,
		Handler:           a.API.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 2)
	var internal *http.Server
	if h := a.API.InternalHandler(); h != nil {
		internal = &http.Server{Addr: a.Config.Server.InternalAddr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			a.Log.Info("callback API listening", "addr", a.Config.Server.InternalAddr)
			errc <- internal.ListenAndServe()
		}()
	}
	go func() {
		a.Log.Info("listening", "addr", a.Config.Server.Addr, "queue", a.Broker.Name(), "storage", a.Store.Name(),
			"providers", strings.Join(a.Models.Names(), ","), "default_provider", a.Models.DefaultName())
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	a.Log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Drain turns first, while the listeners are still up: an external agent
	// finishing its turn still needs the callback API.
	a.Engine.Shutdown(sctx)
	_ = srv.Shutdown(sctx)
	if internal != nil {
		_ = internal.Shutdown(sctx)
	}
	return a.Close(sctx)
}

func (a *App) Close(ctx context.Context) error {
	var errs []error
	if a.Broker != nil {
		errs = append(errs, a.Broker.Close())
	}
	if a.Store != nil {
		errs = append(errs, a.Store.Close())
	}
	if a.shutdownObs != nil {
		errs = append(errs, a.shutdownObs(ctx))
	}
	return errors.Join(errs...)
}
