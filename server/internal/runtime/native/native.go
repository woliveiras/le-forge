// Package native is the built-in agent runtime: a tool-calling loop over
// any configured model provider. It needs no framework and no external
// process - a node with a prompt, a model and tool grants is a complete
// agent. Tools execute in the platform's gateway under the node's grants;
// the model never touches the host.
//
// Beyond granted tools the model gets four platform tools that need no
// grant because they only touch the agent's own turn: emit, state_get,
// state_set and finish. For models without native tool calling the runtime
// also honours text sentinels (NO FURTHER WORK:, CONFIRM STOP:, BROADCAST:,
// and EMIT {...} lines).
package native

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/llm"
	"github.com/woliveiras/le-forge/server/internal/runtime"
	"github.com/woliveiras/le-forge/server/internal/tools"
)

type Runtime struct {
	// MaxRounds bounds model round-trips per turn (default 40).
	MaxRounds int
}

func New() *Runtime { return &Runtime{MaxRounds: 40} }

func (r *Runtime) Kind() domain.RuntimeKind { return domain.RuntimeNative }

func (r *Runtime) Prepare(context.Context, runtime.ReplicaContext) error { return nil }
func (r *Runtime) Release(context.Context, runtime.ReplicaContext) error { return nil }

var platformTools = []llm.ToolSchema{
	{
		Name:        "emit",
		Description: "Send a message to downstream agents. The platform routes it by type, target and channel conditions.",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"type":{"type":"string","description":"message type, e.g. task.ready"},
			"payload":{"type":"object","description":"JSON object payload"},
			"target":{"type":"string","description":"optional node to send toward"},
			"channel":{"type":"string","description":"optional channel to force"}},
			"required":["type","payload"]}`),
	},
	{
		Name:        "state_get",
		Description: "Read a value from your durable memory (shared by replicas of this node).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}`),
	},
	{
		Name:        "state_set",
		Description: "Write a value to your durable memory for future turns.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"value":{"type":"string"}},"required":["key","value"]}`),
	},
	{
		Name:        "finish",
		Description: "End this turn. signal: \"\" (default), \"no_further_work\" (declare the job complete) or \"confirm_stop\" (confirm a previous declaration). broadcast: one line for ring neighbours.",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"summary":{"type":"string"},"signal":{"type":"string","enum":["","no_further_work","confirm_stop"]},
			"broadcast":{"type":"string"}}}`),
	},
}

func (r *Runtime) Turn(ctx context.Context, in *runtime.TurnInput, env runtime.TurnEnv) (*runtime.TurnResult, error) {
	res := &runtime.TurnResult{}
	toolset := append(append([]llm.ToolSchema(nil), in.Tools...), platformTools...)
	msgs := []llm.ChatMessage{{Role: llm.RoleUser, Content: in.Prompt}}
	var lastText string
	rounds := r.MaxRounds
	if rounds <= 0 {
		rounds = 40
	}
	for round := 0; round < rounds; round++ {
		resp, err := env.Chat(ctx, &llm.ChatRequest{System: in.System, Messages: msgs, Tools: toolset})
		if err != nil {
			return nil, fmt.Errorf("model call: %w", err)
		}
		res.Usage.InputTokens += resp.Usage.InputTokens
		res.Usage.OutputTokens += resp.Usage.OutputTokens
		msg := resp.Message
		msg.Role = llm.RoleAssistant
		msgs = append(msgs, msg)
		if t := strings.TrimSpace(msg.Content); t != "" {
			lastText = t
			env.Output(ctx, t)
		}
		if len(msg.ToolCalls) == 0 {
			break
		}
		finished := false
		for _, call := range msg.ToolCalls {
			out, done := r.call(ctx, env, res, call)
			msgs = append(msgs, llm.ChatMessage{Role: llm.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: out})
			finished = finished || done
		}
		if finished {
			break
		}
		if round == rounds-1 {
			env.Output(ctx, fmt.Sprintf("turn ended after %d model rounds", rounds))
		}
	}
	applySentinels(ctx, env, res, lastText)
	if res.Summary == "" {
		res.Summary = truncate(lastText, 1000)
	}
	return res, nil
}

// call executes one tool call; it returns the text for the model and
// whether the call ended the turn.
func (r *Runtime) call(ctx context.Context, env runtime.TurnEnv, res *runtime.TurnResult, call llm.ToolCall) (string, bool) {
	args := call.Arguments
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	switch call.Name {
	case "emit":
		var em runtime.Emit
		if err := json.Unmarshal(args, &em); err != nil {
			return "error: " + err.Error(), false
		}
		if err := env.Emit(ctx, em); err != nil {
			return "error: " + err.Error(), false
		}
		env.Output(ctx, "→ emit "+em.Type)
		return "queued for delivery when the turn succeeds", false
	case "state_get":
		var a struct{ Key string }
		_ = json.Unmarshal(args, &a)
		v, err := env.StateGet(ctx, a.Key)
		if err != nil {
			return "(not set)", false
		}
		return string(v), false
	case "state_set":
		var a struct{ Key, Value string }
		_ = json.Unmarshal(args, &a)
		if err := env.StatePut(ctx, a.Key, []byte(a.Value)); err != nil {
			return "error: " + err.Error(), false
		}
		return "ok", false
	case "finish":
		var a struct {
			Summary, Signal, Broadcast string
		}
		_ = json.Unmarshal(args, &a)
		res.Summary = a.Summary
		res.Signal = runtime.Signal(a.Signal)
		res.Broadcast = a.Broadcast
		return "turn finished", true
	}
	res.ToolCalls++
	env.Output(ctx, "→ "+call.Name+" "+truncate(string(args), 300))
	out, err := env.InvokeTool(ctx, call.Name, args)
	switch {
	case errors.Is(err, tools.ErrDenied), errors.Is(err, tools.ErrUnknownTool):
		return "permission denied: " + call.Name + " is not granted to you", false
	case errors.Is(err, tools.ErrRejected):
		return "a human rejected this call: " + err.Error(), false
	case err != nil:
		return "error: " + err.Error(), false
	}
	content := out.Content
	if out.IsError {
		content = "error: " + content
	}
	if out.Truncated {
		content += "\n…(output truncated)"
	}
	return content, false
}

var (
	reNoMore    = regexp.MustCompile(`(?im)^[\s*_#>]*NO FURTHER WORK:`)
	reConfirm   = regexp.MustCompile(`(?im)^[\s*_#>]*CONFIRM STOP:`)
	reBroadcast = regexp.MustCompile(`(?im)^[\s*_#>]*BROADCAST:\s*(.+)$`)
	reEmit      = regexp.MustCompile(`(?m)^EMIT\s+(\{.*\})\s*$`)
)

// applySentinels supports models without tool calling.
func applySentinels(ctx context.Context, env runtime.TurnEnv, res *runtime.TurnResult, text string) {
	if res.Signal == "" {
		switch {
		case reConfirm.MatchString(text):
			res.Signal = runtime.SignalConfirmStop
		case reNoMore.MatchString(text):
			res.Signal = runtime.SignalNoFurtherWork
		}
	}
	if res.Broadcast == "" {
		if m := reBroadcast.FindStringSubmatch(text); m != nil {
			res.Broadcast = strings.TrimSpace(m[1])
		}
	}
	for _, m := range reEmit.FindAllStringSubmatch(text, -1) {
		var em runtime.Emit
		if json.Unmarshal([]byte(m[1]), &em) == nil && em.Type != "" {
			_ = env.Emit(ctx, em)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
