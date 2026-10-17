package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/mcpconfig"
	"github.com/woliveiras/le-forge/server/internal/mcpconfig/mcptest"
)

func TestMCPOAuthBrowserAPI(t *testing.T) {
	srv, a := setupApp(t, nil)
	var callbackConfig map[string]string
	if code := call(t, srv, "GET", "/api/v1/mcp/oauth/config", viewerTok, "", "", &callbackConfig); code != 200 || callbackConfig["callbackUrl"] != "http://localhost:8080/api/v1/mcp/oauth/callback" {
		t.Fatal(code, callbackConfig)
	}
	provider := mcptest.NewOAuth("")
	defer provider.Close()
	upstream := mcptest.New()
	defer upstream.Close()
	a.MCP.OAuthClient = provider.Client()
	a.MCP.Discover = func(ctx context.Context, endpoint, token string) ([]mcpconfig.Tool, error) {
		return mcpconfig.DiscoverWithClient(ctx, upstream.URL, token, upstream.Client())
	}
	in := mcpconfig.Input{ID: "oauth", Name: "OAuth", Service: "generic", Endpoint: provider.URL + "/mcp", Enabled: true, OAuth: &mcpconfig.OAuthConfig{Issuer: provider.URL, ClientID: "client", Scopes: []string{"read"}}}
	body, _ := json.Marshal(in)
	if code := call(t, srv, "POST", "/api/v1/mcp/connections", adminTok, "application/json", string(body), nil); code != 201 {
		t.Fatal(code)
	}
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, path, token, body string) *http.Response {
		t.Helper()
		r, e := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		res, e := browser.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		return res
	}
	res := request("POST", "/api/v1/mcp/connections/oauth/oauth/start", viewerTok, "")
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
	res = request("POST", "/api/v1/mcp/connections/oauth/oauth/start", adminTok, "")
	var start mcpconfig.OAuthStart
	if err := json.NewDecoder(res.Body).Decode(&start); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	// The configured callback is independent of Host and contains no bearer credential.
	authorize, _ := url.Parse(start.URL)
	q := authorize.Query()
	if strings.Contains(start.URL, adminTok) || q.Get("redirect_uri") != "http://localhost:8080/api/v1/mcp/oauth/callback" {
		t.Fatal("wrong callback")
	}
	q.Set("decision", "allow")
	authorize.RawQuery = q.Encode()
	providerClient := provider.Client()
	providerClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	consent, e := providerClient.Get(authorize.String())
	if e != nil {
		t.Fatal(e)
	}
	consent.Body.Close()
	callback, _ := url.Parse(consent.Header.Get("Location"))
	res = request("GET", callback.RequestURI(), "", "")
	res.Body.Close()
	if res.StatusCode != 303 || strings.Contains(res.Header.Get("Location"), "code=") {
		t.Fatal("callback leaked code", res.StatusCode)
	}
	res = request("POST", "/api/v1/mcp/oauth/complete", operatorTok, `{"state":"`+start.State+`"}`)
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
	res = request("POST", "/api/v1/mcp/oauth/complete", adminTok, `{"state":"`+start.State+`"}`)
	raw, e := io.ReadAll(res.Body)
	res.Body.Close()
	if e != nil || res.StatusCode != 200 {
		t.Fatal(res.StatusCode, string(raw), e)
	}
	var connection mcpconfig.View
	if e = json.Unmarshal(raw, &connection); e != nil {
		t.Fatal(e)
	}
	if connection.Status != "ready" || connection.OAuth.Status != "authorized" || provider.Refreshes.Load() != 1 {
		t.Fatal(connection, provider.Refreshes.Load())
	}
	if strings.Contains(string(raw), "fixture-mcp-token") || strings.Contains(string(raw), "fixture-refresh-token") {
		t.Fatal("secret disclosure")
	}
	res = request("POST", "/api/v1/mcp/oauth/complete", adminTok, `{"state":"`+start.State+`"}`)
	res.Body.Close()
	if res.StatusCode != 422 {
		t.Fatal("callback replay", res.StatusCode)
	}
	res = request("POST", "/api/v1/mcp/connections/oauth/oauth/revoke", adminTok, "")
	raw, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	if e = json.Unmarshal(raw, &connection); e != nil {
		t.Fatal(e)
	}
	if connection.OAuth.Status != "revoked" || connection.CredentialConfigured || provider.Revocations.Load() != 2 {
		t.Fatal(connection)
	}
}
