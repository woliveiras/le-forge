package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/woliveiras/le-forge/server/internal/tools"
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
		t.Fatal("dynamic MCP tools must not be cached in the static registry")
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

func TestMCPGatewayWithTLSAndPersistedRegistry(t *testing.T) {
	_, a := setupApp(t, nil)
	upstream := mcptest.New()
	defer upstream.Close()
	a.MCP.Discover = func(ctx context.Context, endpoint, token string) ([]mcpconfig.Tool, error) {
		return mcpconfig.DiscoverWithClient(ctx, endpoint, token, upstream.Client())
	}
	a.MCP.Call = func(ctx context.Context, endpoint, token, name string, args json.RawMessage) (json.RawMessage, error) {
		return mcpconfig.CallWithClient(ctx, endpoint, token, name, args, upstream.Client())
	}
	in := mcpconfig.Input{ID: "remote", Name: "Remote", Service: "generic", Endpoint: upstream.URL, Enabled: true, Credential: &mcpconfig.Credential{Value: "fixture-mcp-token"}}
	if _, err := a.MCP.Put(t.Context(), in, 0, "admin"); err != nil {
		t.Fatal(err)
	}
	v, err := a.MCP.Refresh(t.Context(), in.ID, "admin")
	if err != nil {
		t.Fatal(err)
	}
	p := &tools.Principal{GraphID: "graph", NodeID: "allowed", Grants: []domain.ToolGrant{{Name: "mcp.remote.search"}}}
	result, err := a.Gateway.Invoke(t.Context(), p, "mcp.remote.search", json.RawMessage(`{"query":"issue"}`))
	if err != nil || result.IsError || !strings.Contains(result.Content, "MCP fixture search completed") {
		t.Fatal(result, err)
	}
	other := &tools.Principal{GraphID: "graph", NodeID: "other", Grants: []domain.ToolGrant{{Name: "*"}}}
	if _, err = a.Gateway.Invoke(t.Context(), other, "mcp.remote.search", nil); !errors.Is(err, tools.ErrDenied) {
		t.Fatal(err)
	}
	in.Enabled = false
	in.Credential = nil
	if _, err = a.MCP.Put(t.Context(), in, v.Revision, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Gateway.Invoke(t.Context(), p, "mcp.remote.search", nil); !errors.Is(err, tools.ErrDenied) {
		t.Fatal(err)
	}
}

func TestMCPVersionedTemplatesPersistence(t *testing.T) {
	srv, a := setupApp(t, nil)
	var catalogue []mcpconfig.Template
	if code := call(t, srv, "GET", "/api/v1/mcp/templates", viewerTok, "", "", &catalogue); code != 200 || len(catalogue) != 5 {
		t.Fatal(code, catalogue)
	}
	template := catalogue[0]
	in := mcpconfig.Input{ID: "from-template", Name: "From template", Service: template.Service, Endpoint: template.Endpoint, Enabled: true, Template: &template.TemplateRef}
	body, _ := json.Marshal(in)
	if code := call(t, srv, "POST", "/api/v1/mcp/connections", operatorTok, "application/json", string(body), nil); code != 403 {
		t.Fatal(code)
	}
	var saved mcpconfig.View
	if code := call(t, srv, "POST", "/api/v1/mcp/connections", adminTok, "application/json", string(body), &saved); code != 201 {
		t.Fatal(code)
	}
	loaded, err := a.MCP.Get(t.Context(), in.ID)
	if err != nil || loaded.Template == nil || *loaded.Template != template.TemplateRef || loaded.Status != "untested" || len(loaded.Tools) != 0 {
		t.Fatal(loaded, err)
	}
	var read mcpconfig.View
	if code := call(t, srv, "GET", "/api/v1/mcp/connections/"+in.ID, viewerTok, "", "", &read); code != 200 || read.Template == nil || *read.Template != template.TemplateRef {
		t.Fatal(code, read)
	}
	for _, bad := range []mcpconfig.Input{
		{ID: "bad-version", Name: "Bad", Service: template.Service, Endpoint: template.Endpoint, Template: &mcpconfig.TemplateRef{ID: template.ID, Version: 999}},
		{ID: "bad-endpoint", Name: "Bad", Service: template.Service, Endpoint: "https://other.example/mcp", Template: &template.TemplateRef},
	} {
		body, _ = json.Marshal(bad)
		if code := call(t, srv, "POST", "/api/v1/mcp/connections", adminTok, "application/json", string(body), nil); code != 422 {
			t.Fatal(code)
		}
		if _, err := a.MCP.Get(t.Context(), bad.ID); err == nil {
			t.Fatal("invalid template persisted")
		}
	}
}
