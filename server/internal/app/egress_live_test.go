//go:build !windows

package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/environment"
	"github.com/woliveiras/le-forge/server/internal/events"
	"github.com/woliveiras/le-forge/server/internal/storage/fsstore"
	"github.com/woliveiras/le-forge/server/internal/tools"
)

type egressWorkspace string

func (w egressWorkspace) Ensure(context.Context, *domain.Graph) (string, error) {
	return string(w), nil
}
func (w egressWorkspace) HostPath(p string) string { return p }

func TestEgressRealSandbox(t *testing.T) {
	bin, image := os.Getenv("LEFORGE_TEST_ENV_DOCKER_BIN"), os.Getenv("LEFORGE_TEST_EGRESS_IMAGE")
	if bin == "" || image == "" {
		t.Skip("run make test-egress")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id := fmt.Sprintf("egress-test-%d", time.Now().UnixNano())
	store, err := fsstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Isolation.Docker.Bin = bin
	cfg.Isolation.Docker.EgressImage = os.Getenv("LEFORGE_TEST_EGRESS_PROXY_IMAGE")
	if cfg.Isolation.Docker.EgressImage == "" {
		t.Fatal("test proxy image required")
	}
	cfg.Isolation.Docker.EnvironmentPolicy.RunNetworks = []string{"none", "egress"}
	cfg.Isolation.Docker.EnvironmentPolicy.EgressHosts = []string{"example.com", "127.0.0.1.nip.io"}
	cfg.Isolation.Docker.Capacity.DefaultMemory = "64m"
	cfg.Isolation.Docker.Capacity.DefaultCPU = "0.25"
	bus := events.NewBus(store, slog.Default())
	manager, err := environment.New(environment.Options{Store: store, Workspaces: egressWorkspace(id + "-workspace"), DockerBin: bin, Policy: cfg.Isolation.Docker.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: id}, Spec: domain.GraphSpec{Environment: &domain.EnvironmentSpec{Image: image, Network: domain.EnvironmentNetwork{Run: "egress", AllowedHosts: []string{"example.com", "127.0.0.1.nip.io"}}}}}
	// Seed a previously prepared image, exercising the production runtime factory.
	data := fmt.Sprintf(`{"graph":%q,"state":"ready","hash":%q,"image":%q}`, id, environment.Hash(g.Spec.Environment), image)
	if err := store.KVPut(ctx, "environment", id, []byte(data)); err != nil {
		t.Fatal(err)
	}
	spec := engine.SandboxSpec{Graph: g, Node: &domain.Node{ID: "worker", Isolation: domain.IsolationSpec{Mode: domain.IsolationDocker}}, HostWorkdir: id + "-workspace"}
	factory := sandboxFactory(cfg, manager, store, bus)
	sandbox, err := factory(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sandbox.Close(); err != nil {
			t.Error(err)
		}
		out, err := tools.DockerCommand(context.Background(), bin, "", "volume", "rm", id+"-workspace").CombinedOutput()
		if err != nil {
			t.Error(string(out), err)
		}
	})
	run := func(command string, success bool) {
		t.Helper()
		result, err := sandbox.Exec(ctx, tools.ExecRequest{Shell: command})
		if err != nil || (result.ExitCode == 0) != success {
			t.Fatalf("%s: %+v %v", command, result, err)
		}
	}
	name := "leforge-sbx-" + id + "-worker-0-svc-leforge-egress"
	// The allowed fixture name resolves to loopback; policy must still reject it.
	run(`output=$(/probe http://127.0.0.1.nip.io); echo "$output"; test "$output" = 403`, true)
	run("/probe http://example.com", true)
	run("/probe https://example.com", true)
	run("/probe http://example.org", false)
	run("/probe http://169.254.169.254", false)
	run("/probe http://1.1.1.1 direct", false)
	run(`! awk '$2 == "00000000" { found=1 } END { exit !found }' /proc/net/route`, true)
	suspend, ok := sandbox.(interface{ Suspend(context.Context) error })
	if !ok {
		t.Fatal("missing suspension")
	}
	if err := suspend.Suspend(ctx); err != nil {
		t.Fatal(err)
	}
	sandbox, err = factory(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	run("/probe https://example.com", true)
	logs, err := tools.DockerCommand(ctx, bin, "", "logs", name).CombinedOutput()
	if err != nil || !strings.Contains(string(logs), `"outcome":"denied"`) {
		t.Fatal(string(logs), err)
	}
	if out, err := tools.DockerCommand(ctx, bin, "", "stop", name).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	if _, err := sandbox.Exec(ctx, tools.ExecRequest{Shell: "/probe http://example.com"}); err == nil {
		t.Fatal("proxy failure not propagated")
	}
	t.Log("HTTP/HTTPS allowed, unlisted hosts/private IPs/direct egress blocked, suspension/resume and proxy failure verified")
}
