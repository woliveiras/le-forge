// Package config loads platform configuration from a YAML file and the
// environment. Precedence: built-in defaults < config file < environment.
// Secrets are never read from the file directly - the file names the
// environment variable or mounted file that holds them.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/woliveiras/le-forge/server/internal/llm"
)

type Config struct {
	Instance      string        `yaml:"instance"`
	Server        Server        `yaml:"server"`
	Auth          Auth          `yaml:"auth"`
	Storage       Storage       `yaml:"storage"`
	Queue         Queue         `yaml:"queue"`
	Workspaces    Workspaces    `yaml:"workspaces"`
	LLM           LLM           `yaml:"llm"`
	Secrets       Secrets       `yaml:"secrets"`
	Runtimes      Runtimes      `yaml:"runtimes"`
	Isolation     Isolation     `yaml:"isolation"`
	Tools         Tools         `yaml:"tools"`
	Observability Observability `yaml:"observability"`
	Engine        Engine        `yaml:"engine"`
	// Graphs are loaded (created or updated) at startup from these files or
	// directories - GitOps-style bootstrapping of example or production graphs.
	Bootstrap Bootstrap `yaml:"bootstrap"`
}

type Server struct {
	Addr string `yaml:"addr"`
	// InternalURL is how runtimes reach the callback API (http://forged:8080).
	InternalURL string `yaml:"internalUrl"`
	// InternalAddr, when set (":8081"), serves the runtime callback API
	// (/internal/v1) on its own listener and removes it from Addr, so public
	// ingress can never reach it. InternalURL must then point at this port.
	InternalAddr string `yaml:"internalAddr"`
	// WebDir serves the built UI when set.
	WebDir      string   `yaml:"webDir"`
	CORSOrigins []string `yaml:"corsOrigins"`
	// MaxBodyBytes bounds request bodies (default 8 MiB).
	MaxBodyBytes int64 `yaml:"maxBodyBytes"`
	// ProtectMetrics requires a viewer token on /metrics (scrape with a
	// bearer token). Off by default so a stock Prometheus can scrape.
	ProtectMetrics bool `yaml:"protectMetrics"`
}

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

func (r Role) Allows(need Role) bool {
	rank := map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleAdmin: 3}
	return rank[r] >= rank[need]
}

type Token struct {
	Name      string `yaml:"name"`
	Role      Role   `yaml:"role"`
	TokenEnv  string `yaml:"tokenEnv"`
	TokenFile string `yaml:"tokenFile"`
	// Token inline is accepted only for local development; a warning is logged.
	Token string `yaml:"token"`
}

type Auth struct {
	// Disabled turns authentication off entirely (loopback-only dev use).
	Disabled bool    `yaml:"disabled"`
	Tokens   []Token `yaml:"tokens"`
	// OIDC (optional): JWTs issued by this issuer are accepted; the role is
	// taken from RoleClaim. Verification uses the issuer's JWKS.
	OIDC *OIDC `yaml:"oidc"`
}

type OIDC struct {
	Issuer    string `yaml:"issuer"`
	Audience  string `yaml:"audience"`
	RoleClaim string `yaml:"roleClaim"`
}

type Storage struct {
	Type string `yaml:"type"` // fs | sqlite | postgres
	DSN  string `yaml:"dsn"`
	Dir  string `yaml:"dir"`
}

type Queue struct {
	Type    string            `yaml:"type"` // fs | nats | redis | rabbitmq | kafka | memory
	URL     string            `yaml:"url"`
	Dir     string            `yaml:"dir"`
	Options map[string]string `yaml:"options"`
}

type Workspaces struct {
	// Root holds one directory per graph (unless the graph names a path).
	Root string `yaml:"root"`
	// HostRoot is the same directory as seen by the Docker daemon, needed to
	// bind-mount workspaces into sandbox containers when the platform itself
	// runs in a container. Empty = Root.
	HostRoot string `yaml:"hostRoot"`
	// AllowAbsolute lets graphs point at arbitrary absolute paths.
	AllowAbsolute bool `yaml:"allowAbsolute"`
}

