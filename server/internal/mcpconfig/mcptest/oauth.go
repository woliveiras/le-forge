package mcptest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
)

// OAuthServer is a real TLS identity-provider fixture, not a production issuer.
// It validates the PKCE verifier, redirect URI and resource at code exchange.
type OAuthServer struct {
	*httptest.Server
	mu          sync.Mutex
	codes       map[string]url.Values
	Refreshes   atomic.Int32
	Revocations atomic.Int32
}

func NewOAuth(resource string) *OAuthServer { return newOAuth(resource, false) }

// NewGitHubOAuth reproduces GitHub.com metadata and token wire formats locally.
func NewGitHubOAuth() *OAuthServer { return newOAuth("https://api.githubcopilot.com/mcp/", true) }

func newOAuth(resource string, github bool) *OAuthServer {
	f := &OAuthServer{codes: map[string]url.Values{}}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server/login/oauth":
			if !github {
				w.WriteHeader(404)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"issuer": "https://github.com/login/oauth", "authorization_endpoint": "https://github.com/login/oauth/authorize", "token_endpoint": "https://github.com/login/oauth/access_token", "code_challenge_methods_supported": []string{"S256"}})
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token", "revocation_endpoint": f.URL + "/revoke", "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic"}})
		case "/.well-known/oauth-protected-resource/mcp/":
			json.NewEncoder(w).Encode(map[string]any{"resource": resource, "authorization_servers": []string{"https://github.com/login/oauth"}})
		case "/.well-known/oauth-protected-resource/mcp", "/resource":
			json.NewEncoder(w).Encode(map[string]any{"resource": resource, "authorization_servers": []string{f.URL}})
		case "/mcp/":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://api.githubcopilot.com/.well-known/oauth-protected-resource/mcp/"`)
			w.WriteHeader(401)
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+f.URL+`/resource"`)
			w.WriteHeader(401)
		case "/authorize", "/login/oauth/authorize":
			q := r.URL.Query()
			if q.Get("code_challenge_method") != "S256" || q.Get("resource") != resource {
				http.Error(w, "invalid authorization", 400)
				return
			}
			if q.Get("decision") == "" {
				w.Header().Set("Content-Type", "text/html")
				form := template.Must(template.New("consent").Parse(`<h1>Fixture OAuth consent</h1><p>Requested scopes: {{.Get "scope"}}</p><form method="get">{{range $k,$v:=.}}{{range $v}}<input type="hidden" name="{{$k}}" value="{{.}}">{{end}}{{end}}<button name="decision" value="allow">Allow fixture access</button><button name="decision" value="deny">Deny fixture access</button></form>`))
				if err := form.Execute(w, q); err != nil {
					http.Error(w, "render failed", 500)
				}
				return
			}
			target, err := url.Parse(q.Get("redirect_uri"))
			if err != nil {
				http.Error(w, "invalid redirect", 400)
				return
			}
			out := target.Query()
			out.Set("state", q.Get("state"))
			out.Set("iss", f.URL)
			if github {
				out.Set("iss", "https://github.com/login/oauth")
			}
			if q.Get("decision") == "deny" {
				out.Set("error", "access_denied")
			} else {
				code := "fixture-code-" + q.Get("state")
				f.mu.Lock()
				f.codes[code] = q
				f.mu.Unlock()
				out.Set("code", code)
			}
			target.RawQuery = out.Encode()
			http.Redirect(w, r, target.String(), 303)
		case "/token", "/login/oauth/access_token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", 400)
				return
			}
			if github && (r.Form.Get("client_secret") != "fixture-client-secret" || r.Header.Get("Authorization") != "") {
				json.NewEncoder(w).Encode(map[string]string{"error": "incorrect_client_credentials", "error_description": "fixture-client-secret must not leak"})
				return
			}
			if r.Form.Get("resource") != resource {
				http.Error(w, "wrong resource", 400)
				return
			}
			expiry := 1
			if r.Form.Get("grant_type") == "authorization_code" {
				f.mu.Lock()
				q, ok := f.codes[r.Form.Get("code")]
				delete(f.codes, r.Form.Get("code"))
				f.mu.Unlock()
				hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				if !ok || q.Get("client_id") != r.Form.Get("client_id") || q.Get("redirect_uri") != r.Form.Get("redirect_uri") || q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(hash[:]) {
					http.Error(w, "invalid code proof", 400)
					return
				}
			} else if r.Form.Get("grant_type") == "refresh_token" {
				if r.Form.Get("refresh_token") != "fixture-refresh-token" {
					http.Error(w, "invalid refresh", 400)
					return
				}
				f.Refreshes.Add(1)
				expiry = 3600
				if r.Form.Get("client_id") == "fail-refresh" {
					http.Error(w, "fixture provider unavailable", 503)
					return
				}
			} else {
				http.Error(w, "invalid grant", 400)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture-mcp-token", "refresh_token": "fixture-refresh-token", "token_type": "Bearer", "expires_in": expiry, "scope": func() string {
				if github {
					return "read:user,read:org"
				}
				return "read"
			}()})
		case "/revoke":
			f.Revocations.Add(1)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", 400)
				return
			}
			if r.Form.Get("client_id") == "fail-revoke" {
				http.Error(w, "revocation unavailable", 503)
				return
			}
			fmt.Fprint(w, `{}`)
		case "/stats":
			json.NewEncoder(w).Encode(map[string]int32{"refreshes": f.Refreshes.Load(), "revocations": f.Revocations.Load()})
		default:
			w.WriteHeader(404)
		}
	}))
	if resource == "" {
		resource = f.URL + "/mcp"
	}
	return f
}

// GitHubClient routes only these fixture public URLs to the TLS server. It is
// injected exclusively by tests; production retains public-network validation.
func (f *OAuthServer) GitHubClient() *http.Client {
	client := f.Client()
	transport := client.Transport
	target, _ := url.Parse(f.URL)
	client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" || r.URL.Host == "api.githubcopilot.com" {
			r = r.Clone(r.Context())
			u := *r.URL
			u.Scheme, u.Host = target.Scheme, target.Host
			r.URL = &u
			r.Host = target.Host
		}
		return transport.RoundTrip(r)
	})
	return client
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
