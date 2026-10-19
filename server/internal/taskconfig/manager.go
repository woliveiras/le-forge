// Package taskconfig stores user requests without granting execution authority.
package taskconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/storage"
)

const prefix = "tasks/drafts/"

var validID = regexp.MustCompile(`^tsk_[a-z0-9]{1,80}$`)
var ErrInvalid = errors.New("invalid task")

type Input struct {
	Title        string `json:"title"`
	Request      string `json:"request"`
	Outcome      string `json:"outcome"`
	GraphID      string `json:"graphId"`
	GraphVersion int    `json:"graphVersion"`
	NodeID       string `json:"nodeId"`
}
type Draft struct {
	Input
	ID        string    `json:"id"`
	Revision  int64     `json:"revision"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"createdBy"`
	UpdatedBy string    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Page struct {
	Items []*Draft `json:"items"`
	Next  string   `json:"next,omitempty"`
}
type Manager struct{ Store storage.Store }

func Validate(in Input) error {
	for _, field := range []struct {
		name, value string
		max         int
	}{
		{"title", in.Title, 200}, {"request", in.Request, 16384}, {"graphId", in.GraphID, 128}, {"nodeId", in.NodeID, 128},
	} {
		if strings.TrimSpace(field.value) == "" || len(field.value) > field.max || !utf8.ValidString(field.value) || strings.ContainsRune(field.value, 0) {
			return fmt.Errorf("%w: %s is required and must be valid text within %d bytes", ErrInvalid, field.name, field.max)
		}
	}
	switch in.Outcome {
	case "understand", "implement", "investigate", "review", "freeform":
	default:
		return fmt.Errorf("%w: unknown outcome", ErrInvalid)
	}
	if in.GraphVersion < 1 {
		return fmt.Errorf("%w: graphVersion must be positive", ErrInvalid)
	}
	return nil
}
func decode(rec *storage.Setting) (*Draft, error) {
	var d Draft
	if err := json.Unmarshal(rec.Value, &d); err != nil {
		return nil, errors.New("invalid stored task draft")
	}
	d.Revision = rec.Revision
	return &d, nil
}
func (m Manager) Get(ctx context.Context, id string) (*Draft, error) {
	if !validID.MatchString(id) {
		return nil, storage.ErrNotFound
	}
	rec, err := m.Store.GetSetting(ctx, prefix+id)
	if err != nil {
		return nil, err
	}
	return decode(rec)
}
func (m Manager) List(ctx context.Context, after string, limit int) (Page, error) {
	page := Page{Items: []*Draft{}}
	if after != "" && !validID.MatchString(after) {
		return page, fmt.Errorf("%w: invalid cursor", ErrInvalid)
	}
	if limit < 1 || limit > 100 {
		return page, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalid)
	}
	records, err := m.Store.ListSettings(ctx, prefix)
	if err != nil {
		return page, err
	}
	for _, rec := range records {
		if rec.Key <= prefix+after {
			continue
		}
		if len(page.Items) == limit {
			page.Next = page.Items[len(page.Items)-1].ID
			break
		}
		d, err := decode(rec)
		if err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, d)
	}
	return page, nil
}
func (m Manager) Save(ctx context.Context, id string, in Input, revision int64, actor string) (*Draft, error) {
	if err := Validate(in); err != nil {
		return nil, err
	}
	// Saving is allowed only against the explicitly selected current version.
	// Retained drafts may later become stale; reads do not discard them.
	g, err := m.Store.GetGraph(ctx, in.GraphID, 0)
	if err != nil {
		return nil, err
	}
	if g.Metadata.Version != in.GraphVersion {
		return nil, storage.ErrConflict
	}
	n := g.Node(in.NodeID)
	if n == nil || n.Disabled {
		return nil, fmt.Errorf("%w: select an enabled agent in the workflow", ErrInvalid)
	}
	now := time.Now().UTC()
	d := &Draft{Input: in, ID: id, Status: "draft", CreatedBy: actor, UpdatedBy: actor, CreatedAt: now, UpdatedAt: now}
	if id == "" {
		if revision != 0 {
			return nil, storage.ErrConflict
		}
		d.ID = domain.NewID("tsk")
	} else {
		old, err := m.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if revision < 1 || revision != old.Revision {
			return nil, storage.ErrConflict
		}
		d.CreatedAt = old.CreatedAt
		d.CreatedBy = old.CreatedBy
	}
	// Audit intent before the CAS; success is represented by the persisted record.
	// Failure to audit prevents mutation. The event includes no request content.
	ev := domain.NewEvent("task.draft_save_requested", in.GraphID, map[string]any{"taskId": d.ID, "expectedRevision": revision}).WithActor(actor)
	if err := m.Store.AppendEvent(ctx, ev); err != nil {
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	rev, err := m.Store.PutSetting(ctx, prefix+d.ID, b, revision, actor)
	if err != nil {
		return nil, err
	}
	d.Revision = rev
	return d, nil
}