type LLM struct {
	Default   string               `yaml:"default"`
	Providers []llm.ProviderConfig `yaml:"providers"`
}

// Secrets configures the installation key that encrypts credentials
// operators enter in the console (model provider keys, integration tokens).
type Secrets struct {
	// KeyEnv names the environment variable holding the key (base64 or hex,
	// 32 bytes). It takes precedence over KeyFile.
	KeyEnv string `yaml:"keyEnv"`
	// KeyFile holds the key when KeyEnv is unset or empty.
	KeyFile string `yaml:"keyFile"`
	// GenerateKeyFile creates KeyFile with a random key on first start when
	// no key is configured, so a fresh install can store credentials from
	// the browser. Back the file up: without it, stored credentials must be
	// entered again.
	GenerateKeyFile bool `yaml:"generateKeyFile"`
	// PreviousKeysEnv names a variable with comma-separated older keys that
	// still decrypt (key rotation); values are re-encrypted with the
	// current key at startup.
	PreviousKeysEnv string `yaml:"previousKeysEnv"`
}

type Runtimes struct {
	// External turns are bounded by the node's limits.turnTimeout.
	// SharedSecret is sent to external runtimes as X-Leforge-Runtime-Secret.
	SharedSecretEnv string `yaml:"sharedSecretEnv"`
}

type Isolation struct {
	Docker DockerIsolation `yaml:"docker"`
}

type DockerIsolation struct {
	Bin     string `yaml:"bin"`
	Image   string `yaml:"image"`
	Network string `yaml:"network"`
}

type Tools struct {
	AllowPrivateNetworks bool `yaml:"allowPrivateNetworks"`
}

type Observability struct {
	ServiceName  string `yaml:"serviceName"`
	OTLPEndpoint string `yaml:"otlpEndpoint"` // http://otel-collector:4318
	LogLevel     string `yaml:"logLevel"`
	LogFormat    string `yaml:"logFormat"` // json | text
	GrafanaURL   string `yaml:"grafanaUrl"`
	// TraceSampleRatio in [0,1] (default 1).
	TraceSampleRatio float64 `yaml:"traceSampleRatio"`
}

type Engine struct {
	LeaseTTL time.Duration `yaml:"leaseTTL"`
	// ReconcileInterval drives autoscaling, lease renewal and failover.
	ReconcileInterval time.Duration `yaml:"reconcileInterval"`
	// MaxConcurrentTurns caps turns across all graphs on this instance (0 = unlimited).
	MaxConcurrentTurns int `yaml:"maxConcurrentTurns"`
	// RetryBackoff is the base delay for nacked inputs (doubles per attempt).
	RetryBackoff time.Duration `yaml:"retryBackoff"`
}

type Bootstrap struct {
	Graphs []string `yaml:"graphs"`
	// Start lists graph ids to start after bootstrapping.
	Start []string `yaml:"start"`
}

// Default returns a configuration that runs on a laptop with no services:
// SQLite storage, filesystem queues, the mock model provider.
func Default() *Config {
	host, _ := os.Hostname()
	return &Config{
		Instance: host,
		Server: Server{
			Addr:         ":8080",
			InternalURL:  "http://127.0.0.1:8080",
			MaxBodyBytes: 8 << 20,
		},
		Storage:    Storage{Type: "sqlite", DSN: "file:data/leforge.db"},
		Queue:      Queue{Type: "fs", Dir: "data/queues"},
		Workspaces: Workspaces{Root: "data/workspaces"},
		LLM: LLM{
			Default:   "mock",
			Providers: []llm.ProviderConfig{{Name: "mock", Type: "mock", DefaultModel: "mock-1"}},
		},
		Secrets: Secrets{
			KeyEnv: "LEFORGE_SECRET_KEY", KeyFile: "data/secret.key", GenerateKeyFile: true,
			PreviousKeysEnv: "LEFORGE_SECRET_KEY_PREVIOUS",
		},
		Isolation: Isolation{
			Docker: DockerIsolation{Bin: "docker", Image: "leforge/sandbox:latest", Network: "none"},
		},
		Observability: Observability{ServiceName: "leforge", LogLevel: "info", LogFormat: "json", TraceSampleRatio: 1},
		Engine: Engine{
			LeaseTTL:          20 * time.Second,
			ReconcileInterval: 2 * time.Second,
			RetryBackoff:      2 * time.Second,
		},
	}
}

