package mcpconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamableDiscovery(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(fmt.Sprint(sse), func(t *testing.T) {
			initialized := false
			deleted := false
			pages := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Error("missing auth")
				}
				if r.Method == "DELETE" {
					deleted = true
					w.WriteHeader(204)
					return
				}
				var req struct {
					ID     int            `json:"id"`
					Method string         `json:"method"`
					Params map[string]any `json:"params"`
				}
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					t.Error("invalid request")
					return
				}
				if req.Method != "initialize" && (r.Header.Get("Mcp-Session-Id") != "session" || r.Header.Get("MCP-Protocol-Version") != protocol) {
					t.Error("missing negotiated headers")
				}
				var result any
				switch req.Method {
				case "initialize":
					w.Header().Set("Mcp-Session-Id", "session")
					result = map[string]any{"protocolVersion": protocol, "capabilities": map[string]any{"tools": map[string]any{}}}
				case "notifications/initialized":
					initialized = true
					w.WriteHeader(202)
					return
				case "tools/list":
					if !initialized {
						t.Error("not initialized")
					}
					pages++
					if pages == 1 {
						result = map[string]any{"tools": []any{map[string]any{"name": "search", "inputSchema": map[string]string{"type": "object"}}}, "nextCursor": "next"}
					} else {
						if req.Params["cursor"] != "next" {
							t.Error("missing cursor")
						}
						result = map[string]any{"tools": []any{}}
					}
				}
				body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
				if sse {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\n", body)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.Write(body)
				}
			}))
			defer server.Close()
			tools, e := DiscoverWithClient(t.Context(), server.URL, "synthetic-token", server.Client())
			if e != nil || len(tools) != 1 || pages != 2 || !deleted {
				t.Fatal(tools, e, pages, deleted)
			}
		})
	}
}
func TestTransportRejectsBadResponsesAndNetwork(t *testing.T) {
	for _, body := range []string{`not-json`, `{"jsonrpc":"2.0","id":2,"result":{}}`, `{"jsonrpc":"2.0","id":1,"error":{"message":"secret"}}`, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"bad"}}`} {
		t.Run(body, func(t *testing.T) {
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
			}))
			defer s.Close()
			if _, e := DiscoverWithClient(t.Context(), s.URL, "", s.Client()); e == nil {
				t.Fatal("bad response accepted")
			}
		})
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, e := Discover(ctx, "https://127.0.0.1/mcp", ""); e == nil {
		t.Fatal("private endpoint accepted")
	}
}

func TestCatalogueBoundsAndRedaction(t *testing.T) {
	for _, mode := range []string{"duplicate", "bad-schema", "bad-name", "cursor-loop", "token-echo", "too-many", "oversized", "http-error"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     int    `json:"id"`
					Method string `json:"method"`
				}
				json.NewDecoder(r.Body).Decode(&req)
				if req.Method == "notifications/initialized" {
					w.WriteHeader(202)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				var result any = map[string]any{"protocolVersion": protocol, "capabilities": map[string]any{"tools": map[string]any{}}}
				if req.Method == "tools/list" {
					if mode == "http-error" {
						w.WriteHeader(503)
						fmt.Fprint(w, "synthetic-token")
						return
					}
					tool := Tool{Name: "search", InputSchema: json.RawMessage(`{"type":"object"}`)}
					catalog := []Tool{tool}
					cursor := ""
					switch mode {
					case "duplicate":
						catalog = append(catalog, tool)
					case "bad-schema":
						catalog[0].InputSchema = json.RawMessage(`{"type":"array"}`)
					case "bad-name":
						catalog[0].Name = "../search"
					case "cursor-loop":
						catalog = []Tool{}
						cursor = "same"
					case "token-echo":
						catalog[0].Description = "synthetic-token"
					case "too-many":
						catalog = make([]Tool, 501)
						for i := range catalog {
							catalog[i] = tool
							catalog[i].Name = fmt.Sprintf("tool%d", i)
						}
					case "oversized":
						catalog[0].Description = strings.Repeat("x", maxBody+1)
					}
					result = map[string]any{"tools": catalog, "nextCursor": cursor}
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer server.Close()
			if _, e := DiscoverWithClient(t.Context(), server.URL, "synthetic-token", server.Client()); e == nil || strings.Contains(e.Error(), "synthetic-token") {
				t.Fatal("unsafe result", e)
			}
		})
	}
}
func TestDiscoveryCancellation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if _, e := DiscoverWithClient(ctx, server.URL, "", server.Client()); e == nil {
		t.Fatal("cancelled request succeeded")
	}
}
