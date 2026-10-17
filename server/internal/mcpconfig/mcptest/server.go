// Package mcptest supplies a real TLS MCP fixture for integration/browser tests.
package mcptest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
)

func New() *httptest.Server { return newServer(false) }

// NewRecovering expires every first session before dispatch to exercise recovery.
func NewRecovering() *httptest.Server { return newServer(true) }
func newServer(recovering bool) *httptest.Server {
	var sessions atomic.Int64
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-mcp-token" {
			http.Error(w, "fixture authentication required", 401)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(204)
			return
		}
		var q struct {
			ID     int            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			w.WriteHeader(400)
			return
		}
		var result any
		switch q.Method {
		case "initialize":
			session := "fixture-session"
			if recovering {
				session = strconv.FormatInt(sessions.Add(1), 10)
			}
			w.Header().Set("Mcp-Session-Id", session)
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}}
		case "notifications/initialized":
			if recovering {
				n, _ := strconv.Atoi(r.Header.Get("Mcp-Session-Id"))
				if n%2 == 1 {
					w.WriteHeader(404)
					return
				}
			}
			w.WriteHeader(202)
			return
		case "tools/call":
			if (!recovering && r.Header.Get("Mcp-Session-Id") != "fixture-session") || r.Header.Get("MCP-Protocol-Version") != "2025-06-18" {
				w.WriteHeader(400)
				return
			}
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": "MCP fixture search completed"}}}
		case "tools/list":
			if (!recovering && r.Header.Get("Mcp-Session-Id") != "fixture-session") || r.Header.Get("MCP-Protocol-Version") != "2025-06-18" {
				w.WriteHeader(400)
				return
			}
			if q.Params["cursor"] == nil {
				result = map[string]any{"tools": []any{map[string]any{"name": "search", "description": "Search fixture issues", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]string{"type": "string"}}}}}, "nextCursor": "second"}
			} else {
				result = map[string]any{"tools": []any{map[string]any{"name": "read_issue", "description": "Read fixture issue", "inputSchema": map[string]string{"type": "object"}}}}
			}
		default:
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}))
}
