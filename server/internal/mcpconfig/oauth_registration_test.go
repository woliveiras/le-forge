package mcpconfig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/mcpconfig/mcptest"
)

const registrationCallback = "https://forge.example/api/v1/mcp/oauth/callback"

func TestRegistrationSelection(t *testing.T) {
	for _, tc := range []struct {
		mode, callback string
		cimd, dcr      bool
		want           string
	}{
		{"manual", registrationCallback, true, true, "manual"}, {"auto", registrationCallback, true, true, "cimd"}, {"auto", "http://localhost:8080/api/v1/mcp/oauth/callback", true, true, "dcr"}, {"dcr", registrationCallback, true, true, "dcr"}, {"cimd", "http://localhost:8080/api/v1/mcp/oauth/callback", true, true, ""}, {"auto", registrationCallback, false, false, ""}, {"cimd", registrationCallback, false, true, ""},
	} {
		t.Run(tc.mode+tc.callback+tc.want, func(t *testing.T) {
			r := record{View: View{ID: "test", OAuth: &OAuthView{OAuthConfig: OAuthConfig{RegistrationMode: tc.mode}}}}
			meta := oauthMetadata{CIMD: tc.cimd, Methods: []string{"none"}}
			if tc.dcr {
				meta.Registration = "https://issuer.example/register"
			}
			plan, err := registrationPlan(r, meta, tc.callback)
			if tc.want == "" {
				if err == nil {
					t.Fatal("accepted unsupported setup")
				}
			} else if err != nil || plan.Mode != tc.want {
				t.Fatal(plan, err)
			}
		})
	}
	for _, mode := range []string{"auto", "dcr", "cimd"} {
		c := OAuthConfig{Issuer: "https://issuer.example", RegistrationMode: mode}
		if err := validateOAuth(&c); err != nil {
			t.Fatal(err)
		}
		c.ClientID = "manual"
		if validateOAuth(&c) == nil {
			t.Fatal("mixed registration accepted")
		}
	}
}
func TestDCRPersistenceConcurrencyAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "reuse", true: "uncertain"}[fail], func(t *testing.T) {
			m := testManager(t)
			f := mcptest.NewOAuth("")
			defer f.Close()
			f.FailRegistration.Store(fail)
			m.OAuthClient = f.Client()
			in := input()
			in.Service = "generic"
			in.Endpoint = f.URL + "/mcp"
			in.OAuth = &OAuthConfig{Issuer: f.URL, RegistrationMode: "dcr", Scopes: []string{"read"}}
			if _, err := m.Put(t.Context(), in, 0, "admin"); err != nil {
				t.Fatal(err)
			}
			if _, err := m.StartOAuth(t.Context(), in.ID, "admin", registrationCallback); err == nil {
				t.Fatal("unreviewed registration accepted")
			}
			if f.Registrations.Load() != 0 {
				t.Fatal("review caused registration")
			}
			var wg sync.WaitGroup
			var successes atomic.Int32
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := m.StartOAuthReviewed(t.Context(), in.ID, "admin", registrationCallback, "dcr"); err == nil {
						successes.Add(1)
					}
				}()
			}
			wg.Wait()
			if f.Registrations.Load() != 1 {
				t.Fatal("duplicate registration", f.Registrations.Load())
			}
			if fail && successes.Load() != 0 {
				t.Fatal("failed registration authorized")
			}
			restarted := New(m.Store, m.Box, nil)
			restarted.OAuthClient = f.Client()
			_, err := restarted.StartOAuthReviewed(t.Context(), in.ID, "admin", registrationCallback, "dcr")
			if (err != nil) != fail {
				t.Fatal(err)
			}
			if f.Registrations.Load() != 1 {
				t.Fatal("restart re-registered")
			}
			if _, err = restarted.PreviewOAuth(t.Context(), in.ID, "https://changed.example/api/v1/mcp/oauth/callback"); err == nil {
				t.Fatal("changed callback reused")
			}
			v, _ := m.Get(t.Context(), in.ID)
			raw, _ := json.Marshal(v)
			if strings.Contains(string(raw), "registered-fixture-client") {
				t.Fatal("internal registration leaked")
			}
		})
	}
}
func TestDCRResponseValidationAndSecretStorage(t *testing.T) {
	for _, variant := range []string{"valid", "wrong-callback", "method", "scope", "expiry", "oversize", "failure", "malformed"} {
		t.Run(variant, func(t *testing.T) {
			m := testManager(t)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var in map[string]any
				if json.NewDecoder(r.Body).Decode(&in) != nil {
					t.Error("bad request")
				}
				if in["token_endpoint_auth_method"] != "client_secret_basic" {
					t.Error("wrong requested method")
				}
				out := map[string]any{"client_id": "registered", "client_secret": "synthetic-registration-secret", "redirect_uris": []string{registrationCallback}, "token_endpoint_auth_method": "client_secret_basic"}
				switch variant {
				case "wrong-callback":
					out["redirect_uris"] = []string{"https://evil.example"}
				case "method":
					out["token_endpoint_auth_method"] = "none"
				case "scope":
					out["scope"] = "admin"
				case "expiry":
					out["client_secret_expires_at"] = 1
				case "oversize":
					out["client_id"] = strings.Repeat("x", 70000)
				case "failure":
					w.WriteHeader(503)
				case "malformed":
					w.Write([]byte("synthetic-registration-secret"))
					return
				}
				json.NewEncoder(w).Encode(out)
			}))
			defer server.Close()
			m.OAuthClient = server.Client()
			r := record{View: View{ID: "test", OAuth: &OAuthView{OAuthConfig: OAuthConfig{RegistrationMode: "dcr", Scopes: []string{"read"}}}}, Registration: &clientRegistration{}}
			err := m.registerClient(t.Context(), &r, oauthMetadata{Registration: server.URL, Methods: []string{"client_secret_basic"}}, registrationCallback)
			if variant == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if r.OAuthClientSealed == "" || strings.Contains(r.OAuthClientSealed, "synthetic-registration-secret") {
					t.Fatal("unencrypted credential")
				}
				b, e := m.Box.Open(r.OAuthClientSealed, prefix+r.ID+"/oauth-client")
				if e != nil || string(b) != "synthetic-registration-secret" {
					t.Fatal("credential binding", e)
				}
			} else if err == nil || strings.Contains(err.Error(), "synthetic-registration-secret") {
				t.Fatal("unsafe error", err)
			}
		})
	}
}

func TestCIMDDocumentURLBoundaries(t *testing.T) {
	for _, base := range []string{"https://", "https://localhost", "https://LOCALHOST.", "https://127.0.0.1", "https://10.0.0.1", "http://forge.example", "https://user:secret@forge.example"} {
		if _, err := clientDocumentURL(base+"/api/v1/mcp/oauth/callback", "test"); err == nil {
			t.Fatal("invalid public document URL accepted", base)
		}
	}
	for _, suffix := range []string{"?secret=x", "#fragment", "/other"} {
		if _, err := clientDocumentURL(registrationCallback+suffix, "test"); err == nil {
			t.Fatal("invalid callback accepted")
		}
	}
	if got, err := clientDocumentURL(registrationCallback, "test"); err != nil || got != "https://forge.example/api/v1/mcp/oauth/client/test" {
		t.Fatal(got, err)
	}
}
