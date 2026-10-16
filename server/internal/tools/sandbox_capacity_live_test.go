//go:build !windows

package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/sandboxcapacity"
	"github.com/woliveiras/le-forge/server/internal/storage/fsstore"
)

func TestSandboxCapacityRealDaemon(t *testing.T) {
	bin := os.Getenv("LEFORGE_TEST_ENV_DOCKER_BIN")
	if bin == "" {
		t.Skip("run make test-sandbox-capacity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, e := fsstore.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	prefix := fmt.Sprintf("leforge-capacity-test-%d", time.Now().UnixNano())
	waited := make(chan string, 10)
	budget := &sandboxcapacity.Manager{Store: store, Scope: prefix, Policy: sandboxcapacity.Policy{MaxActive: 1, TotalMemory: "64m", DefaultMemory: "64m"}, Notify: func(n, s string) {
		if s == "waiting" {
			waited <- n
		}
	}}
	newSandbox := func(suffix string) *DockerSandbox {
		t.Helper()
		copy := *budget
		s, e := NewDockerSandbox(DockerSandboxConfig{Name: prefix + suffix, Image: "alpine:3.22", HostDir: prefix + suffix, User: "10001", DockerBin: bin, Capacity: &copy})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := newSandbox("-a")
	active, e := a.inventory(ctx)
	if e != nil {
		t.Fatal(e)
	}
	// Include existing bounded sandboxes without modifying or stopping user work.
	total := int64(64 << 20)
	for _, c := range active {
		if c.Memory <= 0 {
			t.Fatal("existing unbounded sandbox prevents capacity validation; stop it explicitly before this test")
		}
		total += c.Memory
	}
	budget.Policy.MaxActive = len(active) + 1
	budget.Policy.TotalMemory = strconv.FormatInt(total+(64<<20), 10)
	a = newSandbox("-a")
	b := newSandbox("-b")
	t.Cleanup(func() {
		for _, s := range []*DockerSandbox{a, b} {
			if e := s.Close(); e != nil {
				t.Error(e)
			}
			if out, e := DockerCommand(context.Background(), bin, "", "volume", "rm", s.cfg.HostDir).CombinedOutput(); e != nil {
				t.Errorf("cleanup test volume: %v %s", e, out)
			}
		}
	})
	if e := a.Start(ctx); e != nil {
		t.Fatal(e)
	}
	out, e := a.docker(ctx, "inspect", "--format", "{{.HostConfig.Memory}} {{.HostConfig.MemorySwap}}", a.cfg.Name)
	if e != nil || strings.TrimSpace(out) != "67108864 67108864" {
		t.Fatalf("kernel limits: %q %v", out, e)
	}
	result, e := a.Exec(ctx, ExecRequest{Shell: `test "$(id -u)" = 10001; test "$(wc -l < /proc/net/route)" -eq 1; if test -f /sys/fs/cgroup/memory.max; then test "$(cat /sys/fs/cgroup/memory.max)" = 67108864; else test "$(cat /sys/fs/cgroup/memory/memory.limit_in_bytes)" = 67108864; fi`})
	if e != nil || result.ExitCode != 0 {
		t.Fatalf("cgroup check %+v %v", result, e)
	}
	done := make(chan error, 1)
	go func() { done <- b.Start(ctx) }()
	select {
	case <-waited:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	out, e = b.docker(ctx, "ps", "-q", "--filter", "name=^/"+b.cfg.Name+"$")
	if e != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("waiting sandbox was started: %s %v", out, e)
	}
	if e := a.Close(); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	out, e = b.docker(ctx, "inspect", "--format", "{{.State.Running}} {{.HostConfig.Memory}}", b.cfg.Name)
	if e != nil || strings.TrimSpace(out) != "true 67108864" {
		t.Fatalf("queued sandbox: %q %v", out, e)
	}
	// With a second slot available, memory alone must still queue admission.
	budget.Policy.MaxActive = len(active) + 2
	budget.Policy.TotalMemory = strconv.FormatInt(total, 10)
	c := newSandbox("-c")
	go func() { done <- c.Start(ctx) }()
	select {
	case <-waited:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if e := c.Close(); e != nil {
		t.Fatal(e)
	}
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Fatalf("queued memory request cancellation: %v", e)
	}
	// A restarted controller reuses the same reservation and container.
	restored := newSandbox("-b")
	if e := restored.Start(ctx); e != nil {
		t.Fatal(e)
	}
	t.Log("real daemon: one available slot; count-limited sandbox waits then starts after removal; memory-only queue cancels without starting; 64 MiB cgroup limit and no extra swap; restart reuses reservation; uid 10001/network none")
}