// Load reads path (if non-empty and present) and applies environment overrides.
func Load(path string) (*Config, error) {
	c := Default()
	if path == "" {
		path = os.Getenv("LEFORGE_CONFIG")
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		b = []byte(os.ExpandEnv(string(b)))
		if err := yaml.Unmarshal(b, c); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	applyEnv(c)
	return c, c.Validate()
}

func applyEnv(c *Config) {
	str := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			*dst = v
		}
	}
	str("LEFORGE_INSTANCE", &c.Instance)
	str("LEFORGE_ADDR", &c.Server.Addr)
	str("LEFORGE_INTERNAL_URL", &c.Server.InternalURL)
	str("LEFORGE_INTERNAL_ADDR", &c.Server.InternalAddr)
	str("LEFORGE_WEB_DIR", &c.Server.WebDir)
	str("LEFORGE_STORAGE_TYPE", &c.Storage.Type)
	str("LEFORGE_STORAGE_DSN", &c.Storage.DSN)
	str("LEFORGE_STORAGE_DIR", &c.Storage.Dir)
	str("LEFORGE_QUEUE_TYPE", &c.Queue.Type)
	str("LEFORGE_QUEUE_URL", &c.Queue.URL)
	str("LEFORGE_QUEUE_DIR", &c.Queue.Dir)
	str("LEFORGE_WORKSPACES", &c.Workspaces.Root)
	str("LEFORGE_WORKSPACES_HOST", &c.Workspaces.HostRoot)
	str("LEFORGE_DEFAULT_PROVIDER", &c.LLM.Default)
	str("LEFORGE_SANDBOX_IMAGE", &c.Isolation.Docker.Image)
	str("LEFORGE_SECRET_KEY_FILE", &c.Secrets.KeyFile)
	if v := os.Getenv("LEFORGE_SECRET_KEY_GENERATE"); v != "" {
		c.Secrets.GenerateKeyFile, _ = strconv.ParseBool(v)
	}
	str("LEFORGE_LOG_LEVEL", &c.Observability.LogLevel)
	str("LEFORGE_LOG_FORMAT", &c.Observability.LogFormat)
	str("LEFORGE_GRAFANA_URL", &c.Observability.GrafanaURL)
	str("OTEL_EXPORTER_OTLP_ENDPOINT", &c.Observability.OTLPEndpoint)
	str("OTEL_SERVICE_NAME", &c.Observability.ServiceName)
	if v := os.Getenv("LEFORGE_PROTECT_METRICS"); v != "" {
		c.Server.ProtectMetrics, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("LEFORGE_AUTH_DISABLED"); v != "" {
		c.Auth.Disabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("LEFORGE_BOOTSTRAP_GRAPHS"); v != "" {
		c.Bootstrap.Graphs = splitList(v)
	}
	if v := os.Getenv("LEFORGE_BOOTSTRAP_START"); v != "" {
		c.Bootstrap.Start = splitList(v)
	}
	// LEFORGE_ADMIN_TOKEN is the quickest way to secure a deployment: one
	// admin token from the environment, no config file needed.
	if os.Getenv("LEFORGE_ADMIN_TOKEN") != "" {
		c.Auth.Tokens = append(c.Auth.Tokens, Token{Name: "admin", Role: RoleAdmin, TokenEnv: "LEFORGE_ADMIN_TOKEN"})
	}
	// Convenience provider definitions from well-known env vars, so a single
	// variable enables a backend without editing config.
	addProv := func(name, typ, baseURL, keyEnv, model string) {
		for _, p := range c.LLM.Providers {
			if p.Name == name {
				return
			}
		}
		c.LLM.Providers = append(c.LLM.Providers, llm.ProviderConfig{Name: name, Type: typ, BaseURL: baseURL, APIKeyEnv: keyEnv, DefaultModel: model})
	}
	if u := os.Getenv("OLLAMA_BASE_URL"); u != "" {
		addProv("ollama", "ollama", u, "", envOr("OLLAMA_MODEL", "llama3.2"))
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		addProv("openai", "openai", envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"), "OPENAI_API_KEY", envOr("OPENAI_MODEL", "gpt-4o-mini"))
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		addProv("anthropic", "anthropic", envOr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"), "ANTHROPIC_API_KEY", envOr("ANTHROPIC_MODEL", "claude-sonnet-5-5"))
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		addProv("openrouter", "openrouter", "https://openrouter.ai/api/v1", "OPENROUTER_API_KEY", envOr("OPENROUTER_MODEL", "meta-llama/llama-3.3-70b-instruct"))
	}
	if u := os.Getenv("VLLM_BASE_URL"); u != "" {
		addProv("vllm", "vllm", u, "", os.Getenv("VLLM_MODEL"))
	}
	if u := os.Getenv("HF_BASE_URL"); u != "" {
		// Local TGI needs no key; hosted HF endpoints do.
		keyEnv := ""
		if os.Getenv("HF_API_KEY") != "" {
			keyEnv = "HF_API_KEY"
		}
		addProv("huggingface", "huggingface", u, keyEnv, envOr("HF_MODEL", "tgi"))
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) Validate() error {
	if c.Instance == "" {
		c.Instance = "leforge-0"
	}
	if c.Engine.LeaseTTL < 3*time.Second {
		return fmt.Errorf("engine.leaseTTL must be >= 3s")
	}
	if c.Engine.ReconcileInterval <= 0 || c.Engine.ReconcileInterval > c.Engine.LeaseTTL/2 {
		c.Engine.ReconcileInterval = c.Engine.LeaseTTL / 4
	}
	if c.Engine.RetryBackoff <= 0 {
		c.Engine.RetryBackoff = 2 * time.Second
	}
	if c.Server.MaxBodyBytes <= 0 {
		c.Server.MaxBodyBytes = 8 << 20
	}
	seen := map[string]bool{}
	for _, p := range c.LLM.Providers {
		if p.Name == "" || p.Type == "" {
			return fmt.Errorf("llm provider needs name and type")
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate llm provider %q", p.Name)
		}
		seen[p.Name] = true
	}
	if c.LLM.Default != "" && !seen[c.LLM.Default] {
		return fmt.Errorf("llm.default %q is not a configured provider", c.LLM.Default)
	}
	if !c.Auth.Disabled && len(c.Auth.Tokens) == 0 && c.Auth.OIDC == nil {
		return fmt.Errorf("no API credentials configured: set LEFORGE_ADMIN_TOKEN, auth.tokens, auth.oidc, or LEFORGE_AUTH_DISABLED=true for local development")
	}
	for _, t := range c.Auth.Tokens {
		switch t.Role {
		case RoleViewer, RoleOperator, RoleAdmin:
		default:
			return fmt.Errorf("auth token %q has invalid role %q", t.Name, t.Role)
		}
	}
	return nil
}

// ResolveToken returns the secret value of a token definition.
func (t Token) Resolve() (string, error) {
	switch {
	case t.TokenEnv != "":
		v := os.Getenv(t.TokenEnv)
		if v == "" {
			return "", fmt.Errorf("token %q: env %s is empty", t.Name, t.TokenEnv)
		}
		return v, nil
	case t.TokenFile != "":
		b, err := os.ReadFile(t.TokenFile)
		if err != nil {
			return "", fmt.Errorf("token %q: %w", t.Name, err)
		}
		return strings.TrimSpace(string(b)), nil
	case t.Token != "":
		return t.Token, nil
	}
	return "", fmt.Errorf("token %q has no value", t.Name)
}
