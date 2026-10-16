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
	"strings"
	"syscall"
	"time"

	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/llm"
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
	root, err := os.MkdirTemp("", "leforge-browser-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	bin := filepath.Join(root, "docker-fixture")
	// Consume build input so context uploads exercise the production tar pipe.
	script := `#!/bin/sh
case "$1" in
 version) echo linux/arm64;;
 build) data=$(cat); case "$data" in *E2E_BUILD_FAIL*) echo 'E2E controlled build failure' >&2; exit 1;; esac;;
 buildx) if [ "$2" = build ]; then data=$(cat); case "$data" in *E2E_BUILD_FAIL*) echo 'E2E controlled build failure' >&2; exit 1;; esac; fi;;
 inspect) echo "No such container" >&2; exit 1;;
 exec) echo e2e-executed;;
esac
exit 0
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		return err
	}
	cfg := config.Default()
	cfg.Instance = "browser-e2e"
	cfg.LLM = config.LLM{}
	cfg.Server.Addr = "127.0.0.1:18089"
	cfg.Server.InternalAddr = ""
	cfg.Server.WebDir = "../web/dist"
	cfg.Storage = config.Storage{Type: "sqlite", Dir: filepath.Join(root, "store")}
	cfg.Queue = config.Queue{Type: "memory"}
	cfg.Workspaces.Root = filepath.Join(root, "ws")
	cfg.Secrets = config.Secrets{KeyFile: filepath.Join(root, "fixture.key"), GenerateKeyFile: true}
	cfg.Auth.Tokens = []config.Token{{Name: "e2e-admin", Role: config.RoleAdmin, Token: "e2e-admin-token"}, {Name: "e2e-viewer", Role: config.RoleViewer, Token: "e2e-viewer-token"}}
	cfg.Isolation.Docker.Bin = bin
	cfg.Isolation.Docker.Host = ""
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
	for _, id := range []string{"discover-a", "discover-b", "proposal-accept", "proposal-reject", "analysis-failure", "build-failure"} {
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
		if strings.HasPrefix(id, "discover-") || id == "build-failure" {
			body := "FROM alpine:3.22\nRUN echo ready\n"
			if id == "build-failure" {
				body = "FROM alpine:3.22\nRUN E2E_BUILD_FAIL\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(body), 0600); err != nil {
				return err
			}
		}
		g := &domain.Graph{Metadata: domain.GraphMetadata{Name: id}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "worker", Loop: domain.LoopSpec{Mode: domain.LoopOnce}, Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}, Isolation: domain.IsolationSpec{Mode: domain.IsolationDocker}}}}}
		if strings.HasPrefix(id, "discover-") {
			g.Spec.Workspace.Repo = "https://example.invalid/same-repository.git"
		}
		if _, _, err := a.Engine.SaveGraph(ctx, g, 0, "e2e-fixture"); err != nil {
			return err
		}
	}
	return a.Run(ctx)
}
