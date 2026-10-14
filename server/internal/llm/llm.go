// Package llm is the model-provider abstraction. One interface, Provider,
// is everything a new backend must implement. The rest of the platform -
// native agents, and external agents through the LLM gateway endpoint -
// speaks only this vocabulary, so swapping Ollama for vLLM or Anthropic for
// OpenRouter is a config change, never a code change.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// Provider is implemented by every model backend.
type Provider interface {
	// Name is the configured instance name ("local-ollama"), not the type.
	Name() string
	// Chat performs one completion, optionally with tool definitions. A
	// response either carries text, tool calls, or both.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// Models lists models the backend offers (best effort; may be empty).
	Models(ctx context.Context) ([]ModelInfo, error)
}

// Role of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	// ToolCalls is set on assistant messages that request tool invocations.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID links a RoleTool message to the call it answers.
	ToolCallID string `json:"toolCallId,omitempty"`
	// Name of the tool for RoleTool messages.
	Name string `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"` // platform name, e.g. "filesystem.read"
	Arguments json.RawMessage `json:"arguments"`
}

// ToolSchema describes a tool to the model (JSON Schema for parameters).
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	System      string        `json:"system,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []ToolSchema  `json:"tools,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"maxTokens,omitempty"`
	// JSON asks for a JSON object response where the backend supports it.
	JSON bool `json:"json,omitempty"`
	// Stop sequences.
	Stop []string `json:"stop,omitempty"`
	// Metadata is passed through for logging/tracing, never to the model.
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ChatResponse struct {
	Message    ChatMessage `json:"message"`
	StopReason string      `json:"stopReason"` // "stop", "tool_calls", "length", ...
	Model      string      `json:"model"`
	Usage      Usage       `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	ContextSize int    `json:"contextSize,omitempty"`
}

// ProviderConfig is one entry of the "providers" list in platform config.
type ProviderConfig struct {
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"` // openai | anthropic | ollama | openrouter | huggingface | vllm | mock
	BaseURL string `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	// APIKeyEnv names the environment variable holding the key. Keys are
	// never written in config files and never forwarded to agents.
	APIKeyEnv string `json:"apiKeyEnv,omitempty" yaml:"apiKeyEnv,omitempty"`
	// APIKeyFile reads the key from a file (Docker/K8s secrets).
	APIKeyFile string `json:"apiKeyFile,omitempty" yaml:"apiKeyFile,omitempty"`
	// APIKeyValue is a key decrypted in memory from a credential the
	// operator stored through the console. It is never marshalled.
	APIKeyValue  string            `json:"-" yaml:"-"`
	DefaultModel string            `json:"defaultModel,omitempty" yaml:"defaultModel,omitempty"`
	Headers      map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	TimeoutSec   int               `json:"timeoutSec,omitempty" yaml:"timeoutSec,omitempty"`
	MaxRetries   int               `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	// Concurrency caps simultaneous requests to this backend (0 = unlimited).
	Concurrency int `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	// ContextWindow is the context size in tokens the backend should use for
	// a request (prompt plus answer). Providers that let the client choose it
	// (Ollama's num_ctx) send it; 0 means the provider's default. Prompts that
	// do not fit fail with ErrContextExceeded instead of being truncated.
	ContextWindow int `json:"contextWindow,omitempty" yaml:"contextWindow,omitempty"`
}

// ErrContextExceeded: a request does not fit the model's context window.
// Backends that truncate silently (Ollama) would otherwise drop the start of
// the conversation, including the task.
var ErrContextExceeded = errors.New("prompt exceeds the model context window")

// EstimateTokens is a deliberately rough count (about 4 bytes per token)
// used to catch prompts that cannot fit before sending them.
func EstimateTokens(req *ChatRequest) int {
	n := len(req.System)
	for _, m := range req.Messages {
		n += len(m.Content) + len(m.Name) + 8
		for _, tc := range m.ToolCalls {
			n += len(tc.Name) + len(tc.Arguments)
		}
	}
	for _, t := range req.Tools {
		n += len(t.Name) + len(t.Description) + len(t.Parameters)
	}
	return n / 4
}

// Factory builds a Provider from its config. Register one per type.
type Factory func(cfg ProviderConfig) (Provider, error)

var factories = map[string]Factory{}

// RegisterType makes a provider type available to config. Adding a backend
// is: implement Provider, call RegisterType in an init().
func RegisterType(typ string, f Factory) { factories[strings.ToLower(typ)] = f }

// Types lists registered provider types.
func Types() []string {
	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	return out
}

// New builds a provider from config.
func New(cfg ProviderConfig) (Provider, error) {
	f, ok := factories[strings.ToLower(cfg.Type)]
	if !ok {
		return nil, errors.New("llm: unknown provider type " + cfg.Type)
	}
	return f(cfg)
}

// WireToolName maps platform tool names ("filesystem.read") to the
// character set every vendor accepts for function names (^[a-zA-Z0-9_-]+$).
func WireToolName(name string) string { return strings.ReplaceAll(name, ".", "__") }

// PlatformToolName reverses WireToolName.
func PlatformToolName(wire string) string { return strings.ReplaceAll(wire, "__", ".") }
