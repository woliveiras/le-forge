package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"github.com/woliveiras/le-forge/server/internal/sandboxcapacity"
	"github.com/woliveiras/le-forge/server/internal/tools"
)

type buildBudgetKey struct{}
type buildBudget struct {
	id     string
	memory int64
	cpu    int64
}

func builderName(id string) string      { return "leforge-builder-" + id }
func builderContainer(id string) string { return "buildx_buildkit_" + builderName(id) + "0" }

// One reservation covers sequential build, cache copies and setup. It survives
// crashes until this graph's next build/reset confirms all workers are gone.
func (m *Manager) withBuildCapacity(ctx context.Context, g *domain.Graph, log io.Writer, run func(context.Context) (string, []CacheMount, error)) (image string, caches []CacheMount, err error) {
	if m.opt.Capacity == nil {
		return run(ctx)
	}
	budget := *m.opt.Capacity
	budget.Notify = func(name, state string) {
		if state == "waiting" {
			setProgress(ctx, "waiting_capacity", "")
		} else {
			setProgress(ctx, "preparing", "")
		}
		fmt.Fprintf(log, "==> Environment capacity: %s\n", state)
		m.opt.Log.Info("environment capacity", "graph", g.Metadata.ID, "state", state)
		m.publish(context.WithoutCancel(ctx), domain.EventType("sandbox.capacity"), g.Metadata.ID, "", map[string]any{"phase": "environment", "state": state})
	}
	memory, err := budget.Memory(g.Spec.Environment.Memory)
	if err != nil {
		return "", nil, err
	}
	cpu, err := budget.CPU(g.Spec.Environment.CPU)
	if err != nil {
		return "", nil, err
	}
	// The graph build lease excludes concurrent recovery of the same pipeline.
	if err := m.cleanBuildResources(ctx, g.Metadata.ID); err != nil {
		return "", nil, err
	}
	if err := budget.Release(ctx, buildContainer(g.Metadata.ID)); err != nil {
		return "", nil, err
	}
	inventory := func(ctx context.Context) ([]sandboxcapacity.Container, error) {
		return tools.DockerCapacityInventory(ctx, func(ctx context.Context, args ...string) (string, error) {
			var out bytes.Buffer
			err := m.docker(ctx, nil, &out, args...)
			return out.String(), err
		})
	}
	if err := budget.ReserveCPU(ctx, buildContainer(g.Metadata.ID), memory, cpu, inventory); err != nil {
		return "", nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		e := m.cleanBuildResources(cleanup, g.Metadata.ID)
		if e == nil {
			e = budget.Release(cleanup, buildContainer(g.Metadata.ID))
		}
		if e != nil {
			fmt.Fprintf(log, "==> Capacity retained: cleanup failed: %v\n", e)
			err = errors.Join(err, fmt.Errorf("environment cleanup (retry build or reset): %w", e))
		}
	}()
	return run(context.WithValue(ctx, buildBudgetKey{}, buildBudget{g.Metadata.ID, memory, cpu}))
}

// Names are deterministic so a new control-plane process can recover workers.
func (m *Manager) cleanBuildResources(ctx context.Context, id string) error {
	for _, name := range []string{builderContainer(id), buildContainer(id), buildContainer(id) + "-copy"} {
		var out bytes.Buffer
		if err := m.docker(ctx, nil, &out, "ps", "-aq", "--filter", "name=^/"+regexp.QuoteMeta(name)+"$"); err != nil {
			return fmt.Errorf("list build workers: %w", err)
		}
		if ids := strings.Fields(out.String()); len(ids) > 0 {
			if err := m.docker(ctx, nil, io.Discard, append([]string{"rm", "-f"}, ids...)...); err != nil {
				return fmt.Errorf("remove build worker: %w", err)
			}
		}
	}
	return nil
}

func boundedRunArgs(ctx context.Context, args []string) []string {
	b, ok := ctx.Value(buildBudgetKey{}).(buildBudget)
	if !ok || len(args) == 0 || args[0] != "run" {
		return args
	}
	result := []string{"run", "--memory", strconv.FormatInt(b.memory, 10), "--memory-swap", strconv.FormatInt(b.memory, 10), "--cpus", sandboxcapacity.FormatCPU(b.cpu)}
	named := false
	// All arguments originate in this package, with separate flag/value tokens.
	for i := 1; i < len(args); i++ {
		if args[i] == "--memory" || args[i] == "--memory-swap" || args[i] == "--cpus" {
			i++
			continue
		}
		if args[i] == "--name" {
			named = true
		}
		result = append(result, args[i])
	}
	if !named {
		result = append([]string{result[0], "--name", buildContainer(b.id) + "-copy"}, result[1:]...)
	}
	return result
}

