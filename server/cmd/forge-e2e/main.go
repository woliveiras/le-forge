//go:build e2e

// forge-e2e is a disposable, loopback-only browser test installation. It is
// deliberately excluded from production builds. No real Docker or LLM is used.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/llm"
	"github.com/woliveiras/le-forge/server/internal/sandboxcapacity"
)

type analysisProvider struct{}

func (analysisProvider) Name() string                                    { return "e2e" }
func (analysisProvider) Models(context.Context) ([]llm.ModelInfo, error) { return nil, nil }
func (analysisProvider) Chat(_ context.Context, r *llm.ChatRequest) (*llm.ChatResponse, error) {
	for _, m := range r.Messages {
		if strings.Contains(m.Content, "analysis-failure") {
			return nil, fmt.Errorf("E2E analysis provider unavailable")
		}
	}
	return &llm.ChatResponse{Message: llm.ChatMessage{Content: `{"environment":{"image":"alpine:3.22","network":{"setup":"none","run":"none"}},"rationale":"E2E minimal shell, no dependency setup required.","tools":["sh"]}`}}, nil
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	if os.Getenv("LEFORGE_E2E_REAL") == "1" {
		return runReal()
	}
	root, err := os.MkdirTemp("", "leforge-browser-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	bin := filepath.Join(root, "docker-fixture")
	if err := os.WriteFile(bin, []byte(dockerFixture), 0700); err != nil {
		return err
	}
	cfg := config.Default()
	cfg.Instance = "browser-e2e"
	cfg.LLM = config.LLM{}
	addr, err := browserAddress(os.Getenv("LEFORGE_E2E_PORT"))
	if err != nil {
		return err
	}
	cfg.Server.Addr = addr
	cfg.Server.InternalAddr = ""
	cfg.Server.WebDir = "../web/dist"
	cfg.Storage = config.Storage{Type: "sqlite", Dir: filepath.Join(root, "store")}
	cfg.Queue = config.Queue{Type: "memory"}
	cfg.Workspaces.Root = filepath.Join(root, "ws")
	cfg.Secrets = config.Secrets{KeyFile: filepath.Join(root, "fixture.key"), GenerateKeyFile: true}
	cfg.Auth.Tokens = []config.Token{{Name: "e2e-admin", Role: config.RoleAdmin, Token: "e2e-admin-token"}, {Name: "e2e-viewer", Role: config.RoleViewer, Token: "e2e-viewer-token"}}
	cfg.Isolation.Docker.Bin = bin
	cfg.Isolation.Docker.Host = ""
	cfg.Isolation.Docker.IdleTimeout = time.Second
	cfg.Isolation.Docker.Capacity = sandboxcapacity.Policy{TotalCPU: "1", DefaultCPU: "0.1", TotalMemory: "256m", DefaultMemory: "32m", MaxActive: 10}
	cfg.Observability.LogLevel = "error"
	cfg.Engine.ReconcileInterval = 100 * time.Millisecond
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	a.Models.Add(llm.ProviderConfig{Name: "e2e", DefaultModel: "fixture"}, analysisProvider{})
	for _, id := range []string{"discover-a", "discover-b", "proposal-accept", "proposal-reject", "analysis-failure", "build-failure", "retry-build", "cpu-holder", "cpu-waiter", "memory-holder", "memory-waiter", "idle-resume", "preparation-holder", "preparation-waiter"} {
		dir := filepath.Join(cfg.Workspaces.Root, id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if out, err := exec.CommandContext(ctx, "git", "init", "-q", dir).CombinedOutput(); err != nil {
			return fmt.Errorf("init fixture: %w: %s", err, out)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"`+id+`"}`), 0600); err != nil {
			return err
		}
		// Identical manifests and repository identity allow cross-task reuse.
		if strings.HasPrefix(id, "discover-") {
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"same-repository"}`), 0600); err != nil {
				return err
			}
		}
		if strings.HasPrefix(id, "discover-") || id == "build-failure" || id == "retry-build" {
			body := "FROM alpine:3.22\nRUN echo ready\n"
			if id == "retry-build" {
				body = "FROM alpine:3.22\nRUN E2E_FAIL_ONCE\n"
			}
			if id == "build-failure" {
				body = "FROM alpine:3.22\nRUN E2E_BUILD_FAIL\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(body), 0600); err != nil {
				return err
			}
		}
		g := &domain.Graph{Metadata: domain.GraphMetadata{Name: id}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "worker", Loop: domain.LoopSpec{Mode: domain.LoopOnce}, Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}, Isolation: domain.IsolationSpec{Mode: domain.IsolationDocker}}}}}

		node := &g.Spec.Nodes[0]
		switch id {
		case "cpu-holder", "memory-holder", "preparation-holder":
			node.Loop = domain.LoopSpec{Mode: domain.LoopContinuous, Interval: domain.Duration(100 * time.Millisecond)}
			if id == "cpu-holder" || id == "preparation-holder" {
				node.Isolation.CPU = "0.8"
			} else {
				node.Isolation.Memory = "224m"
			}
		case "preparation-waiter":
			g.Spec.Environment = &domain.EnvironmentSpec{Image: "alpine:3.22", CPU: "0.4", Network: domain.EnvironmentNetwork{Setup: "none", Run: "none"}, Setup: []string{"echo preparation-completed"}}
		case "cpu-waiter":
			node.Isolation.CPU = "0.4"
		case "memory-waiter":
			node.Isolation.Memory = "64m"
		case "idle-resume":
			node.Loop = domain.LoopSpec{Mode: domain.LoopEvent}
			if err := os.WriteFile(filepath.Join(dir, "preserved.txt"), []byte("workspace survives suspension\n"), 0600); err != nil {
				return err
			}
		}
		if strings.HasPrefix(id, "discover-") {
			g.Spec.Workspace.Repo = "https://example.invalid/same-repository.git"
		}
		if _, _, err := a.Engine.SaveGraph(ctx, g, 0, "e2e-fixture"); err != nil {
			return err
		}
	}
	return a.Run(ctx)
}

// browserAddress keeps disposable controllers on loopback, including parallel runs.
func browserAddress(port string) (string, error) {
	if port == "" {
		port = "18089"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid LEFORGE_E2E_PORT")
	}
	return fmt.Sprintf("127.0.0.1:%d", n), nil
}
