//go:build !windows

package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/storage/fsstore"
)

// Opt-in against the real dedicated daemon. The wrapper supplied by the script
// forwards stdin, so the same tar transfer used in production is exercised.
func TestBuildContextRealSandbox(t *testing.T) {
	bin := os.Getenv("LEFORGE_TEST_ENV_DOCKER_BIN")
	if bin == "" {
		t.Skip("run make test-environment-context against the dedicated sandbox daemon")
	}
	dir := t.TempDir()
	store, err := fsstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Options{Store: store, Workspaces: fakeWorkspaces{dir}, DockerBin: bin, Instance: "context-test", Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("context-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := manager.Reset(ctx, id, "test"); err != nil {
			t.Errorf("cleanup test resources: %v", err)
		}
	})
	for name, body := range map[string]string{"copy.txt": "copied", "ignored.txt": "synthetic excluded fixture", ".env": "synthetic excluded fixture", ".dockerignore": "ignored.txt\n", "Dockerfile": "FROM alpine:3.22\nCOPY copy.txt /app/copy.txt\nADD payload.tar /app/unpacked/\nCOPY . /snapshot/\n"} {
		writeDiscoveryFile(t, dir, name, body)
	}
	var payload bytes.Buffer
	tw := tar.NewWriter(&payload)
	if err := tw.WriteHeader(&tar.Header{Name: "extracted.txt", Mode: 0644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("added")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload.tar"), payload.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	selection := discoverSelection(t, dir, "")
	env, err := Resolve(context.Background(), dir, selection)
	if err != nil {
		t.Fatal(err)
	}
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: id}, Spec: domain.GraphSpec{Environment: env}}
	wait := func() *View {
		t.Helper()
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			v, e := manager.Get(context.Background(), g)
			if e != nil {
				t.Fatal(e)
			}
			if v.State != StateBuilding {
				return v
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("real context build timed out")
		return nil
	}
	if _, err := manager.Build(context.Background(), g, false, "test"); err != nil {
		t.Fatal(err)
	}
	v := wait()
	if v.State != StateReady {
		t.Fatalf("build failed: %s\n%s", v.Error, v.Log)
	}
	var out bytes.Buffer
	check := `set -ex; test "$(id -u)" = 10001 && test "$(cat /app/copy.txt)" = copied && test "$(cat /app/unpacked/extracted.txt)" = added && test ! -e /snapshot/ignored.txt && test ! -e /snapshot/.env && test ! -e /snapshot/.leforge-build.Dockerfile && test "$(wc -l < /proc/net/route)" -eq 1 && echo context-sandbox-ok`
	if err := manager.docker(context.Background(), nil, &out, "run", "--rm", "--network", "none", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--entrypoint", "sh", v.Image, "-c", check); err != nil {
		t.Fatalf("sandbox: %v %s", err, out.String())
	}
	if !strings.Contains(out.String(), "context-sandbox-ok") {
		t.Fatalf("unexpected sandbox output %q", out.String())
	}
	// Verify the built image exists only on the nested daemon, not on the host.
	if err := exec.Command("docker", "image", "inspect", v.Image).Run(); err == nil {
		t.Fatal("environment image was built on the host daemon")
	}
	t.Log("COPY, local archive ADD, dockerignore, uid 10001 and network none verified in real sandbox")
	writeDiscoveryFile(t, dir, "copy.txt", "changed")
	if _, err := manager.Build(context.Background(), g, false, "test"); err == nil {
		t.Fatal("changed context reused cached image")
	}
	env, err = Resolve(context.Background(), dir, selection)
	if err != nil {
		t.Fatal(err)
	}
	g.Spec.Environment = env
	if _, err := manager.Build(context.Background(), g, false, "test"); err != nil {
		t.Fatal(err)
	}
	rebuilt := wait()
	if rebuilt.State != StateReady || rebuilt.Image == v.Image {
		t.Fatalf("rebuild: %+v", rebuilt)
	}
	// A missing COPY source must fail visibly and must not yield a ready image.
	writeDiscoveryFile(t, dir, "Dockerfile", "FROM alpine:3.22\nCOPY missing-file /missing\n")
	env, err = Resolve(context.Background(), dir, discoverSelection(t, dir, ""))
	if err != nil {
		t.Fatal(err)
	}
	g.Spec.Environment = env
	if _, err := manager.Build(context.Background(), g, false, "test"); err != nil {
		t.Fatal(err)
	}
	failed := wait()
	if failed.State != StateFailed || failed.Error == "" {
		t.Fatalf("missing source reported success: %+v", failed)
	}
	t.Log("changed-context rejection, rebuild and missing-source failure verified")
	// Select a development stage while an unrelated stage contains a failing COPY.
	writeDiscoveryFile(t, dir, "Dockerfile", `FROM alpine:3.22 AS builder
COPY copy.txt /artifact
FROM alpine:3.22 AS broken
COPY --from=builder /does-not-exist /missing
FROM alpine:3.22 AS development
COPY --from=0 /artifact /from-index
RUN echo development > /stage
FROM alpine:3.22 AS runtime
COPY --from=development /from-index /from-name
RUN echo runtime > /stage
`)
	selection = discoverSelection(t, dir, "")
	previous := ""
	for _, target := range []string{"development", "runtime", ""} {
		selection.Target = target
		env, err = Resolve(context.Background(), dir, selection)
		if err != nil {
			t.Fatal(err)
		}
		g.Spec.Environment = env
		if _, err := manager.Build(context.Background(), g, false, "test"); err != nil {
			t.Fatal(err)
		}
		built := wait()
		if built.State != StateReady || built.Image == previous {
			t.Fatalf("target %q: %+v", target, built)
		}
		previous = built.Image
		expected, file := target, "/from-index"
		if target != "development" {
			expected, file = "runtime", "/from-name"
		}
		out.Reset()
		if err := manager.docker(context.Background(), nil, &out, "run", "--rm", "--network", "none", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--entrypoint", "sh", built.Image, "-ec", `test "$(id -u)" = 10001; test "$(cat /stage)" = "$1"; test "$(cat "$2")" = changed`, "check", expected, file); err != nil {
			t.Fatalf("target %q sandbox: %v %s", target, err, out.String())
		}
	}
	selection.Target = "broken"
	env, err = Resolve(context.Background(), dir, selection)
	if err != nil {
		t.Fatal(err)
	}
	g.Spec.Environment = env
	if _, err := manager.Build(context.Background(), g, false, "test"); err != nil {
		t.Fatal(err)
	}
	if broken := wait(); broken.State != StateFailed || broken.Error == "" {
		t.Fatalf("stage dependency failure: %+v", broken)
	}
	t.Log("named/numeric COPY --from, explicit/default targets, target-specific images and dependency failure verified in real sandbox")

}
