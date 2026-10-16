package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/mcpconfig"
	"github.com/woliveiras/le-forge/server/internal/mcpconfig/mcptest"
)

func TestMCPRegistryDiscoveryAndPermissions(t *testing.T) {
	srv, a := setupApp(t, nil)
	upstream := mcptest.New()
	defer upstream.Close()
	a.MCP.Discover = func(ctx context.Context, endpoint, token string) ([]mcpconfig.Tool, error) {
		return mcpconfig.DiscoverWithClient(ctx, endpoint, token, upstream.Client())
	}
	in := mcpconfig.Input{ID: "team", Name: "Team", Service: "github", Endpoint: upstream.URL, Enabled: true, Credential: &mcpconfig.Credential{Value: "fixture-mcp-token"}}
	body, _ := json.Marshal(in)
	path := "/api/v1/mcp/connections"
	var v mcpconfig.View
	if code := call(t, srv, "POST", path, viewerTok, "application/json", string(body), nil); code != 403 {
		t.Fatal(code)
	}
	if code := call(t, srv, "POST", path, adminTok, "application/json", string(body), &v); code != 201 || !v.CredentialConfigured {
		t.Fatal(code, v)
	}
	if code := call(t, srv, "POST", path+"/team/tools/refresh", viewerTok, "", "", nil); code != 403 {
		t.Fatal(code)
	}
	if code := call(t, srv, "POST", path+"/team/tools/refresh", operatorTok, "", "", &v); code != 200 || v.Status != "ready" || len(v.Tools) != 2 {
		t.Fatal(code, v)
	}
	var safe any
	if code := call(t, srv, "GET", path, viewerTok, "", "", &safe); code != 200 {
		t.Fatal(code)
	}
	b, _ := json.Marshal(safe)
	if strings.Contains(string(b), "fixture-mcp-token") || strings.Contains(string(b), "sealed") {
		t.Fatal("secret disclosed")
	}
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "mcp-graph", Name: "MCP"}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "worker", Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}, Tools: []domain.ToolGrant{{Name: "mcp.team.search"}}}}}}
	gb, _ := json.Marshal(g)
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", string(gb), nil); code != 201 {
		t.Fatal(code)
	}
	saved, _, e := a.Engine.GetGraphDocument(t.Context(), "mcp-graph", 0)
	if e != nil || saved.Spec.Nodes[0].Tools[0].Name != "mcp.team.search" {
		t.Fatal(saved, e)
	}
	if _, ok := a.Tools.Get("mcp.team.search"); ok {
		t.Fatal("discovery must not enable invocation")
	}
	in.Enabled = false
	in.Credential = nil
	body, _ = json.Marshal(in)
	put := func(rev int64) int {
		req, _ := http.NewRequest("PUT", srv.URL+path+"/team", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("If-Match", fmt.Sprint(rev))
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if code := put(1); code != 409 {
		t.Fatal("stale write", code)
	}
	if code := put(v.Revision); code != 200 {
		t.Fatal(code)
	}
	g.Metadata.ID = "blocked"
	gb, _ = json.Marshal(g)
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", string(gb), nil); code != 422 {
		t.Fatal("disabled grant", code)
	}
	var catalogue []any
	call(t, srv, "GET", "/api/v1/mcp/tools", viewerTok, "", "", &catalogue)
	if len(catalogue) != 0 {
		t.Fatal(catalogue)
	}
	var report struct{ Valid bool }
	if code := call(t, srv, "POST", "/api/v1/graphs/validate", adminTok, "application/json", string(gb), &report); code != 200 || report.Valid {
		t.Fatal(code, report)
	}
}
func TestMCPInputAndUpstreamFailures(t *testing.T) {
	srv, a := setupApp(t, nil)
	upstream := mcptest.New()
	defer upstream.Close()
	a.MCP.Discover = func(ctx context.Context, endpoint, token string) ([]mcpconfig.Tool, error) {
		return mcpconfig.DiscoverWithClient(ctx, endpoint, token, upstream.Client())
	}
	for _, body := range []string{`{}`, `{"id":"bad","credential":{"unexpected":"secret"}}`, `{} {}`, `{"id":"team","name":"Team","service":"generic","endpoint":"http://localhost","enabled":true}`} {
		if code := call(t, srv, "POST", "/api/v1/mcp/connections", adminTok, "application/json", body, nil); code != 400 && code != 422 {
			t.Fatal(code)
		}
	}
	body := fmt.Sprintf(`{"id":"noauth","name":"No auth","service":"generic","endpoint":%q,"enabled":true}`, upstream.URL)
	if code := call(t, srv, "POST", "/api/v1/mcp/connections", adminTok, "application/json", body, nil); code != 201 {
		t.Fatal(code)
	}
	var v mcpconfig.View
	if code := call(t, srv, "POST", "/api/v1/mcp/connections/noauth/test", operatorTok, "", "", &v); code != 200 || v.Status != "unavailable" || len(v.Tools) != 0 {
		t.Fatal(code, v)
	}
	if code := call(t, srv, "PUT", "/api/v1/mcp/connections/noauth", adminTok, "application/json", body, nil); code != 400 {
		t.Fatal("missing revision", code)
	}
}
