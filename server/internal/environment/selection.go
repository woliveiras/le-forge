package environment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"gopkg.in/yaml.v3"
)

var ErrInvalidDefinition = errors.New("environment definition is invalid or changed; discover and review it again")

// Selection refers to the exact candidate shown by discovery, never arbitrary
// caller-supplied source text. Preview is read-only; acceptance uses graph PUT.
type Selection struct {
	Source  string `json:"source"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Service string `json:"service,omitempty"`
}

func Resolve(ctx context.Context, dir string, sel Selection) (*domain.EnvironmentSpec, error) {
	env, err := resolve(ctx, dir, sel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDefinition, err)
	}
	return env, nil
}

func resolve(ctx context.Context, dir string, sel Selection) (*domain.EnvironmentSpec, error) {
	discovery, err := Discover(ctx, dir)
	if err != nil {
		return nil, err
	}
	if discovery.Source != sel.Source {
		return nil, fmt.Errorf("repository source priority changed")
	}
	found := false
	for _, c := range discovery.Candidates {
		if c.Path == sel.Path && c.SHA256 == sel.SHA256 && c.Error == "" && sel.SHA256 != "" {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("selected candidate changed or is unavailable")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	origin := &domain.EnvironmentOrigin{Source: sel.Source, Path: sel.Path, SHA256: sel.SHA256, Service: sel.Service, Files: map[string]string{}}
	read := func(name string) ([]byte, error) {
		if !fsLocal(name) {
			return nil, fmt.Errorf("file path must stay inside the repository")
		}
		data, err := readDefinition(root, name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		origin.Files[name] = hex.EncodeToString(sum[:])
		return data, nil
	}
	data, err := read(sel.Path)
	if err != nil {
		return nil, err
	}
	if origin.Files[sel.Path] != sel.SHA256 {
		return nil, fmt.Errorf("selected file changed during review")
	}
	env := &domain.EnvironmentSpec{}
	switch sel.Source {
	case "leforge":
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(env); err != nil {
			return nil, fmt.Errorf("invalid environment fields")
		}
		if env.Origin != nil {
			return nil, fmt.Errorf("repository definitions cannot set origin")
		}
		if env.Dockerfile != "" && !strings.Contains(env.Dockerfile, "\n") && !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(env.Dockerfile)), "FROM ") {
			content, err := read(env.Dockerfile)
			if err != nil {
				return nil, err
			}
			env.Dockerfile = string(content)
		}
	case "devcontainer":
		var doc struct {
			Name    string `json:"name"`
			Image   string `json:"image"`
			Command string `json:"postCreateCommand"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&doc); err != nil {
			return nil, fmt.Errorf("unsupported devcontainer fields")
		}
		env.Image = doc.Image
		if doc.Command != "" {
			env.Setup = []string{doc.Command}
		}
	case "dockerfile":
		env.Dockerfile = string(data)
	case "compose":
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("invalid Compose document")
		}
		for key := range doc {
			if key != "services" && key != "name" && key != "version" {
				return nil, fmt.Errorf("unsupported Compose field %q", key)
			}
		}
		services, ok := doc["services"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Compose services")
		}
		service, ok := services[sel.Service].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("select an existing Compose service")
		}
		for key := range service {
			if key != "image" && key != "build" {
				return nil, fmt.Errorf("unsupported selected service field %q; companion services and runtime settings are not supported yet", key)
			}
		}
		if image, ok := service["image"]; ok {
			var valid bool
			env.Image, valid = image.(string)
			if !valid {
				return nil, fmt.Errorf("service image must be a string")
			}
		}
		if build, ok := service["build"]; ok {
			if env.Image != "" {
				return nil, fmt.Errorf("select a service with image or build, not both")
			}
			contextDir, dockerfile := ".", "Dockerfile"
			switch b := build.(type) {
			case string:
				contextDir = b
			case map[string]any:
				for key := range b {
					if key != "context" && key != "dockerfile" {
						return nil, fmt.Errorf("unsupported Compose build field %q", key)
					}
				}
				if v, exists := b["context"]; exists {
					var ok bool
					contextDir, ok = v.(string)
					if !ok {
						return nil, fmt.Errorf("build context must be a string")
					}
				}
				if v, exists := b["dockerfile"]; exists {
					var ok bool
					dockerfile, ok = v.(string)
					if !ok {
						return nil, fmt.Errorf("Dockerfile path must be a string")
					}
				}
			default:
				return nil, fmt.Errorf("unsupported Compose build")
			}
			if !fsLocal(contextDir) || !fsLocal(dockerfile) {
				return nil, fmt.Errorf("build paths must stay inside the repository")
			}
			content, err := read(path.Join(path.Dir(sel.Path), contextDir, dockerfile))
			if err != nil {
				return nil, err
			}
			env.Dockerfile = string(content)
		}
	default:
		return nil, fmt.Errorf("unsupported source")
	}
	if sel.Source != "compose" && sel.Service != "" {
		return nil, fmt.Errorf("service is only valid for Compose")
	}
	// Repository files may not widen execution networking or set resource budgets.
	// Operators can explicitly configure those separately as a manual environment.
	if env.Network.Run != "" && env.Network.Run != "none" {
		return nil, fmt.Errorf("repository execution network must be none")
	}
	if env.CPU != "" || env.Memory != "" {
		return nil, fmt.Errorf("repository resource overrides are not supported; configure a manual environment")
	}
	if strings.Contains(env.Image, "$") {
		return nil, fmt.Errorf("image interpolation is not supported")
	}
	// Make effective network defaults visible in the review, not implicit.
	if env.Network.Setup == "" {
		env.Network.Setup = "bridge"
	}
	if env.Network.Run == "" {
		env.Network.Run = "none"
	}
	if env.Dockerfile != "" {
		if err := validateRepositoryDockerfile(env.Dockerfile); err != nil {
			return nil, err
		}
	}
	if err := graph.ValidateEnvironment(env).Err(); err != nil {
		return nil, err
	}
	env.Origin = origin
	return env, nil
}