// combinedBuildfile keeps local stages inside one BuildKit solve. The generated
// user layer must inherit the selected stage, not a daemon-local intermediate tag.
func combinedBuildfile(source, target string, caches []string) (string, error) {
	stages, err := graph.DockerfileStages(source)
	if err != nil {
		return "", err
	}
	if len(stages) == 0 {
		return "", fmt.Errorf("Dockerfile requires FROM")
	}
	if err := graph.ValidateBuildTarget(source, target); err != nil {
		return "", err
	}
	selected := len(stages) - 1
	if target != "" {
		for i, name := range stages {
			if name == strings.ToLower(target) {
				selected = i
			}
		}
	}
	lines, err := graph.DockerfileLines(source)
	if err != nil {
		return "", err
	}
	alias := stages[selected]
	if alias == "" {
		alias = "leforge-selected"
		// Avoid collisions with both named stages and external image references.
		for strings.Contains(strings.ToLower(source), alias) {
			alias += "x"
		}
		stage := -1
		for i, line := range lines {
			if strings.EqualFold(strings.Fields(line)[0], "FROM") {
				stage++
				if stage == selected {
					lines[i] += " AS " + alias
				}
			}
		}
	}
	return strings.Join(lines, "\n") + "\n" + UserLayer(alias, caches), nil
}

func (m *Manager) prepareBoundedImage(ctx context.Context, g *domain.Graph, final string, log io.Writer) (tag string, err error) {
	b, ok := ctx.Value(buildBudgetKey{}).(buildBudget)
	if !ok {
		return "", fmt.Errorf("environment build requires capacity admission")
	}
	env := g.Spec.Environment
	source := env.Dockerfile
	var snapshot *ContextSnapshot
	if env.Context != "" {
		dir, e := m.opt.Workspaces.Ensure(ctx, g)
		if e != nil {
			return "", e
		}
		snapshot, e = SnapshotContext(ctx, dir, env)
		if e != nil {
			return "", e
		}
		defer snapshot.Close()
		if env.Origin == nil || snapshot.SHA256 != env.Origin.ContextSHA256 {
			return "", fmt.Errorf("%w: build context changed after review", ErrInvalidDefinition)
		}
		source, e = contextBuildfile(source)
		if e != nil {
			return "", e
		}
	}
	var dockerfile string
	if env.Image != "" {
		dockerfile = UserLayer(env.Image, env.Caches)
	} else {
		dockerfile, err = combinedBuildfile(source, env.Target, env.Caches)
		if err != nil {
			return "", err
		}
	}
	var input io.Reader = strings.NewReader(dockerfile)
	args := []string{"buildx", "build", "--builder", builderName(b.id), "--load", "--pull", "--progress", "plain", "-t", final + "-user", "--label", "leforge.graph=" + b.id, "--label", "leforge.environment=" + Hash(env)}
	if snapshot != nil {
		f, e := wrappedContext(snapshot.File, dockerfile)
		if e != nil {
			return "", e
		}
		defer func() { f.Close(); os.Remove(f.Name()) }()
		input = f
		args = append(args, "--file", contextDockerfile)
	}
	// Remove only our generated Buildx configuration. Physical workers were
	// already reconciled before admission; an absent local builder is harmless.
	var builders bytes.Buffer
	if err = m.docker(ctx, nil, &builders, "buildx", "ls", "--format", "{{.Name}}"); err != nil {
		return "", err
	}
	for _, name := range strings.Fields(builders.String()) {
		if strings.TrimSuffix(name, "*") == builderName(b.id) {
			if err = m.docker(ctx, nil, log, "buildx", "rm", "--force", builderName(b.id)); err != nil {
				return "", err
			}
		}
	}
	memory := strconv.FormatInt(b.memory, 10)
	create := []string{"buildx", "create", "--name", builderName(b.id), "--driver", "docker-container", "--driver-opt", "memory=" + memory + ",memory-swap=" + memory + ",restart-policy=no,cpu-period=100000,cpu-quota=" + strconv.FormatInt(b.cpu*100, 10)}
	if err = m.docker(ctx, nil, log, create...); err != nil {
		return "", fmt.Errorf("create bounded builder: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		e := m.docker(cleanup, nil, log, "buildx", "rm", "--force", builderName(b.id))
		if e != nil {
			err = errors.Join(err, fmt.Errorf("remove bounded builder: %w", e))
		}
	}()
	fmt.Fprintf(log, "==> Building with a %d-byte worker memory cap\n", b.memory)
	if err = m.docker(ctx, input, log, append(args, "-")...); err != nil {
		return "", fmt.Errorf("bounded Dockerfile build: %w", err)
	}
	return final + "-user", nil
}

// Transform only the generated Dockerfile after validating the original digest.
func wrappedContext(input io.Reader, dockerfile string) (file *os.File, err error) {
	file, err = os.CreateTemp("", "leforge-bounded-context-*.tar")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()
	tr := tar.NewReader(input)
	tw := tar.NewWriter(file)
	found := false
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return file, e
		}
		var body io.Reader = tr
		if h.Name == contextDockerfile {
			h.Size = int64(len(dockerfile))
			body = strings.NewReader(dockerfile)
			found = true
		}
		if e = tw.WriteHeader(h); e != nil {
			return file, e
		}
		if _, e = io.Copy(tw, body); e != nil {
			return file, e
		}
	}
	if !found {
		return file, fmt.Errorf("snapshot lacks generated Dockerfile")
	}
	if err = tw.Close(); err != nil {
		return file, err
	}
	_, err = file.Seek(0, io.SeekStart)
	return file, err
}
