// Package mcpconfig manages discovery and explicit grants. It does not invoke tools.
package mcpconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/events"
	"github.com/woliveiras/le-forge/server/internal/secrets"
	"github.com/woliveiras/le-forge/server/internal/storage"
)

const prefix = "mcp/connections/"

var ErrInvalid = errors.New("invalid MCP configuration")
var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var toolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

type Credential struct {
	Value string `json:"value,omitempty"`
	Clear bool   `json:"clear,omitempty"`
}
type Input struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Service    string      `json:"service"`
	Endpoint   string      `json:"endpoint"`
	Enabled    bool        `json:"enabled"`
	Credential *Credential `json:"credential,omitempty"`
}
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}
type View struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Service              string     `json:"service"`
	Endpoint             string     `json:"endpoint"`
	Enabled              bool       `json:"enabled"`
	Revision             int64      `json:"revision"`
	Status               string     `json:"status"`
	CredentialConfigured bool       `json:"credentialConfigured"`
	Tools                []Tool     `json:"tools"`
	ToolsRevision        int64      `json:"toolsRevision"`
	LastTest             *time.Time `json:"lastTest,omitempty"`
	Error                string     `json:"error,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	UpdatedBy            string     `json:"updatedBy"`
}
type record struct {
	View
	Sealed string `json:"sealed,omitempty"`
}
type Manager struct {
	Store    storage.Store
	Box      *secrets.Box
	Bus      *events.Bus
	Discover func(context.Context, string, string) ([]Tool, error)
}

func New(store storage.Store, box *secrets.Box, bus *events.Bus) *Manager {
	return &Manager{Store: store, Box: box, Bus: bus, Discover: Discover}
}
func validate(in Input) error {
	u, e := url.Parse(in.Endpoint)
	if !identifier.MatchString(in.ID) || len(strings.TrimSpace(in.Name)) == 0 || len(in.Name) > 120 {
		return fmt.Errorf("%w: valid id and display name required", ErrInvalid)
	}
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(in.Endpoint) > 2048 {
		return fmt.Errorf("%w: endpoint must be HTTPS without user info, query or fragment", ErrInvalid)
	}
	switch in.Service {
	case "github", "bitbucket", "jira", "confluence", "generic":
	default:
		return fmt.Errorf("%w: unknown service", ErrInvalid)
	}
	if c := in.Credential; c != nil && (len(c.Value) > 16384 || strings.ContainsAny(c.Value, "\r\n") || c.Clear && c.Value != "") {
		return fmt.Errorf("%w: invalid credential update", ErrInvalid)
	}
	return nil
}
func (m *Manager) load(ctx context.Context, id string) (record, error) {
	var r record
	if !identifier.MatchString(id) {
		return r, fmt.Errorf("%w: invalid connection id", ErrInvalid)
	}
	s, e := m.Store.GetSetting(ctx, prefix+id)
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(s.Value, &r)
	r.Revision = s.Revision
	return r, e
}
func (m *Manager) Get(ctx context.Context, id string) (View, error) {
	r, e := m.load(ctx, id)
	return r.View, e
}
func (m *Manager) List(ctx context.Context) ([]View, error) {
	ss, e := m.Store.ListSettings(ctx, prefix)
	if e != nil {
		return nil, e
	}
	out := []View{}
	for _, s := range ss {
		var r record
		if e = json.Unmarshal(s.Value, &r); e != nil {
			return nil, e
		}
		r.Revision = s.Revision
		out = append(out, r.View)
	}
	return out, nil
}
func (m *Manager) save(ctx context.Context, r record, rev int64, actor, event string) (View, error) {
	r.UpdatedAt = time.Now().UTC()
	r.UpdatedBy = actor
	r.CredentialConfigured = r.Sealed != ""
	b, e := json.Marshal(r)
	if e != nil {
		return View{}, e
	}
	r.Revision, e = m.Store.PutSetting(ctx, prefix+r.ID, b, rev, actor)
	if e != nil {
		return View{}, e
	}
	if m.Bus != nil {
		m.Bus.Publish(ctx, domain.NewEvent(domain.EventType(event), "", map[string]any{"connectionId": r.ID, "revision": r.Revision, "status": r.Status, "toolCount": len(r.Tools)}).WithActor(actor))
	}
	return r.View, nil
}
func (m *Manager) Put(ctx context.Context, in Input, revision int64, actor string) (View, error) {
	if e := validate(in); e != nil {
		return View{}, e
	}
	r, e := m.load(ctx, in.ID)
	if revision == 0 {
		if e == nil {
			return View{}, storage.ErrConflict
		}
		if !errors.Is(e, storage.ErrNotFound) {
			return View{}, e
		}
		r = record{View: View{ID: in.ID, CreatedAt: time.Now().UTC(), Tools: []Tool{}}}
	} else {
		if e != nil {
			return View{}, e
		}
		if r.Revision != revision {
			return View{}, storage.ErrConflict
		}
	}
	if r.Endpoint != "" && r.Endpoint != in.Endpoint && r.Sealed != "" && (in.Credential == nil || !in.Credential.Clear && in.Credential.Value == "") {
		return View{}, fmt.Errorf("%w: replace or clear the credential when changing endpoint", ErrInvalid)
	}
	changed := r.Endpoint != in.Endpoint || in.Credential != nil || r.Enabled != in.Enabled
	r.Name = in.Name
	r.Service = in.Service
	r.Endpoint = in.Endpoint
	r.Enabled = in.Enabled
	if in.Credential != nil {
		if in.Credential.Clear {
			r.Sealed = ""
		} else if in.Credential.Value != "" {
			r.Sealed, e = m.Box.Seal([]byte(in.Credential.Value), prefix+in.ID)
			if e != nil {
				return View{}, e
			}
		}
	}
	if changed {
		r.Tools = []Tool{}
		r.ToolsRevision++
		r.LastTest = nil
		r.Error = ""
		r.Status = "untested"
	}
	if !r.Enabled {
		r.Status = "disabled"
	}
	return m.save(ctx, r, revision, actor, "mcp.connection.changed")
}
func (m *Manager) Refresh(ctx context.Context, id, actor string) (View, error) {
	r, e := m.load(ctx, id)
	if e != nil {
		return View{}, e
	}
	if !r.Enabled {
		return View{}, fmt.Errorf("%w: connection disabled", ErrInvalid)
	}
	token := ""
	if r.Sealed != "" {
		b, e := m.Box.Open(r.Sealed, prefix+id)
		if e != nil {
			return View{}, errors.New("MCP credential cannot be decrypted")
		}
		token = string(b)
	}
	opctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	started := time.Now()
	catalog, discoverErr := m.Discover(opctx, r.Endpoint, token)
	now := time.Now().UTC()
	r.LastTest = &now
	r.ToolsRevision++
	r.Tools = []Tool{}
	r.Error = ""
	r.Status = "ready"
	if discoverErr != nil {
		r.Status = "unavailable"
		r.Error = "Discovery failed: check endpoint, TLS, network policy and credentials."
		var detail *DiscoveryError
		if errors.As(discoverErr, &detail) {
			r.Error = "Discovery failed: " + detail.Reason
		}
	} else {
		r.Tools = catalog
	}
	result, e := m.save(ctx, r, r.Revision, actor, "mcp.tools.refreshed")
	if m.Bus != nil {
		outcome := "success"
		if discoverErr != nil || e != nil {
			outcome = "failed"
		}
		m.Bus.Publish(ctx, domain.NewEvent("mcp.connection.tested", "", map[string]any{"connectionId": id, "outcome": outcome, "durationMs": time.Since(started).Milliseconds()}).WithActor(actor))
	}
	return result, e
}

// ValidateGrants only accepts exact, currently discovered tool names. Wildcards
// do not opt agents into future connection catalogues.
func (m *Manager) ValidateGrants(ctx context.Context, g *domain.Graph) error {
	grants := append([]domain.ToolGrant{}, g.Spec.Defaults.Tools...)
	for _, n := range g.Spec.Nodes {
		grants = append(grants, n.Tools...)
	}
	var views []View
	for _, grant := range grants {
		if !strings.HasPrefix(grant.Name, "mcp.") {
			continue
		}
		if views == nil {
			var e error
			views, e = m.List(ctx)
			if e != nil {
				return e
			}
		}
		found := false
		for _, v := range views {
			if !v.Enabled || v.Status != "ready" {
				continue
			}
			for _, t := range v.Tools {
				if grant.Name == "mcp."+v.ID+"."+t.Name {
					found = true
				}
			}
		}
		if !found {
			return fmt.Errorf("%w: MCP grant must name an enabled discovered tool", ErrInvalid)
		}
	}
	return nil
}