func fsLocal(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\$\x00") || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// Current builds have no repository context. Fail explicitly instead of letting
// COPY/ADD or external frontends silently change the accepted build semantics.
func validateRepositoryDockerfile(text string) error {
	pending := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") {
			directive := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "#")))
			key, _, isDirective := strings.Cut(directive, "=")
			if isDirective && (strings.TrimSpace(key) == "syntax" || strings.TrimSpace(key) == "escape") {
				return fmt.Errorf("custom Dockerfile parser directives are not supported")
			}
			continue
		}
		if strings.HasSuffix(line, "\\") {
			pending += strings.TrimSuffix(line, "\\") + " "
			continue
		}
		line = pending + line
		pending = ""
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 2 {
			return fmt.Errorf("Dockerfile instruction requires an argument")
		}
		switch strings.ToUpper(fields[0]) {
		case "FROM", "RUN", "ENV", "ARG", "WORKDIR", "USER", "SHELL", "LABEL", "CMD", "ENTRYPOINT", "EXPOSE", "STOPSIGNAL", "HEALTHCHECK":
			if strings.HasPrefix(fields[1], "--") || strings.Contains(line, "<<") {
				return fmt.Errorf("Dockerfile flags and heredocs are not supported")
			}
		default:
			return fmt.Errorf("unsupported Dockerfile instruction %q; repository COPY/ADD and build contexts are not supported yet", fields[0])
		}
	}
	if pending != "" {
		return fmt.Errorf("unterminated Dockerfile continuation")
	}
	return nil
}

// ValidateAccepted re-resolves the source immediately before a build. It checks
// every referenced file and the actual normalized settings, not just the image hash.
func ValidateAccepted(ctx context.Context, dir string, env *domain.EnvironmentSpec) error {
	if err := graph.ValidateEnvironment(env).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDefinition, err)
	}
	if env.Origin == nil {
		return nil
	}
	o := env.Origin
	resolved, err := Resolve(ctx, dir, Selection{Source: o.Source, Path: o.Path, SHA256: o.SHA256, Service: o.Service})
	if err != nil {
		return err
	}
	// JSON comparison treats omitted empty slices consistently after persistence.
	a, _ := json.Marshal(env)
	b, _ := json.Marshal(resolved)
	if !bytes.Equal(a, b) {
		return fmt.Errorf("%w: definition or referenced files changed after review", ErrInvalidDefinition)
	}
	return nil
}
